package common

import (
	"fmt"
	"time"
)

func FormatElapsed(elapsed time.Duration) string {
	seconds := int(elapsed.Round(time.Second) / time.Second)
	if seconds < 0 {
		seconds = 0
	}
	if seconds >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}
