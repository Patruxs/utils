package common

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const countSeparator = " · "

type Count struct {
	Label string
	N     int
	Tone  Tone
}

func RenderCounts(width int, counts []Count) string {
	separator := Muted.Render(countSeparator)
	var lines []string
	current := ""
	for _, count := range counts {
		item := Muted.Render(fmt.Sprintf("%d %s", count.N, count.Label))
		if count.N != 0 {
			item = count.Tone.Style().Bold(true).Render(fmt.Sprintf("%d %s", count.N, count.Label))
		}
		if current != "" && lipgloss.Width(current)+lipgloss.Width(countSeparator)+lipgloss.Width(item) > width {
			lines = append(lines, current)
			current = ""
		}
		if current != "" {
			current += separator
		}
		current += item
	}
	if current != "" {
		lines = append(lines, current)
	}
	for i, line := range lines {
		lines[i] = Truncate(line, width)
	}
	return strings.Join(lines, "\n")
}
