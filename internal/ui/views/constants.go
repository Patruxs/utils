package views

import "time"

const (
	osWindows = "windows"

	optionSSHKeys           = "ssh_keys"
	optionBrowserProfiles   = "browser_profiles"
	optionCredentialManager = "credential_manager"
	optionForceStop         = "force_stop"

	cleanerTitle        = "System & Credential Cleaner"
	cleanerSubtitle     = "Dry-run-first cleanup for local developer credentials, shell history, and user-profile caches."
	cleanerBaselineNote = "Always included: dev credentials/configs, AI tool configs/caches, shell/tool histories, IDE auth/cache/data/history, and browser caches. Dry-run only lists deletions; execute deletes matching files. Force-stop acts in both modes."
	cleanerSafetyNotice = "Targets stay under the current user profile. Admin/root elevation is never requested."

	cleanerRunTimeout = 5 * time.Minute

	defaultLogViewportHeight     = 12
	cleanerOptionsReservedHeight = 10
	cleanerOptionsMinHeight      = 3
	cleanerLogReservedHeight     = 18
	cleanerLogMinHeight          = 5
	cleanerLogMaxHeight          = 20
)
