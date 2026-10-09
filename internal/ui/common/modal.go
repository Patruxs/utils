package common

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	modalMinWidth   = 44
	modalMaxWidth   = 80
	modalPadding    = "  "
	modalButtonGap  = "      "
	modalChromeRows = 6
)

type ConfirmResult int

const (
	ConfirmPending ConfirmResult = iota
	ConfirmCanceled
	ConfirmAccepted
)

type confirmFocus int

const (
	focusCancel confirmFocus = iota
	focusAction
)

type Confirm struct {
	Title  string
	Lines  []string
	Action string
	open   bool
	focus  confirmFocus
}

var confirmKeys = struct {
	Move    key.Binding
	Select  key.Binding
	Accept  key.Binding
	Dismiss key.Binding
}{
	Move:    key.NewBinding(key.WithKeys("left", "right", "tab", "shift+tab", "h", "l"), key.WithHelp("←→", "choose")),
	Select:  key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("enter", "select")),
	Accept:  key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "confirm")),
	Dismiss: key.NewBinding(key.WithKeys("esc", "n"), key.WithHelp("esc", "cancel")),
}

func NewConfirm(title, action string, lines ...string) Confirm {
	return Confirm{Title: title, Lines: lines, Action: action, open: true, focus: focusCancel}
}

func (c Confirm) Open() bool {
	return c.open
}

func (c Confirm) Keys() KeyList {
	return KeyList{confirmKeys.Move, confirmKeys.Select, confirmKeys.Accept, confirmKeys.Dismiss}
}

func (c Confirm) Update(msg tea.Msg) (Confirm, ConfirmResult) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !c.open || !ok {
		return c, ConfirmPending
	}
	switch {
	case key.Matches(keyMsg, confirmKeys.Move):
		c.focus = 1 - c.focus
	case key.Matches(keyMsg, confirmKeys.Dismiss):
		return c.close(ConfirmCanceled)
	case key.Matches(keyMsg, confirmKeys.Accept):
		return c.close(ConfirmAccepted)
	case key.Matches(keyMsg, confirmKeys.Select):
		if c.focus == focusAction {
			return c.close(ConfirmAccepted)
		}
		return c.close(ConfirmCanceled)
	}
	return c, ConfirmPending
}

func (c Confirm) close(result ConfirmResult) (Confirm, ConfirmResult) {
	c.open = false
	c.focus = focusCancel
	return c, result
}

func (c Confirm) View(maxWidth, maxHeight int) string {
	if !c.open {
		return ""
	}
	title := ToneWarning.Glyph() + " " + c.Title
	natural := lipgloss.Width(title) + 6
	for _, line := range c.Lines {
		natural = MaxInt(natural, lipgloss.Width(line)+len(modalPadding)+panelChrome)
	}
	width := MinInt(MaxInt(natural, modalMinWidth), MinInt(modalMaxWidth, maxWidth))
	panel := Panel{Title: title, Variant: PanelDanger, Width: width}
	textWidth := MaxInt(1, panel.InnerWidth()-len(modalPadding))

	var body []string
	for _, line := range c.Lines {
		for _, wrapped := range WrapStyled(line, textWidth) {
			body = append(body, modalPadding+wrapped)
		}
	}
	if room := maxHeight - modalChromeRows; room >= 1 && len(body) > room {
		hidden := len(body) - room + 1
		body = append(body[:room-1], modalPadding+Muted.Render(fmt.Sprintf("… %d more", hidden)))
	}

	buttons := c.buttons()
	buttonIndent := MaxInt(0, (panel.InnerWidth()-lipgloss.Width(buttons))/2)
	rows := append([]string{""}, body...)
	rows = append(rows, "", strings.Repeat(" ", buttonIndent)+buttons)
	return panel.Render(strings.Join(rows, "\n"))
}

func (c Confirm) buttons() string {
	cancel, action := "  Cancel  ", "  "+c.Action+"  "
	if c.focus == focusCancel {
		cancel = lipgloss.NewStyle().Bold(true).Render("[ Cancel ]")
	} else {
		action = ToneDanger.Style().Bold(true).Render("[ " + c.Action + " ]")
	}
	return cancel + modalButtonGap + action
}

func Overlay(base, top string, width, height int) string {
	baseLines := strings.Split(FitHeight(base, height), "\n")
	for i, line := range baseLines {
		baseLines[i] = Dim(line, width)
	}
	if top == "" {
		return strings.Join(baseLines, "\n")
	}
	topLines := strings.Split(top, "\n")
	if len(topLines) > height {
		topLines = topLines[:height]
	}
	topWidth := 0
	for _, line := range topLines {
		topWidth = MaxInt(topWidth, lipgloss.Width(line))
	}
	topWidth = MinInt(topWidth, width)
	x := (width - topWidth) / 2
	y := (height - len(topLines)) / 2
	for i, line := range topLines {
		row := y + i
		plain := ansi.Strip(baseLines[row])
		left := Muted.Faint(true).Render(PadRight(ansi.Cut(plain, 0, x), x))
		right := Muted.Faint(true).Render(ansi.Cut(plain, x+topWidth, width))
		baseLines[row] = left + PadRight(Truncate(line, topWidth), topWidth) + right
	}
	return strings.Join(baseLines, "\n")
}

func Dim(line string, width int) string {
	return Muted.Faint(true).Render(Truncate(ansi.Strip(line), width))
}
