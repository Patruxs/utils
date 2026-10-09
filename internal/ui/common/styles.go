package common

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/lipgloss"
)

var (
	App      = lipgloss.NewStyle().Padding(AppPaddingY, AppPaddingX)
	Title    = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	Section  = lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess)
	Accent   = lipgloss.NewStyle().Foreground(ColorAccent)
	Selected = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	Muted    = lipgloss.NewStyle().Foreground(ColorSubtle)
	Help     = lipgloss.NewStyle().Foreground(ColorSubtle)
	Warning  = lipgloss.NewStyle().Foreground(ColorWarning)
	Error    = lipgloss.NewStyle().Foreground(ColorDanger)
	Success  = lipgloss.NewStyle().Foreground(ColorSuccess)
)

func NewHelpModel() help.Model {
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	helpModel := help.New()
	helpModel.ShortSeparator = " · "
	helpModel.FullSeparator = "    "
	helpModel.Styles.ShortKey = keyStyle
	helpModel.Styles.ShortDesc = Muted
	helpModel.Styles.ShortSeparator = Muted
	helpModel.Styles.FullKey = keyStyle
	helpModel.Styles.FullDesc = Muted
	helpModel.Styles.FullSeparator = Muted
	helpModel.Styles.Ellipsis = Muted
	return helpModel
}
