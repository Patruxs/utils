package views

import "time"

const (
	osWindows = "windows"

	optionSSHKeys           = "ssh_keys"
	optionBrowserProfiles   = "browser_profiles"
	optionCredentialManager = "credential_manager"
	optionForceStop         = "force_stop"
	optionShellHistory      = "shell_history"
	optionFullToolReset     = "full_tool_reset"

	cleanerTitle        = "System & Credential Cleaner"
	cleanerSubtitle     = "Dry-run-first cleanup for local developer credentials, shell history, and user-profile caches."
	cleanerBaselineNote = "Always included: credential and token files for cloud, Git, package-manager, and AI tools, and IDE and Copilot sign-in data. Histories, browser data, and tool folders are kept unless their option is on. Dry-run only lists deletions; execute deletes matching files. Force-stop acts in both modes."
	cleanerSafetyNotice = "Deletes only inside the current user profile, never through a link that leads outside it. Admin/root elevation is never requested."

	cleanerRunTimeout = 5 * time.Minute

	runCanceledText = "Canceled. The run stopped early; the activity above shows what it did before it stopped."

	defaultLogViewportHeight     = 12
	cleanerOptionsReservedHeight = 10
	cleanerOptionsMinHeight      = 3
	cleanerLogReservedHeight     = 18
	cleanerLogMinHeight          = 5
	cleanerLogMaxHeight          = 20
)
