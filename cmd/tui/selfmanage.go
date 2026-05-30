package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultReleaseRepo = "Patruxs/utils"
	utilsBinaryName    = "utils"
)

type githubRelease struct {
	TagName string               `json:"tag_name"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func updateSelf(currentVersion string) error {
	exePath, err := currentExecutablePath()
	if err != nil {
		return err
	}

	repo := strings.TrimSpace(os.Getenv("UTILS_REPO"))
	if repo == "" {
		repo = defaultReleaseRepo
	}

	fmt.Printf("Checking latest UTILS release from %s...\n", repo)
	release, err := fetchLatestRelease(repo)
	if err != nil {
		return err
	}
	if release.TagName == "" {
		return errors.New("latest release response did not include a tag")
	}
	if currentVersion != "dev" && currentVersion == release.TagName {
		fmt.Printf("UTILS is already up to date (%s).\n", currentVersion)
		return nil
	}

	assetName, err := releaseAssetName(release.TagName)
	if err != nil {
		return err
	}
	asset, ok := findReleaseAsset(release.Assets, assetName)
	if !ok {
		return fmt.Errorf("latest release %s does not include asset %s", release.TagName, assetName)
	}

	tempDir, err := os.MkdirTemp("", "utils-update-*")
	if err != nil {
		return fmt.Errorf("create temporary update directory: %w", err)
	}
	if runtime.GOOS != "windows" {
		defer os.RemoveAll(tempDir)
	}

	archivePath := filepath.Join(tempDir, asset.Name)
	newBinaryPath := filepath.Join(tempDir, binaryFileName())
	fmt.Printf("Downloading %s...\n", asset.Name)
	if err := downloadFile(asset.BrowserDownloadURL, archivePath); err != nil {
		return err
	}
	if err := extractBinary(archivePath, newBinaryPath); err != nil {
		return err
	}

	fmt.Printf("Updating UTILS from %s to %s...\n", currentVersion, release.TagName)
	if runtime.GOOS == "windows" {
		if err := scheduleWindowsUpdate(exePath, newBinaryPath, tempDir, release.TagName); err != nil {
			return err
		}
		fmt.Println("Update scheduled. It will finish a few seconds after this command exits.")
		return nil
	}

	if err := installBinary(exePath, newBinaryPath); err != nil {
		return err
	}
	fmt.Printf("UTILS updated successfully to %s.\n", release.TagName)
	return nil
}

func uninstallSelf() error {
	exePath, err := currentExecutablePath()
	if err != nil {
		return err
	}

	fmt.Printf("Warning: uninstalling UTILS by removing %s\n", exePath)
	if runtime.GOOS == "windows" {
		if err := scheduleWindowsUninstall(exePath); err != nil {
			return err
		}
		fmt.Println("Uninstall scheduled. It will finish a few seconds after this command exits.")
		return nil
	}

	if err := os.Remove(exePath); err != nil {
		return err
	}
	fmt.Println("UTILS executable removed successfully.")
	return nil
}

func currentExecutablePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(exePath)
	if err != nil {
		return "", err
	}
	return absPath, nil
}

func fetchLatestRelease(repo string) (githubRelease, error) {
	var release githubRelease
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo), nil)
	if err != nil {
		return release, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "utils-self-update")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return release, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return release, fmt.Errorf("GitHub latest release request failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return release, err
	}
	return release, nil
}

func releaseAssetName(tag string) (string, error) {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return "", fmt.Errorf("unsupported architecture %s; supported architectures are amd64 and arm64", runtime.GOARCH)
	}

	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("%s_%s_windows_%s.zip", utilsBinaryName, tag, runtime.GOARCH), nil
	case "linux", "darwin":
		return fmt.Sprintf("%s_%s_%s_%s.tar.gz", utilsBinaryName, tag, runtime.GOOS, runtime.GOARCH), nil
	default:
		return "", fmt.Errorf("unsupported OS %s", runtime.GOOS)
	}
}

func findReleaseAsset(assets []githubReleaseAsset, name string) (githubReleaseAsset, bool) {
	for _, asset := range assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return githubReleaseAsset{}, false
}

func downloadFile(url, destination string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "utils-self-update")

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	out, err := os.Create(destination)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func extractBinary(archivePath, destination string) error {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractBinaryFromZip(archivePath, destination)
	}
	if strings.HasSuffix(archivePath, ".tar.gz") {
		return extractBinaryFromTarGz(archivePath, destination)
	}
	return fmt.Errorf("unsupported archive format: %s", archivePath)
}

func extractBinaryFromZip(archivePath, destination string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.FileInfo().IsDir() || path.Base(file.Name) != binaryFileName() {
			continue
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		defer src.Close()
		return writeExtractedBinary(src, destination)
	}
	return fmt.Errorf("archive did not contain %s", binaryFileName())
}

func extractBinaryFromTarGz(archivePath, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg || path.Base(header.Name) != binaryFileName() {
			continue
		}
		return writeExtractedBinary(tarReader, destination)
	}
	return fmt.Errorf("archive did not contain %s", binaryFileName())
}

func writeExtractedBinary(src io.Reader, destination string) error {
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(destination, 0755)
}

func installBinary(targetPath, newBinaryPath string) error {
	targetDir := filepath.Dir(targetPath)
	tempTarget, err := os.CreateTemp(targetDir, ".utils-new-*")
	if err != nil {
		return err
	}
	tempTargetPath := tempTarget.Name()
	defer os.Remove(tempTargetPath)

	src, err := os.Open(newBinaryPath)
	if err != nil {
		tempTarget.Close()
		return err
	}
	defer src.Close()

	if _, err := io.Copy(tempTarget, src); err != nil {
		tempTarget.Close()
		return err
	}
	if err := tempTarget.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tempTargetPath, 0755); err != nil {
		return err
	}
	if err := os.Rename(tempTargetPath, targetPath); err != nil {
		return err
	}
	return nil
}

func scheduleWindowsUpdate(targetPath, newBinaryPath, tempDir, tag string) error {
	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$target = %s
$newBinary = %s
$tempDir = %s
$pidToWait = %d
Wait-Process -Id $pidToWait -ErrorAction SilentlyContinue
Start-Sleep -Milliseconds 300
$deadline = (Get-Date).AddSeconds(30)
while ($true) {
  try {
    Copy-Item -LiteralPath $newBinary -Destination $target -Force
    Unblock-File -LiteralPath $target -ErrorAction SilentlyContinue
    break
  } catch {
    if ((Get-Date) -gt $deadline) { throw }
    Start-Sleep -Milliseconds 500
  }
}
Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
Write-Host ('UTILS updated successfully to ' + %s + '.')
`, powershellString(targetPath), powershellString(newBinaryPath), powershellString(tempDir), os.Getpid(), powershellString(tag))
	return startPowerShellScript(script)
}

