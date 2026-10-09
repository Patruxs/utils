package common

import "github.com/charmbracelet/lipgloss"

var (
	Accent   = lipgloss.NewStyle().Foreground(ColorAccent)
	Selected = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	Muted    = lipgloss.NewStyle().Foreground(ColorMuted)
	Warning  = lipgloss.NewStyle().Foreground(ColorWarning)
	Error    = lipgloss.NewStyle().Foreground(ColorDanger)
	Success  = lipgloss.NewStyle().Foreground(ColorSuccess)
)
