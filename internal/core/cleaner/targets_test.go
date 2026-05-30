package cleaner

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDeveloperTargetsIncludeIDEAndCopilotData(t *testing.T) {
	home := t.TempDir()
	appData := filepath.Join(home, "AppData", "Roaming")
	localAppData := filepath.Join(home, "AppData", "Local")
	fs := envOnlyFS{
		envAPPDATA:      appData,
		envLOCALAPPDATA: localAppData,
	}

	targets := developerTargets(home, fs)

	switch runtime.GOOS {
	case osWindows:
		assertTarget(t, targets, filepath.Join(appData, "Code", "User", "globalStorage"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(appData, "Code", "Cache"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(appData, "Code - Insiders", "User", "workspaceStorage"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(localAppData, ".IdentityService"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(localAppData, "Microsoft", "VisualStudio"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(localAppData, "Microsoft", "VSCommon"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(appData, "GitHub Copilot"), targetLabelCopilotAuthCacheData)
	case "darwin":
		assertTarget(t, targets, filepath.Join(home, "Library", "Application Support", "Code", "User", "globalStorage"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(home, "Library", "Caches", "com.microsoft.VSCode"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(home, "Library", "Application Support", "VisualStudio"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(home, "Library", "Application Support", "GitHub Copilot"), targetLabelCopilotAuthCacheData)
	default:
		assertTarget(t, targets, filepath.Join(home, ".config", "Code", "User", "globalStorage"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(home, ".config", "Code", "Cache"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(home, ".cache", "Code"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(home, ".config", "GitHub Copilot"), targetLabelCopilotAuthCacheData)
	}

	assertTarget(t, targets, filepath.Join(home, ".github-copilot"), targetLabelCopilotAuthCacheData)
}

func TestCredentialManagerAllowlistIncludesVisualStudioAndCopilot(t *testing.T) {
	for _, target := range []string{
		"vscodevscode.github-authentication",
		"VS Code Azure Login",
		"Visual Studio Account",
		"Microsoft_VisualStudio_Token",
		"github.copilot",
		"GitHub Copilot",
	} {
		if !matchesCredentialAllowlist(target) {
			t.Fatalf("expected Credential Manager target %q to match allowlist", target)
		}
	}

	for _, target := range []string{
		"MicrosoftAccount:user=person@example.com",
		"random-code-signing",
	} {
		if matchesCredentialAllowlist(target) {
			t.Fatalf("expected Credential Manager target %q not to match allowlist", target)
		}
	}
}

func assertTarget(t *testing.T, targets []targetPath, wantPath string, wantLabel string) {
	t.Helper()

	wantPath = filepath.Clean(wantPath)
	for _, target := range targets {
		if filepath.Clean(target.path) == wantPath && target.label == wantLabel {
			return
		}
	}

	t.Fatalf("expected target %q with label %q, got %#v", wantPath, wantLabel, targets)
}

type envOnlyFS map[string]string

func (fs envOnlyFS) UserHomeDir() (string, error) {
	return "", nil
}

func (fs envOnlyFS) Getenv(key string) string {
	return fs[key]
}

func (fs envOnlyFS) MkdirAll(string, os.FileMode) error {
	return nil
}

func (fs envOnlyFS) Lstat(string) (os.FileInfo, error) {
	return nil, os.ErrNotExist
}

func (fs envOnlyFS) ReadDir(string) ([]os.DirEntry, error) {
	return nil, os.ErrNotExist
}

func (fs envOnlyFS) RemoveAll(string) error {
	return nil
}

func (fs envOnlyFS) WriteFile(string, []byte, os.FileMode) error {
	return nil
}

func (fs envOnlyFS) EvalSymlinks(path string) (string, error) {
	return path, nil
}

func (fs envOnlyFS) Glob(string) ([]string, error) {
	return nil, nil
}
