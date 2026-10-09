package cleaner

import (
	"os"
	"time"
)

const (
	osWindows = "windows"

	commandWaitDelay = 3 * time.Second

	userPrivateDirPerm  os.FileMode = 0o700
	userPrivateFilePerm os.FileMode = 0o600

	commandTasklist = "tasklist.exe"
	commandTaskkill = "taskkill"
	commandPgrep    = "pgrep"
	commandPkill    = "pkill"
	commandCmdkey   = "cmdkey.exe"

	commandArgTasklistFormat   = "/FO"
	commandArgTasklistCSV      = "CSV"
	commandArgTasklistNoHeader = "/NH"
	commandArgTaskkillForce    = "/F"
	commandArgTaskkillImage    = "/IM"
	commandArgExactProcess     = "-x"
	commandArgCmdkeyList       = "/list"
	commandArgCmdkeyDelete     = "/delete:"

	cmdkeyTargetPrefix  = "Target:"
	logAttrCleanupLevel = "cleanup_level"

	envAPPDATA      = "APPDATA"
	envLOCALAPPDATA = "LOCALAPPDATA"

	targetLabelDeveloperCredential    = "developer credential"
	targetLabelAIToolCredential       = "AI tool credential"
	targetLabelIDECredential          = "IDE sign-in data"
	targetLabelCopilotCredential      = "Copilot sign-in token"
	targetLabelDeveloperConfig        = "developer tool folder/config"
	targetLabelAITool                 = "AI tool credential/config/cache"
	targetLabelIDEAuthCacheData       = "IDE authentication/cache/data"
	targetLabelCopilotAuthCacheData   = "Copilot authentication/cache/data"
	targetLabelSSHClientConfig        = "SSH client config"
	targetLabelSSHKnownHosts          = "SSH known hosts"
	targetLabelSSHKey                 = "SSH private/public key"
	targetLabelShellToolHistory       = "shell/tool history"
	targetLabelBrowserCache           = "browser cache"
	targetLabelBrowserProfile         = "browser profile"
	targetLabelBrowserProfileRoot     = "browser profile root"
	targetLabelFirefoxBrowserProfiles = "Firefox browser profiles"
)
