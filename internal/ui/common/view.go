package common

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type RunningView interface {
	Running() bool
}

type KeyOwner interface {
	OwnsKeys() bool
}

type FooterView interface {
	FooterKeys() help.KeyMap
	FooterStatus() string
}

type ModalView interface {
	Modal() string
}

type ActivatedMsg struct{}

type NoticeMsg struct {
	Tone Tone
	Text string
}

func Notify(tone Tone, text string) tea.Cmd {
	return func() tea.Msg {
		return NoticeMsg{Tone: tone, Text: text}
	}
}

type KeyList []key.Binding

func (k KeyList) ShortHelp() []key.Binding {
	return k
}

func (k KeyList) FullHelp() [][]key.Binding {
	return [][]key.Binding{k}
}
