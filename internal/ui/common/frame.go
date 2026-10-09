package common

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

const (
	MarginX         = 1
	MinWidth        = 50
	MinHeight       = 10
	headerRows      = 2
	wordmarkGlyph   = "◆"
	wordmarkName    = "UTILS"
	wordmarkGap     = "  "
	tabGap          = " "
	hintSeparator   = " · "
	infoSeparator   = " · "
	headerMinGap    = 2
	ruleLight       = "─"
	ruleHeavy       = "━"
	ruleHeavyStart  = "╴"
	ruleHeavyFinish = "╶"
)

func HeaderHeight(int) int {
	return headerRows
}

func Wordmark() string {
	return Accent.Render(wordmarkGlyph) + " " + lipgloss.NewStyle().Bold(true).Render(wordmarkName)
}

type tabCell struct {
	start int
	end   int
}

func tabCells(tabs []string) []tabCell {
	cells := make([]tabCell, 0, len(tabs))
	x := lipgloss.Width(wordmarkGlyph + " " + wordmarkName)
	for i, tab := range tabs {
		x += len(tabGap)
		if i == 0 {
			x += len(wordmarkGap) - len(tabGap)
		}
		width := lipgloss.Width(tab) + 2
		cells = append(cells, tabCell{start: x, end: x + width})
		x += width
	}
	return cells
}

func TabAt(tabs []string, x int) int {
	for i, cell := range tabCells(tabs) {
		if x >= cell.start && x < cell.end {
			return i
		}
	}
	return -1
}

func Header(width int, tabs []string, active int, infoChoices ...string) string {
	left := Wordmark()
	for i, tab := range tabs {
		gap := tabGap
		if i == 0 {
			gap = wordmarkGap
		}
		label := Muted.Render(" " + tab + " ")
		if i == active {
			label = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent).Render(" " + tab + " ")
		}
		left += gap + label
	}

	right := ""
	for _, info := range infoChoices {
		if lipgloss.Width(left)+headerMinGap+lipgloss.Width(info) <= width {
			right = Muted.Render(info)
			break
		}
	}
	line := Truncate(left, width)
	if right != "" {
		line = SpreadLine(width, left, right)
	}
	return line + "\n" + headerRule(width, tabs, active)
}

func headerRule(width int, tabs []string, active int) string {
	if width <= 0 {
		return ""
	}
	cells := tabCells(tabs)
	if active < 0 || active >= len(cells) || cells[active].end+1 > width {
		return Muted.Render(strings.Repeat(ruleLight, width))
	}
	cell := cells[active]
	before := Muted.Render(strings.Repeat(ruleLight, MaxInt(0, cell.start-1)) + ruleHeavyStart)
	heavy := Accent.Render(strings.Repeat(ruleHeavy, cell.end-cell.start))
	after := Muted.Render(ruleHeavyFinish + strings.Repeat(ruleLight, MaxInt(0, width-cell.end-1)))
	if cell.start == 0 {
		before = ""
	}
	return before + heavy + after
}

func Hints(bindings []key.Binding) string {
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	parts := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		if !binding.Enabled() {
			continue
		}
		hint := binding.Help()
		part := keyStyle.Render(hint.Key)
		if hint.Desc != "" {
			part += " " + Muted.Render(hint.Desc)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, Muted.Render(hintSeparator))
}

func Footer(width int, left, status string) string {
	if width <= 0 {
		return ""
	}
	return SpreadLine(width, left, Truncate(status, width))
}

func FooterRoom(width int, status string) int {
	if status == "" {
		return width
	}
	return width - lipgloss.Width(status) - headerMinGap
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
	width, height = MaxInt(1, width), MaxInt(1, height)
	lines := WrapPlain(fmt.Sprintf("Terminal too small (need %dx%d)", MinWidth, MinHeight), width)
	lines = append(lines, Muted.Render(Truncate(fmt.Sprintf("now %dx%d · q quit", width, height), width)))
	block := lipgloss.NewStyle().Align(lipgloss.Center).Render(strings.Join(lines, "\n"))
	return FitHeight(lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, block), height), true
}
