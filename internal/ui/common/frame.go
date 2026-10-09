package common

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/lipgloss"
)

const (
	MarginX             = 1
	MinWidth            = 40
	MinHeight           = 10
	HeaderRuleMinHeight = 20
	appName             = "UTILS"
	appTagline          = "Developer Hub"
	crumbSeparator      = " › "
)

func HeaderHeight(height int) int {
	if height < HeaderRuleMinHeight {
		return 1
	}
	return 2
}

func Header(width, height int, crumbs []string, version string) string {
	left := Title.Render(appName)
	if len(crumbs) == 0 {
		left += "  " + Muted.Render(appTagline)
	}
	for i, crumb := range crumbs {
		if i == len(crumbs)-1 {
			crumb = lipgloss.NewStyle().Bold(true).Render(crumb)
		}
		left += Muted.Render(crumbSeparator) + crumb
	}

	right := ""
	if version != "" {
		right = Muted.Render("version " + version)
	}
	if lipgloss.Width(left)+lipgloss.Width(right)+1 > width {
		right = ""
	}

	line := SpreadLine(width, left, right)
	if HeaderHeight(height) < 2 {
		return line
	}
	return line + "\n" + lipgloss.NewStyle().Foreground(ColorBorder).Render(strings.Repeat("─", MaxInt(0, width)))
}

func Footer(width int, keys help.KeyMap, status string) string {
	if width <= 0 {
		return ""
	}
	status = Truncate(status, width)
	hintsWidth := width
	if status != "" {
		hintsWidth = width - lipgloss.Width(status) - 1
	}

	hints := ""
	if keys != nil && hintsWidth > 0 {
		helpModel := NewHelpModel()
		helpModel.Width = hintsWidth
		hints = Truncate(helpModel.ShortHelpView(keys.ShortHelp()), hintsWidth)
	}
	return SpreadLine(width, hints, status)
}

func Pill(text string, tone Tone) string {
	return tone.Style().Bold(true).Reverse(true).Render(" " + text + " ")
}

func Notice(width int, tone Tone, text string) string {
	prefix := ""
	if glyph := tone.Glyph(); glyph != "" {
		prefix = glyph + " "
	}
	style := tone.Style()
	lines := WrapLine(prefix, text, width)
	for i, line := range lines {
		lines[i] = style.Render(Truncate(line, width))
	}
	return strings.Join(lines, "\n")
}

func FitHeight(body string, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(body, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func TooSmall(width, height int) (string, bool) {
	if width >= MinWidth && height >= MinHeight {
		return "", false
	}
	message := fmt.Sprintf("Terminal too small: need %d×%d, have %d×%d", MinWidth, MinHeight, width, height)
	lines := WrapPlain(message, MaxInt(1, width))
	lines = append(lines, Truncate(Muted.Render("ctrl+c quit"), MaxInt(1, width)))
	return strings.Join(lines, "\n"), true
}
