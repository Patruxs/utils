package common

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type GlobalKeyMap struct {
	Up            key.Binding
	Down          key.Binding
	CancelRun     key.Binding
	CancelAndQuit key.Binding
}

var DefaultKeys = GlobalKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("up/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("down/j", "down"),
	),
	CancelRun: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel run"),
	),
	CancelAndQuit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "cancel and quit"),
	),
}

func IsForceQuit(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeyCtrlC
}
