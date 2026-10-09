package cleaner

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

	targets := append(developerCredentialTargets(home, fs), fullToolResetTargets(home, fs)...)

	switch runtime.GOOS {
	case osWindows:
		assertTarget(t, targets, filepath.Join(appData, "Code", "User", "globalStorage"), targetLabelVSCodeGlobalState)
		assertTarget(t, targets, filepath.Join(appData, "Code", "Cache"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(appData, "Code - Insiders", "User", "workspaceStorage"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(localAppData, ".IdentityService"), targetLabelIDECredential)
		assertTarget(t, targets, filepath.Join(localAppData, "Microsoft", "VisualStudio"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(localAppData, "Microsoft", "VSCommon"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(appData, "GitHub Copilot"), targetLabelCopilotAuthCacheData)
	case "darwin":
		assertTarget(t, targets, filepath.Join(home, "Library", "Application Support", "Code", "User", "globalStorage"), targetLabelVSCodeGlobalState)
		assertTarget(t, targets, filepath.Join(home, "Library", "Caches", "com.microsoft.VSCode"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(home, "Library", "Application Support", "VisualStudio"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(home, "Library", "Application Support", "GitHub Copilot"), targetLabelCopilotAuthCacheData)
	default:
		assertTarget(t, targets, filepath.Join(home, ".config", "Code", "User", "globalStorage"), targetLabelVSCodeGlobalState)
		assertTarget(t, targets, filepath.Join(home, ".config", "Code", "Cache"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(home, ".cache", "Code"), targetLabelIDEAuthCacheData)
		assertTarget(t, targets, filepath.Join(home, ".config", "GitHub Copilot"), targetLabelCopilotAuthCacheData)
	}

	assertTarget(t, targets, filepath.Join(home, ".github-copilot"), targetLabelCopilotAuthCacheData)

	for _, target := range developerCredentialTargets(home, fs) {
		if strings.Contains(target.path, "globalStorage") {
			t.Fatalf("expected VS Code global state to stay out of the always-included targets, got %q", target.path)
		}
	}
}

func TestBrowserTargetsMatchTheOperatingSystem(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("Windows browser paths come from APPDATA and LOCALAPPDATA")
	}
	home := t.TempDir()
	fs := envOnlyFS{}
	profiles := browserProfileTargets(home, fs)
	caches := browserCacheTargets(home, fs)

	if runtime.GOOS == "darwin" {
		assertTarget(t, profiles, filepath.Join(home, "Library", "Application Support", "Microsoft Edge"), targetLabelBrowserProfileRoot)
		assertTarget(t, caches, filepath.Join(home, "Library", "Caches", "Microsoft Edge"), targetLabelBrowserCache)
		assertNoTargetUnder(t, append(profiles, caches...), filepath.Join(home, ".var"), filepath.Join(home, ".config"), filepath.Join(home, ".mozilla"))
		return
	}

	assertTarget(t, profiles, filepath.Join(home, ".config", "microsoft-edge"), targetLabelBrowserProfileRoot)
	assertTarget(t, profiles, filepath.Join(home, ".var", "app", "com.microsoft.Edge", "config", "microsoft-edge"), targetLabelBrowserProfileRoot)
	assertTarget(t, profiles, filepath.Join(home, ".var", "app", "com.google.Chrome", "config", "google-chrome"), targetLabelBrowserProfileRoot)
	assertTarget(t, profiles, filepath.Join(home, ".var", "app", "org.mozilla.firefox", ".mozilla", "firefox"), targetLabelBrowserProfile)
	assertTarget(t, caches, filepath.Join(home, ".cache", "microsoft-edge"), targetLabelBrowserCache)
	assertTarget(t, caches, filepath.Join(home, ".cache", "BraveSoftware", "Brave-Browser"), targetLabelBrowserCache)
	assertTarget(t, caches, filepath.Join(home, ".var", "app", "com.microsoft.Edge", "cache", "microsoft-edge"), targetLabelBrowserCache)

	all := append(append(append(append(append(profiles, caches...),
		developerCredentialTargets(home, fs)...),
		fullToolResetTargets(home, fs)...),
		sshTargets(home, fs)...),
		historyTargets(home, fs)...)
	assertNoTargetUnder(t, all, filepath.Join(home, "Library"))
}

func assertNoTargetUnder(t *testing.T, targets []targetPath, roots ...string) {
	t.Helper()

	for _, target := range targets {
		for _, root := range roots {
			if strings.HasPrefix(filepath.Clean(target.path)+string(filepath.Separator), root+string(filepath.Separator)) {
				t.Fatalf("expected no target under %q on %s, got %q", root, runtime.GOOS, target.path)
			}
		}
	}
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

func (fs envOnlyFS) OpenRoot(string) (*os.Root, error) {
	return nil, os.ErrNotExist
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
