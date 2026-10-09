package common

import "github.com/charmbracelet/lipgloss"

var (
	ColorAccent  = adaptiveColor("#7AA2F7", "111", "12", "#2F5BD3", "26", "4")
	ColorSubtle  = adaptiveColor("#8B93A7", "245", "8", "#6A7080", "242", "8")
	ColorBorder  = adaptiveColor("#3B4261", "238", "8", "#C3C8D4", "251", "7")
	ColorSuccess = adaptiveColor("#9ECE6A", "150", "10", "#2E7D32", "28", "2")
	ColorWarning = adaptiveColor("#E0AF68", "179", "11", "#9A6700", "136", "3")
	ColorDanger  = adaptiveColor("#F7768E", "204", "9", "#C62828", "160", "1")
)

func adaptiveColor(darkTrue, dark256, darkANSI, lightTrue, light256, lightANSI string) lipgloss.CompleteAdaptiveColor {
	return lipgloss.CompleteAdaptiveColor{
		Dark:  lipgloss.CompleteColor{TrueColor: darkTrue, ANSI256: dark256, ANSI: darkANSI},
		Light: lipgloss.CompleteColor{TrueColor: lightTrue, ANSI256: light256, ANSI: lightANSI},
	}
}

type Tone int

const (
	ToneNormal Tone = iota
	ToneSubtle
	ToneAccent
	ToneSuccess
	ToneWarning
	ToneDanger
)

func (t Tone) Style() lipgloss.Style {
	style := lipgloss.NewStyle()
	if color, ok := t.color(); ok {
		style = style.Foreground(color)
	}
	return style
}

func (t Tone) Glyph() string {
	switch t {
	case ToneAccent:
		return "▸"
	case ToneSuccess:
		return "✓"
	case ToneWarning:
		return "!"
	case ToneDanger:
		return "✗"
	default:
		return ""
	}
}

func (t Tone) color() (lipgloss.TerminalColor, bool) {
	switch t {
	case ToneSubtle:
		return ColorSubtle, true
	case ToneAccent:
		return ColorAccent, true
	case ToneSuccess:
		return ColorSuccess, true
	case ToneWarning:
		return ColorWarning, true
	case ToneDanger:
		return ColorDanger, true
	default:
		return nil, false
	}
}
