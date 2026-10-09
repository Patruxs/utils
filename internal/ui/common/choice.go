package common

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const buttonGap = "   "

type Choice struct {
	Label  string
	Detail string
}

func RenderChoices(width int, choices []Choice, selected int) string {
	labelWidth := 0
	for _, choice := range choices {
		labelWidth = MaxInt(labelWidth, lipgloss.Width(choice.Label))
	}

	lines := make([]string, 0, len(choices))
	for i, choice := range choices {
		cursor := "  "
		label := PadRight(choice.Label, labelWidth)
		if i == selected {
			cursor = Accent.Render(ToneAccent.Glyph()) + " "
			label = lipgloss.NewStyle().Bold(true).Render(label)
		}
		line := cursor + label
		if choice.Detail != "" {
			line += buttonGap + Muted.Render(choice.Detail)
		}
		lines = append(lines, Truncate(line, width))
	}
	return strings.Join(lines, "\n")
}

func RenderButtons(labels []string, selected int) string {
	buttons := make([]string, 0, len(labels))
	for i, label := range labels {
		if i == selected {
			buttons = append(buttons, Selected.Render("[ "+label+" ]"))
			continue
		}
		buttons = append(buttons, "  "+label+"  ")
	}
	return strings.Join(buttons, buttonGap)
}

func ConfirmDialog(width int, title string, lines []string, buttons []string, selected int) string {
	panel := Panel{Title: "⚠ " + title, Variant: PanelDanger, Width: width}
	innerWidth := panel.InnerWidth()

	body := make([]string, 0, len(lines)+2)
	for _, line := range lines {
		body = append(body, wrapBullet(line, innerWidth)...)
	}
	if len(buttons) > 0 {
		body = append(body, "", Truncate("  "+RenderButtons(buttons, selected), innerWidth))
	}
	return panel.Render(strings.Join(body, "\n"))
}

func wrapBullet(line string, width int) []string {
	const bullet = "• "
	if !strings.HasPrefix(line, bullet) || width <= len(bullet) {
		return WrapStyled(line, width)
	}
	wrapped := WrapStyled(strings.TrimPrefix(line, bullet), width-lipgloss.Width(bullet))
	for i := range wrapped {
		if i == 0 {
			wrapped[i] = bullet + wrapped[i]
			continue
		}
		wrapped[i] = "  " + wrapped[i]
	}
	return wrapped
}
