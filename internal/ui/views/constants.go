package views

import "time"

const (
	cleanerScopeWidth        = 41
	cleanerPaneMinHeight     = 3
	cleanerBarMinWidth       = 10
	cleanerDefaultBodyHeight = 21
	cleanerEventBuffer       = 64

	cleanerLogPrefix     = "offboarding-cleanup-"
	cleanerLogTimeLayout = "20060102-150405"
	cleanerLogSuffix     = ".log"
	cleanerStoppedPrefix = "Force stopped target process: "
	cleanerReminder      = "Revoke remote sessions, PATs, SSH keys, API keys and SSO sessions from their admin portals."

	cleanerScanTimeout = 30 * time.Second
	cleanerRunTimeout  = 5 * time.Minute
)
