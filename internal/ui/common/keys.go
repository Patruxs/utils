package common

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type GlobalKeyMap struct {
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	Space key.Binding
	Quit  key.Binding
	Yes   key.Binding
	No    key.Binding
	Back  key.Binding

	CancelRun     key.Binding
	CancelAndQuit key.Binding
	ScrollLog     key.Binding
	BackToList    key.Binding
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
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Space: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "select"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Yes: key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "confirm"),
	),
	No: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "cancel"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc", "backspace"),
		key.WithHelp("esc", "back"),
	),
	CancelRun: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel run"),
	),
	CancelAndQuit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "cancel and quit"),
	),
	ScrollLog: key.NewBinding(
		key.WithKeys("up", "down", "pgup", "pgdown"),
		key.WithHelp("up/down/pgup/pgdn", "scroll log"),
	),
	BackToList: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "back to list"),
	),
}

func IsForceQuit(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeyCtrlC
}
