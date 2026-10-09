package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testArchiveName = "utils_v9.9.9_linux_amd64.tar.gz"

func serveRelease(t *testing.T, archive []byte, checksums string, includeChecksums bool) []githubReleaseAsset {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/"+testArchiveName, func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	})
	mux.HandleFunc("/"+checksumsAssetName, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, checksums)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	assets := []githubReleaseAsset{{Name: testArchiveName, BrowserDownloadURL: server.URL + "/" + testArchiveName}}
	if includeChecksums {
		assets = append(assets, githubReleaseAsset{Name: checksumsAssetName, BrowserDownloadURL: server.URL + "/" + checksumsAssetName})
	}
	return assets
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestDownloadVerifiedArchiveAcceptsMatchingChecksum(t *testing.T) {
	archive := []byte("release archive")
	checksums := sha256Hex([]byte("other")) + "  utils_v9.9.9_darwin_arm64.tar.gz\n" + sha256Hex(archive) + "  " + testArchiveName + "\n"
	assets := serveRelease(t, archive, checksums, true)

	archivePath, err := downloadVerifiedArchive(assets, testArchiveName, t.TempDir())
	if err != nil {
		t.Fatalf("expected matching checksum to verify, got %v", err)
	}
	got, err := os.ReadFile(archivePath)
	if err != nil || string(got) != string(archive) {
		t.Fatalf("expected downloaded archive at %s, got %q, %v", archivePath, got, err)
	}
}

func TestDownloadVerifiedArchiveRejectsTamperedArchive(t *testing.T) {
	checksums := sha256Hex([]byte("release archive")) + "  " + testArchiveName + "\n"
	assets := serveRelease(t, []byte("tampered archive"), checksums, true)

	if _, err := downloadVerifiedArchive(assets, testArchiveName, t.TempDir()); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestDownloadVerifiedArchiveFailsClosedWithoutChecksum(t *testing.T) {
	archive := []byte("release archive")

	t.Run("no entry for the archive", func(t *testing.T) {
		assets := serveRelease(t, archive, sha256Hex(archive)+"  utils_v9.9.9_linux_arm64.tar.gz\n", true)
		if _, err := downloadVerifiedArchive(assets, testArchiveName, t.TempDir()); err == nil {
			t.Fatal("expected an archive without a checksum entry to be rejected")
		}
	})

	t.Run("release without checksums.txt", func(t *testing.T) {
		assets := serveRelease(t, archive, "", false)
		if _, err := downloadVerifiedArchive(assets, testArchiveName, t.TempDir()); err == nil {
			t.Fatal("expected a release without checksums.txt to be rejected")
		}
	})
}

func TestPackageManagerForDetectsBrewAndScoopInstalls(t *testing.T) {
	root := t.TempDir()
	if manager, ok := packageManagerFor(`C:\Users\dev\scoop\apps\utils\current\utils.exe`); !ok || manager.uninstallCommand != "scoop uninstall utils" {
		t.Fatalf("expected Scoop, got %+v, %v", manager, ok)
	}
	if manager, ok := packageManagerFor(filepath.Join(root, ".local", "bin", "utils")); ok {
		t.Fatalf("expected a self-installed binary to be managed by UTILS, got %+v", manager)
	}

	cellarBinary := filepath.Join(root, "Cellar", "utils", "1.0.0", "bin", "utils")
	if err := os.MkdirAll(filepath.Dir(cellarBinary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cellarBinary, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	brewLink := filepath.Join(root, "bin", "utils")
	if err := os.MkdirAll(filepath.Dir(brewLink), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(cellarBinary, brewLink); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if manager, ok := packageManagerFor(brewLink); !ok || manager.updateCommand != "brew upgrade utils" {
		t.Fatalf("expected Homebrew through the bin symlink, got %+v, %v", manager, ok)
	}
}

func TestConfirmUninstallRequiresExplicitYes(t *testing.T) {
	if confirmUninstall(strings.NewReader(""), io.Discard, "utils") {
		t.Fatal("closed input must not confirm uninstall")
	}
	if confirmUninstall(strings.NewReader("\n"), io.Discard, "utils") {
		t.Fatal("pressing enter must not confirm uninstall")
	}
	if !confirmUninstall(strings.NewReader("y\n"), io.Discard, "utils") {
		t.Fatal("answering y must confirm uninstall")
	}
}

func TestParseActionRejectsMoreThanOneAction(t *testing.T) {
	if _, err := parseAction("utils", []string{"--update", "--uninstall"}, io.Discard); !errors.Is(err, errConflictingActions) {
		t.Fatalf("expected conflicting actions error, got %v", err)
	}
	if action, err := parseAction("utils", []string{"--update"}, io.Discard); err != nil || action != actionUpdate {
		t.Fatalf("expected a single --update to select update, got %v, %v", action, err)
	}
}
