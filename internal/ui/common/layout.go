package common

import "strings"

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