func scheduleWindowsUninstall(targetPath string) error {
	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$target = %s
$pidToWait = %d
Wait-Process -Id $pidToWait -ErrorAction SilentlyContinue
Start-Sleep -Milliseconds 300
$deadline = (Get-Date).AddSeconds(30)
while ($true) {
  try {
    Remove-Item -LiteralPath $target -Force
    break
  } catch {
    if ((Get-Date) -gt $deadline) { throw }
    Start-Sleep -Milliseconds 500
  }
}
$dir = Split-Path -Parent $target
if ((Test-Path -LiteralPath $dir) -and -not (Get-ChildItem -LiteralPath $dir -Force -ErrorAction SilentlyContinue)) {
  Remove-Item -LiteralPath $dir -Force -ErrorAction SilentlyContinue
}
Write-Host 'UTILS executable removed successfully.'
`, powershellString(targetPath), os.Getpid())
	return startPowerShellScript(script)
}

func startPowerShellScript(script string) error {
	var lastErr error
	for _, name := range []string{"powershell.exe", "powershell"} {
		cmd := exec.Command(name, "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
		if err := cmd.Start(); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return fmt.Errorf("start PowerShell helper: %w", lastErr)
}

func powershellString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func binaryFileName() string {
	if runtime.GOOS == "windows" {
		return utilsBinaryName + ".exe"
	}
	return utilsBinaryName
}
