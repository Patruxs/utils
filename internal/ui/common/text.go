package common

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const ellipsis = "…"

func Truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width == 1 {
		return ellipsis
	}
	return lipgloss.NewStyle().MaxWidth(width-1).Render(text) + ellipsis
}

func PadRight(text string, width int) string {
	gap := width - lipgloss.Width(text)
	if gap <= 0 {
		return text
	}
	return text + strings.Repeat(" ", gap)
}

func SpreadLine(width int, left, right string) string {
	rightWidth := lipgloss.Width(right)
	if right == "" {
		return Truncate(left, width)
	}
	if rightWidth+1 >= width {
		return Truncate(left, width)
	}
	left = Truncate(left, width-rightWidth-1)
	gap := width - lipgloss.Width(left) - rightWidth
	return left + strings.Repeat(" ", gap) + right
}

func WrapStyled(text string, width int) []string {
	if width <= 0 {
		return nil
	}
	if text == "" {
		return []string{""}
	}
	wrapped := lipgloss.NewStyle().Width(width).Render(text)
	lines := strings.Split(wrapped, "\n")
	for i, line := range lines {
		lines[i] = Truncate(strings.TrimRight(line, " "), width)
	}
	return lines
}
