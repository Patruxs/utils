package common

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type PanelVariant int

const (
	PanelNormal PanelVariant = iota
	PanelFocused
	PanelDanger
)

const panelChrome = 4

type Panel struct {
	Title   string
	Meta    string
	Variant PanelVariant
	Width   int
	Height  int
}

func (p Panel) InnerWidth() int {
	return MaxInt(0, p.Width-panelChrome)
}

func (p Panel) Render(body string) string {
	if p.Width < panelChrome {
		return ""
	}
	border, borderStyle := p.border()
	innerWidth := p.InnerWidth()

	rows := strings.Split(body, "\n")
	if p.Height > 0 {
		rows = strings.Split(FitHeight(body, MaxInt(0, p.Height-2)), "\n")
		if p.Height <= 2 {
			rows = nil
		}
	}

	lines := make([]string, 0, len(rows)+2)
	lines = append(lines, p.topBorder(border, borderStyle))
	left := borderStyle.Render(border.Left) + " "
	right := " " + borderStyle.Render(border.Right)
	for _, row := range rows {
		lines = append(lines, left+PadRight(Truncate(row, innerWidth), innerWidth)+right)
	}
	lines = append(lines, borderStyle.Render(border.BottomLeft+strings.Repeat(border.Bottom, p.Width-2)+border.BottomRight))
	return strings.Join(lines, "\n")
}

func (p Panel) border() (lipgloss.Border, lipgloss.Style) {
	switch p.Variant {
	case PanelDanger:
		return lipgloss.ThickBorder(), lipgloss.NewStyle().Foreground(ColorDanger)
	case PanelFocused:
		return lipgloss.RoundedBorder(), lipgloss.NewStyle().Foreground(ColorAccent)
	default:
		return lipgloss.RoundedBorder(), lipgloss.NewStyle().Foreground(ColorMuted)
	}
}

func (p Panel) topBorder(border lipgloss.Border, borderStyle lipgloss.Style) string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	if p.Variant == PanelDanger {
		titleStyle = titleStyle.Foreground(ColorDanger)
	}

	leftCap := border.TopLeft + border.Top
	rightCap := border.Top + border.TopRight
	room := p.Width - lipgloss.Width(leftCap) - lipgloss.Width(rightCap)

	meta := ""
	if p.Meta != "" && lipgloss.Width(p.Meta)+2 <= room-4 {
		meta = p.Meta
	}
	metaWidth := 0
	if meta != "" {
		metaWidth = lipgloss.Width(meta) + 2
	}

	title := ""
	if p.Title != "" && room-metaWidth >= 3 {
		title = Truncate(p.Title, room-metaWidth-2)
	}
	titleWidth := 0
	if title != "" {
		titleWidth = lipgloss.Width(title) + 2
	}

	fill := room - titleWidth - metaWidth
	var b strings.Builder
	b.WriteString(borderStyle.Render(leftCap))
	if title != "" {
		b.WriteString(" " + titleStyle.Render(title) + " ")
	}
	b.WriteString(borderStyle.Render(strings.Repeat(border.Top, fill)))
	if meta != "" {
		b.WriteString(" " + Muted.Render(meta) + " ")
	}
	b.WriteString(borderStyle.Render(rightCap))
	return b.String()
}
