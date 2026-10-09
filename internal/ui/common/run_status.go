package common

import (
	"fmt"
	"strings"
	"time"
)

func RunStatus(width int, spinnerView, label, note string, elapsed time.Duration) string {
	panel := Panel{Title: "Running", Width: width}
	innerWidth := panel.InnerWidth()

	lines := []string{SpreadLine(innerWidth, spinnerView+" "+label, Muted.Render(FormatElapsed(elapsed)))}
	if note != "" {
		for _, line := range WrapLine("", note, innerWidth) {
			lines = append(lines, Muted.Render(line))
		}
	}
	return panel.Render(strings.Join(lines, "\n"))
}

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
