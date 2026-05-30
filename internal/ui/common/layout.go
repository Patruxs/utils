package common

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Layout struct {
	Width  int
	Height int
}

func NewLayout(width, height int) Layout {
	if width <= 0 {
		width = DefaultContentWidth
	}
	if height <= 0 {
		height = DefaultContentHeight
	}

	return Layout{
		Width:  MaxInt(MinContentWidth, width),
		Height: MaxInt(1, height),
	}
}

func ContentWindowSize(msg tea.WindowSizeMsg, reservedHeight int) tea.WindowSizeMsg {
	return tea.WindowSizeMsg{
		Width:  ContentWidth(msg.Width),
		Height: MaxInt(1, ContentHeight(msg.Height)-reservedHeight),
	}
}

func ContentWidth(width int) int {
	return MaxInt(MinContentWidth, width-AppHorizontalPadding)
}

func ContentHeight(height int) int {
	return MaxInt(1, height-AppVerticalPadding)
}

func (l Layout) ListHeight(itemCount, reservedHeight, minHeight int) int {
	height := MaxInt(1, itemCount)
	if l.Height > 0 {
		height = MinInt(height, MaxInt(minHeight, l.Height-reservedHeight))
	}
	return height
}

func (l Layout) ViewportHeight(reservedHeight, minHeight, maxHeight int) int {
	height := maxHeight
	if l.Height > 0 {
		height = MinInt(maxHeight, MaxInt(minHeight, l.Height-reservedHeight))
	}
	return height
}

func (l Layout) RenderWrapped(text string, render func(...string) string) string {
	return RenderWrapped(l.Width, text, render)
}

func (l Layout) WrapLine(prefix, text string) []string {
	return WrapLine(prefix, text, l.Width)
}

func RenderPlainWrapped(width int, text string) string {
	return strings.Join(WrapPlain(text, width), "\n")
}

func RenderWrapped(width int, text string, render func(...string) string) string {
	lines := WrapPlain(text, width)
	if len(lines) == 0 {
		return render("")
	}

	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		rendered = append(rendered, render(line))
	}
	return strings.Join(rendered, "\n")
}

func WrapLine(prefix, text string, width int) []string {
	if width <= 0 {
		width = DefaultContentWidth
	}

	prefixWidth := len([]rune(prefix))
	if prefixWidth >= width {
		return WrapPlain(prefix+text, width)
	}

	firstWidth := MaxInt(1, width-prefixWidth)
	continuation := strings.Repeat(" ", prefixWidth)
	continuationWidth := MaxInt(1, width-prefixWidth)

	runes := []rune(text)
	if len(runes) == 0 {
		return []string{prefix}
	}

	lines := make([]string, 0, (len(runes)/firstWidth)+1)
	linePrefix := prefix
	lineWidth := firstWidth
	for len(runes) > lineWidth {
		cut := wrapCut(runes, lineWidth)
		lines = append(lines, linePrefix+strings.TrimRight(string(runes[:cut]), " "))
		runes = trimLeadingSpaces(runes[cut:])
		linePrefix = continuation
		lineWidth = continuationWidth
	}
	if len(runes) > 0 {
		lines = append(lines, linePrefix+string(runes))
	}

	return lines
}

func WrapPlain(text string, width int) []string {
	if width <= 0 {
		width = DefaultContentWidth
	}

	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	lines := make([]string, 0, (len(runes)/width)+1)
	for len(runes) > width {
		cut := wrapCut(runes, width)
		lines = append(lines, strings.TrimRight(string(runes[:cut]), " "))
		runes = trimLeadingSpaces(runes[cut:])
	}
	if len(runes) > 0 {
		lines = append(lines, string(runes))
	}

	return lines
}

func wrapCut(runes []rune, width int) int {
	cut := MinInt(width, len(runes))
	for i := cut; i > 0; i-- {
		if runes[i-1] == ' ' {
			return i
		}
	}
	return cut
}

func trimLeadingSpaces(runes []rune) []rune {
	for len(runes) > 0 && runes[0] == ' ' {
		runes = runes[1:]
	}
	return runes
}

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
