package common

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const countSeparator = " · "

type Count struct {
	Label    string
	Singular string
	N        int
	Tone     Tone
}

func (c Count) text() string {
	if c.N == 1 && c.Singular != "" {
		return fmt.Sprintf("%d %s", c.N, c.Singular)
	}
	return fmt.Sprintf("%d %s", c.N, c.Label)
}

func RenderCounts(width int, counts []Count) string {
	separator := Muted.Render(countSeparator)
	var lines []string
	current := ""
	for _, count := range counts {
		item := Muted.Render(count.text())
		if count.N != 0 {
			item = count.Tone.Style().Bold(true).Render(count.text())
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
