package cleaner

import (
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type targetPath struct {
	path  string
	label string
}

var credentialManagerAllowlist = []string{
	"git",
	"github",
	"gitlab",
	"bitbucket",
	"aws",
	"azure",
	"docker",
	"gcloud",
	"kube",
	"kubelogin",
	"npm",
	"terraform",
	"devops",
	"vault",
	"vscode",
	"vs code",
	"visual studio",
	"visualstudio",
	"code-insiders",
	"copilot",
	"github.copilot",
	"codex",
	"gemini",
	"claude",
	"antigravity",
}

func developerCredentialTargets(home string, fs FileSystem) []targetPath {
	targets := []targetPath{}
	for _, elems := range [][]string{
		{".aws", "credentials"},
		{".aws", "sso", "cache"},
		{".aws", "cli", "cache"},
		{".ansible", "galaxy_token"},
		{".azure", "msal_token_cache.json"},
		{".azure", "msal_token_cache.bin"},
		{".azure", "accessTokens.json"},
		{".azure", "service_principal_entries.json"},
		{".vault-token"},
		{".boto"},
		{".fly", "config.yml"},
		{".ngrok2", "ngrok.yml"},
		{".config", "ngrok", "ngrok.yml"},
		{".config", "stripe", "config.toml"},
		{".config", "gcloud", "credentials.db"},
		{".config", "gcloud", "access_tokens.db"},
		{".config", "gcloud", "application_default_credentials.json"},
		{".config", "gcloud", "legacy_credentials"},
		{".config", "gh", "hosts.yml"},
		{".config", "doctl", "config.yaml"},
		{".config", "helm", "registry", "config.json"},
		{".wrangler", "config", "default.toml"},
		{".supabase", "access-token"},
		{".oci"},
		{".mc", "config.json"},
		{".s3cfg"},
		{".pulumi", "credentials.json"},
		{".docker", "config.json"},
		{".git-credentials"},
		{".m2", "settings.xml"},
		{".m2", "settings-security.xml"},
		{".gradle", "gradle.properties"},
		{".cargo", "credentials"},
		{".cargo", "credentials.toml"},
		{".kube", "config"},
		{".kube", "cache"},
		{".netrc"},
		{".npmrc"},
		{".pypirc"},
		{".gem", "credentials"},
		{".composer", "auth.json"},
		{".nuget", "NuGet", "NuGet.Config"},
		{".erlang.cookie"},
		{".terraform.d", "credentials.tfrc.json"},
	} {
		targets = append(targets, targetPath{filepath.Join(append([]string{home}, elems...)...), targetLabelDeveloperCredential})
	}

	targets = append(targets,
		targetPath{filepath.Join(home, ".codex", "auth.json"), targetLabelAIToolCredential},
		targetPath{filepath.Join(home, ".claude", ".credentials.json"), targetLabelAIToolCredential},
		targetPath{filepath.Join(home, ".gemini", "oauth_creds.json"), targetLabelAIToolCredential},
	)

	targets = append(targets,
		targetPath{filepath.Join(home, ".config", "github-copilot", "hosts.json"), targetLabelCopilotCredential},
		targetPath{filepath.Join(home, ".config", "github-copilot", "apps.json"), targetLabelCopilotCredential},
	)

	if runtime.GOOS == osWindows {
		targets = append(targets,
			envTarget(fs, envAPPDATA, targetLabelDeveloperCredential, "gcloud", "credentials.db"),
			envTarget(fs, envAPPDATA, targetLabelDeveloperCredential, "gcloud", "access_tokens.db"),
			envTarget(fs, envAPPDATA, targetLabelDeveloperCredential, "gcloud", "application_default_credentials.json"),
			envTarget(fs, envAPPDATA, targetLabelDeveloperCredential, "gcloud", "legacy_credentials"),
			envTarget(fs, envAPPDATA, targetLabelDeveloperCredential, "GitHub CLI", "hosts.yml"),
			envTarget(fs, envLOCALAPPDATA, targetLabelCopilotCredential, "github-copilot", "hosts.json"),
			envTarget(fs, envLOCALAPPDATA, targetLabelCopilotCredential, "github-copilot", "apps.json"),
			envTarget(fs, envLOCALAPPDATA, targetLabelIDECredential, ".IdentityService"),
		)
	}

	return compactTargets(targets)
}

func fullToolResetTargets(home string, fs FileSystem) []targetPath {
	targets := []targetPath{
		{filepath.Join(home, ".aws", "config"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".ansible"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".azure"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".vercel"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".fly"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".config", "ngrok"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".config", "stripe"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".config", "gcloud"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".config", "gh"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".config", "doctl"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".cloudflare"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".wrangler"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".heroku"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".supabase"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".mc"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".packer.d"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".vagrant.d"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".localstack"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".snyk"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".tflint.d"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".krew"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".cache", "pre-commit"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".sonarlint"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".rd"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".docker", "buildx"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".colima"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".lima"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".gitconfig"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".yarnrc"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".mongorc.js"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".wget-hsts"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".bun"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".deno"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".cache", "helm"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".config", "helm"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".minikube"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".config", "k9s"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".local", "state", "k9s"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".jupyter"), targetLabelDeveloperConfig},
		{filepath.Join(home, ".codex"), targetLabelAITool},
		{filepath.Join(home, ".config", "gemini"), targetLabelAITool},
		{filepath.Join(home, ".gemini"), targetLabelAITool},
		{filepath.Join(home, ".antigravity"), targetLabelAITool},
		{filepath.Join(home, ".claude"), targetLabelAITool},
	}
	if runtime.GOOS == osWindows {
		targets = append(targets,
			envTarget(fs, envAPPDATA, targetLabelDeveloperConfig, "gcloud"),
			envTarget(fs, envAPPDATA, targetLabelDeveloperConfig, "GitHub CLI"),
			envTarget(fs, envAPPDATA, targetLabelAITool, "Claude"),
			envTarget(fs, envAPPDATA, targetLabelAITool, "Codex"),
			envTarget(fs, envAPPDATA, targetLabelAITool, "Antigravity"),
			envTarget(fs, envLOCALAPPDATA, targetLabelAITool, "Claude"),
		)
	}

	targets = append(targets, ideAndCopilotTargets(home, fs)...)

	return compactTargets(targets)
}

func ideAndCopilotTargets(home string, fs FileSystem) []targetPath {
	if runtime.GOOS == osWindows {
		targets := []targetPath{
			envTarget(fs, envAPPDATA, targetLabelIDEAuthCacheData, "Microsoft", "VisualStudio"),
			envTarget(fs, envLOCALAPPDATA, targetLabelIDEAuthCacheData, "Microsoft", "VisualStudio"),
			envTarget(fs, envLOCALAPPDATA, targetLabelIDEAuthCacheData, "Microsoft", "VSCommon"),
			envTarget(fs, envLOCALAPPDATA, targetLabelIDEAuthCacheData, "Microsoft", "VisualStudio Services"),
			envTarget(fs, envLOCALAPPDATA, targetLabelIDEAuthCacheData, "Microsoft", "Team Foundation"),
			envTarget(fs, envAPPDATA, targetLabelCopilotAuthCacheData, "GitHub Copilot"),
			envTarget(fs, envLOCALAPPDATA, targetLabelCopilotAuthCacheData, "GitHub Copilot"),
			envTarget(fs, envAPPDATA, targetLabelCopilotAuthCacheData, "github-copilot"),
			envTarget(fs, envLOCALAPPDATA, targetLabelCopilotAuthCacheData, "github-copilot"),
		}

		for _, product := range []string{"Code", "Code - Insiders", "VSCodium"} {
			targets = append(targets, windowsVSCodeTargets(fs, product)...)
		}

		return compactTargets(append(targets, commonCopilotTargets(home)...))
	}

	if runtime.GOOS == "darwin" {
		targets := []targetPath{
			{filepath.Join(home, "Library", "Application Support", "VisualStudio"), targetLabelIDEAuthCacheData},
			{filepath.Join(home, "Library", "Caches", "VisualStudio"), targetLabelIDEAuthCacheData},
			{filepath.Join(home, "Library", "Preferences", "VisualStudio"), targetLabelIDEAuthCacheData},
			{filepath.Join(home, "Library", "Application Support", "GitHub Copilot"), targetLabelCopilotAuthCacheData},
			{filepath.Join(home, "Library", "Caches", "GitHub Copilot"), targetLabelCopilotAuthCacheData},
		}

		for _, product := range []vsCodeProduct{
			{
				dataDir:        filepath.Join(home, "Library", "Application Support", "Code"),
				cacheDir:       filepath.Join(home, "Library", "Caches", "com.microsoft.VSCode"),
				savedStateDir:  filepath.Join(home, "Library", "Saved Application State", "com.microsoft.VSCode.savedState"),
				extensionsRoot: filepath.Join(home, ".vscode"),
			},
			{
				dataDir:        filepath.Join(home, "Library", "Application Support", "Code - Insiders"),
				cacheDir:       filepath.Join(home, "Library", "Caches", "com.microsoft.VSCodeInsiders"),
				savedStateDir:  filepath.Join(home, "Library", "Saved Application State", "com.microsoft.VSCodeInsiders.savedState"),
				extensionsRoot: filepath.Join(home, ".vscode-insiders"),
			},
			{
				dataDir:        filepath.Join(home, "Library", "Application Support", "VSCodium"),
				cacheDir:       filepath.Join(home, "Library", "Caches", "com.vscodium"),
				savedStateDir:  filepath.Join(home, "Library", "Saved Application State", "com.vscodium.savedState"),
				extensionsRoot: filepath.Join(home, ".vscode-oss"),
			},
		} {
			targets = append(targets, vsCodeTargets(product)...)
		}

		return compactTargets(append(targets, commonCopilotTargets(home)...))
	}

	targets := []targetPath{
		{filepath.Join(home, ".config", "GitHub Copilot"), targetLabelCopilotAuthCacheData},
		{filepath.Join(home, ".cache", "GitHub Copilot"), targetLabelCopilotAuthCacheData},
	}

	for _, product := range []vsCodeProduct{
		{
			dataDir:        filepath.Join(home, ".config", "Code"),
			cacheDir:       filepath.Join(home, ".cache", "Code"),
			extensionsRoot: filepath.Join(home, ".vscode"),
		},
		{
			dataDir:        filepath.Join(home, ".config", "Code - Insiders"),
			cacheDir:       filepath.Join(home, ".cache", "Code - Insiders"),
			extensionsRoot: filepath.Join(home, ".vscode-insiders"),
		},
		{
			dataDir:        filepath.Join(home, ".config", "VSCodium"),
			cacheDir:       filepath.Join(home, ".cache", "VSCodium"),
			extensionsRoot: filepath.Join(home, ".vscode-oss"),
		},
	} {
		targets = append(targets, vsCodeTargets(product)...)
	}

	return compactTargets(append(targets, commonCopilotTargets(home)...))
}

type vsCodeProduct struct {
	dataDir        string
	cacheDir       string
	savedStateDir  string
	extensionsRoot string
}

func windowsVSCodeTargets(fs FileSystem, product string) []targetPath {
	dataDir := envTarget(fs, envAPPDATA, targetLabelIDEAuthCacheData, product).path
	return vsCodeTargets(vsCodeProduct{
		dataDir:        dataDir,
		cacheDir:       dataDir,
		extensionsRoot: "",
	})
}

func vsCodeTargets(product vsCodeProduct) []targetPath {
	var targets []targetPath

	if strings.TrimSpace(product.dataDir) != "" {
		targets = append(targets,
			targetPath{filepath.Join(product.dataDir, "User", "globalStorage"), targetLabelVSCodeGlobalState},
			targetPath{filepath.Join(product.dataDir, "User", "workspaceStorage"), targetLabelIDEAuthCacheData},
			targetPath{filepath.Join(product.dataDir, "User", "History"), targetLabelIDEAuthCacheData},
			targetPath{filepath.Join(product.dataDir, "Backups"), targetLabelIDEAuthCacheData},
		)

		for _, dir := range []string{"Cache", "CachedData", "Code Cache", "GPUCache", "Service Worker", "logs"} {
			targets = append(targets, targetPath{filepath.Join(product.dataDir, dir), targetLabelIDEAuthCacheData})
		}
	}

	if strings.TrimSpace(product.cacheDir) != "" && cleanComparablePath(product.cacheDir) != cleanComparablePath(product.dataDir) {
		targets = append(targets, targetPath{product.cacheDir, targetLabelIDEAuthCacheData})
	}

	if strings.TrimSpace(product.savedStateDir) != "" {
		targets = append(targets, targetPath{product.savedStateDir, targetLabelIDEAuthCacheData})
	}

	if strings.TrimSpace(product.extensionsRoot) != "" {
		targets = append(targets,
			targetPath{filepath.Join(product.extensionsRoot, "extensions", "github.copilot"), targetLabelCopilotAuthCacheData},
			targetPath{filepath.Join(product.extensionsRoot, "extensions", "github.copilot-chat"), targetLabelCopilotAuthCacheData},
		)
	}

	return compactTargets(targets)
}

func commonCopilotTargets(home string) []targetPath {
	return []targetPath{
		{filepath.Join(home, ".copilot"), targetLabelCopilotAuthCacheData},
		{filepath.Join(home, ".github-copilot"), targetLabelCopilotAuthCacheData},
		{filepath.Join(home, ".config", "github-copilot"), targetLabelCopilotAuthCacheData},
		{filepath.Join(home, ".cache", "github-copilot"), targetLabelCopilotAuthCacheData},
	}
}

func sshTargets(home string, fs FileSystem) []targetPath {
	targets := []targetPath{
		{filepath.Join(home, ".ssh", "config"), targetLabelSSHClientConfig},
		{filepath.Join(home, ".ssh", "known_hosts"), targetLabelSSHKnownHosts},
	}

	matches, err := fs.Glob(filepath.Join(home, ".ssh", "id_*"))
	if err == nil {
		sort.Strings(matches)
		for _, match := range matches {
			if info, statErr := fs.Lstat(match); statErr == nil && !info.IsDir() {
				targets = append(targets, targetPath{match, targetLabelSSHKey})
			}
		}
	}

	if len(targets) == 2 {
		targets = append(targets, targetPath{filepath.Join(home, ".ssh", "id_*"), targetLabelSSHKey})
	}

	return targets
}

func historyTargets(home string, fs FileSystem) []targetPath {
	targets := []targetPath{
		{filepath.Join(home, ".bash_history"), targetLabelShellToolHistory},
		{filepath.Join(home, ".zsh_history"), targetLabelShellToolHistory},
		{filepath.Join(home, ".python_history"), targetLabelShellToolHistory},
		{filepath.Join(home, ".node_repl_history"), targetLabelShellToolHistory},
		{filepath.Join(home, ".sqlite_history"), targetLabelShellToolHistory},
		{filepath.Join(home, ".psql_history"), targetLabelShellToolHistory},
		{filepath.Join(home, ".mysql_history"), targetLabelShellToolHistory},
		{filepath.Join(home, ".rediscli_history"), targetLabelShellToolHistory},
		{filepath.Join(home, ".dbshell"), targetLabelShellToolHistory},
		{filepath.Join(home, ".lesshst"), targetLabelShellToolHistory},
		{filepath.Join(home, ".local", "share", "fish", "fish_history"), targetLabelShellToolHistory},
		{filepath.Join(home, ".config", "fish", "fish_history"), targetLabelShellToolHistory},
		{filepath.Join(home, ".local", "share", "powershell", "PSReadLine", "ConsoleHost_history.txt"), targetLabelShellToolHistory},
		{filepath.Join(home, ".irb_history"), targetLabelShellToolHistory},
		{filepath.Join(home, ".php_history"), targetLabelShellToolHistory},
		{filepath.Join(home, ".snowsql", "history"), targetLabelShellToolHistory},
		{filepath.Join(home, ".cassandra", "cqlsh_history"), targetLabelShellToolHistory},
		{filepath.Join(home, ".gdb_history"), targetLabelShellToolHistory},
		{filepath.Join(home, ".lldb-history"), targetLabelShellToolHistory},
	}

	if runtime.GOOS == osWindows {
		targets = append(targets,
			envTarget(fs, envAPPDATA, targetLabelShellToolHistory, "Microsoft", "Windows", "PowerShell", "PSReadLine", "ConsoleHost_history.txt"),
			envTarget(fs, envAPPDATA, targetLabelShellToolHistory, "Microsoft", "Windows", "PowerShell", "PSReadLine", "Visual Studio Code Host_history.txt"),
		)
	}

	return compactTargets(targets)
}

func browserCacheTargets(home string, fs FileSystem) []targetPath {
	if runtime.GOOS == osWindows {
		return compactTargets([]targetPath{
			envTarget(fs, envLOCALAPPDATA, targetLabelBrowserCache, "Google", "Chrome", "User Data", "Default", "Cache"),
			envTarget(fs, envLOCALAPPDATA, targetLabelBrowserCache, "Google", "Chrome", "User Data", "Default", "Code Cache"),
			envTarget(fs, envLOCALAPPDATA, targetLabelBrowserCache, "Microsoft", "Edge", "User Data", "Default", "Cache"),
			envTarget(fs, envLOCALAPPDATA, targetLabelBrowserCache, "Microsoft", "Edge", "User Data", "Default", "Code Cache"),
			envTarget(fs, envLOCALAPPDATA, targetLabelBrowserCache, "BraveSoftware", "Brave-Browser", "User Data", "Default", "Cache"),
			envTarget(fs, envLOCALAPPDATA, targetLabelBrowserCache, "BraveSoftware", "Brave-Browser", "User Data", "Default", "Code Cache"),
			envTarget(fs, envLOCALAPPDATA, targetLabelBrowserCache, "CocCoc", "Browser", "User Data", "Default", "Cache"),
			envTarget(fs, envLOCALAPPDATA, targetLabelBrowserCache, "CocCoc", "Browser", "User Data", "Default", "Code Cache"),
			envTarget(fs, envLOCALAPPDATA, targetLabelBrowserCache, "Mozilla", "Firefox", "Profiles"),
		})
	}

	var targets []targetPath
	for _, dir := range linuxChromiumBrowserDirs(home, "cache") {
		targets = append(targets, targetPath{dir, targetLabelBrowserCache})
	}

	return append(targets, []targetPath{
		{filepath.Join(home, ".cache", "mozilla", "firefox"), targetLabelBrowserCache},
		{filepath.Join(home, ".var", "app", flatpakFirefoxID, "cache", "mozilla", "firefox"), targetLabelBrowserCache},
		{filepath.Join(home, "Library", "Caches", "Google", "Chrome"), targetLabelBrowserCache},
		{filepath.Join(home, "Library", "Caches", "Microsoft Edge"), targetLabelBrowserCache},
		{filepath.Join(home, "Library", "Caches", "BraveSoftware", "Brave-Browser"), targetLabelBrowserCache},
		{filepath.Join(home, "Library", "Caches", "Firefox"), targetLabelBrowserCache},
		{filepath.Join(home, "Library", "Caches", "com.apple.Safari"), targetLabelBrowserCache},
	}...)
}

var linuxChromiumBrowsers = []struct {
	flatpakID string
	dir       []string
}{
	{"com.google.Chrome", []string{"google-chrome"}},
	{"org.chromium.Chromium", []string{"chromium"}},
	{"com.microsoft.Edge", []string{"microsoft-edge"}},
	{"com.brave.Browser", []string{"BraveSoftware", "Brave-Browser"}},
}

func linuxChromiumBrowserDirs(home string, kind string) []string {
	var dirs []string
	for _, browser := range linuxChromiumBrowsers {
		dirs = append(dirs, filepath.Join(append([]string{home, "." + kind}, browser.dir...)...))
	}
	for _, browser := range linuxChromiumBrowsers {
		dirs = append(dirs, filepath.Join(append([]string{home, ".var", "app", browser.flatpakID, kind}, browser.dir...)...))
	}
	return dirs
}

func browserProfileTargets(home string, fs FileSystem) []targetPath {
	var targets []targetPath

	if runtime.GOOS == osWindows {
		for _, root := range compactTargets([]targetPath{
			envTarget(fs, envLOCALAPPDATA, targetLabelBrowserProfileRoot, "Google", "Chrome", "User Data"),
			envTarget(fs, envLOCALAPPDATA, targetLabelBrowserProfileRoot, "Microsoft", "Edge", "User Data"),
			envTarget(fs, envLOCALAPPDATA, targetLabelBrowserProfileRoot, "BraveSoftware", "Brave-Browser", "User Data"),
			envTarget(fs, envLOCALAPPDATA, targetLabelBrowserProfileRoot, "CocCoc", "Browser", "User Data"),
		}) {
			targets = append(targets, browserProfileChildren(root.path, fs)...)
		}

		targets = append(targets,
			envTarget(fs, envAPPDATA, targetLabelFirefoxBrowserProfiles, "Mozilla", "Firefox", "Profiles"),
			envTarget(fs, envLOCALAPPDATA, targetLabelFirefoxBrowserProfiles, "Mozilla", "Firefox", "Profiles"),
		)
		return compactTargets(targets)
	}

	for _, root := range append(linuxChromiumBrowserDirs(home, "config"),
		filepath.Join(home, "Library", "Application Support", "Google", "Chrome"),
		filepath.Join(home, "Library", "Application Support", "Microsoft Edge"),
		filepath.Join(home, "Library", "Application Support", "BraveSoftware", "Brave-Browser"),
	) {
		targets = append(targets, browserProfileChildren(root, fs)...)
	}

	targets = append(targets,
		targetPath{filepath.Join(home, ".mozilla", "firefox"), targetLabelBrowserProfile},
		targetPath{filepath.Join(home, ".var", "app", flatpakFirefoxID, ".mozilla", "firefox"), targetLabelBrowserProfile},
		targetPath{filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles"), targetLabelBrowserProfile},
		targetPath{filepath.Join(home, "Library", "Safari"), targetLabelBrowserProfile},
	)

	return compactTargets(targets)
}

func browserProfileChildren(root string, fs FileSystem) []targetPath {
	entries, err := fs.ReadDir(root)
	if err != nil {
		return []targetPath{{root, targetLabelBrowserProfileRoot}}
	}

	var targets []targetPath
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if name == "Default" || strings.HasPrefix(name, "Profile ") {
			targets = append(targets, targetPath{filepath.Join(root, name), targetLabelBrowserProfile})
		}
	}

	if len(targets) == 0 {
		targets = append(targets, targetPath{root, targetLabelBrowserProfileRoot})
	}

	return targets
}

func envTarget(fs FileSystem, envName string, label string, elems ...string) targetPath {
	root := fs.Getenv(envName)
	if strings.TrimSpace(root) == "" {
		return targetPath{}
	}

	parts := append([]string{root}, elems...)
	return targetPath{filepath.Join(parts...), label}
}

func compactTargets(targets []targetPath) []targetPath {
	out := targets[:0]
	for _, target := range targets {
		if strings.TrimSpace(target.path) == "" {
			continue
		}
		out = append(out, target)
	}
	return out
}
