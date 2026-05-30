package common

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/lipgloss"
)

var (
	App      = lipgloss.NewStyle().Padding(AppPaddingY, AppPaddingX)
	Title    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorPrimary))
	Section  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorSuccess))
	Accent   = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary))
	Selected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorPrimary))
	Muted    = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted))
	Help     = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorHelp))
	Warning  = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning))
	Error    = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	Success  = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
)

func NewHelpModel() help.Model {
	helpModel := help.New()
	helpModel.ShortSeparator = "  "
	helpModel.FullSeparator = "    "
	helpModel.Styles.ShortKey = Accent
	helpModel.Styles.ShortDesc = Muted
	helpModel.Styles.ShortSeparator = Muted
	helpModel.Styles.FullKey = Accent
	helpModel.Styles.FullDesc = Muted
	helpModel.Styles.FullSeparator = Muted
	helpModel.Styles.Ellipsis = Muted
	return helpModel
}
