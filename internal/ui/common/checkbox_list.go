package common

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type CheckboxItem struct {
	ID         string
	Label      string
	Details    []string
	FilterText string
	Checked    bool
}

func (i CheckboxItem) FilterValue() string {
	if i.FilterText != "" {
		return i.FilterText
	}
	return i.Label
}

type CheckboxListModel struct {
	list    list.Model
	focused bool
	width   int
	height  int
}

func NewCheckboxList(items []CheckboxItem, width, height int) CheckboxListModel {
	listItems := make([]list.Item, 0, len(items))
	for _, item := range items {
		listItems = append(listItems, item)
	}

	model := list.New(listItems, checkboxDelegate{}, width, height)
	model.SetShowTitle(false)
	model.SetShowStatusBar(false)
	model.SetShowPagination(false)
	model.SetShowHelp(false)
	model.SetShowFilter(false)
	model.DisableQuitKeybindings()
	model.InfiniteScrolling = true
	model.KeyMap.CursorUp = DefaultKeys.Up
	model.KeyMap.CursorDown = DefaultKeys.Down
	model.KeyMap.ClearFilter = DefaultKeys.Back
	model.KeyMap.CancelWhileFiltering = DefaultKeys.Back
	model.KeyMap.AcceptWhileFiltering = DefaultKeys.Enter

	return CheckboxListModel{list: model, width: width, height: height}
}

func (m CheckboxListModel) Update(msg tea.Msg) (CheckboxListModel, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok && !m.list.SettingFilter() {
		if key.Matches(msg, DefaultKeys.Enter, DefaultKeys.Space) {
			return m.ToggleSelected()
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m CheckboxListModel) View() string {
	width := m.width
	if width <= 0 {
		width = DefaultContentWidth
	}

	lines := make([]string, 0, len(m.list.Items()))
	for index, item := range m.list.Items() {
		checkboxItem, ok := item.(CheckboxItem)
		if !ok {
			continue
		}

		selected := m.focused && index == m.list.Index()
		lines = append(lines, renderCheckboxItem(checkboxItem, width, selected)...)
		if shouldRenderCheckboxDetails(checkboxItem, selected) {
			lines = append(lines, renderCheckboxDetails(checkboxItem.Details, width)...)
		}
	}

	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}

func (m *CheckboxListModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width, height)
}

func (m *CheckboxListModel) SetFocused(focused bool) {
	m.focused = focused
	m.list.SetDelegate(checkboxDelegate{focused: focused})
}

func (m *CheckboxListModel) SelectByID(id string) {
	for index, item := range m.list.Items() {
		if checkboxItem, ok := item.(CheckboxItem); ok && checkboxItem.ID == id {
			m.list.Select(index)
			return
		}
	}
}

func (m CheckboxListModel) ToggleSelected() (CheckboxListModel, tea.Cmd) {
	item, ok := m.list.SelectedItem().(CheckboxItem)
	if !ok {
		return m, nil
	}

	item.Checked = !item.Checked
	return m.SetChecked(item.ID, item.Checked)
}

func (m CheckboxListModel) SetChecked(id string, checked bool) (CheckboxListModel, tea.Cmd) {
	for index, item := range m.list.Items() {
		checkboxItem, ok := item.(CheckboxItem)
		if !ok || checkboxItem.ID != id {
			continue
		}

		checkboxItem.Checked = checked
		return m, m.list.SetItem(index, checkboxItem)
	}

	return m, nil
}

func (m CheckboxListModel) Checked(id string) bool {
	for _, item := range m.list.Items() {
		checkboxItem, ok := item.(CheckboxItem)
		if ok && checkboxItem.ID == id {
			return checkboxItem.Checked
		}
	}

	return false
}

func (m CheckboxListModel) AnyChecked() bool {
	for _, item := range m.list.Items() {
		checkboxItem, ok := item.(CheckboxItem)
		if ok && checkboxItem.Checked {
			return true
		}
	}

	return false
}

func (m CheckboxListModel) AtStart() bool {
	return m.list.Index() == 0
}

func (m CheckboxListModel) AtEnd() bool {
	return m.list.Index() >= len(m.list.VisibleItems())-1
}

func (m CheckboxListModel) GoToStart() CheckboxListModel {
	m.list.GoToStart()
	return m
}

func (m CheckboxListModel) GoToEnd() CheckboxListModel {
	m.list.GoToEnd()
	return m
}

func (m CheckboxListModel) MoveUp() CheckboxListModel {
	m.list.CursorUp()
	return m
}

func (m CheckboxListModel) MoveDown() CheckboxListModel {
	m.list.CursorDown()
	return m
}

func (m CheckboxListModel) Len() int {
	return len(m.list.Items())
}

type checkboxDelegate struct {
	focused bool
}

func (d checkboxDelegate) Height() int {
	return 1
}

func (d checkboxDelegate) Spacing() int {
	return 0
}

func (d checkboxDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (d checkboxDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	checkboxItem, ok := item.(CheckboxItem)
	if !ok {
		return
	}

	cursor := " "
	if d.focused && index == m.Index() {
		cursor = Selected.Render(">")
	}

	marker := "[ ]"
	if checkboxItem.Checked {
		marker = "[x]"
	}

	text := fmt.Sprintf("%s %s", marker, checkboxItem.Label)
	if checkboxItem.Checked {
		text = Success.Render(text)
	} else {
		text = Muted.Render(text)
	}

	fmt.Fprintf(w, "  %s %s", cursor, text)
}

func renderCheckboxItem(item CheckboxItem, width int, selected bool) []string {
	cursor := " "
	if selected {
		cursor = ">"
	}

	marker := "[ ]"
	if item.Checked {
		marker = "[x]"
	}

	lines := WrapLine(fmt.Sprintf("  %s ", cursor), fmt.Sprintf("%s %s", marker, item.Label), width)
	for index, line := range lines {
		switch {
		case item.Checked:
			lines[index] = Success.Render(line)
		case selected:
			lines[index] = Selected.Render(line)
		default:
			lines[index] = Muted.Render(line)
		}
	}

	return lines
}

func shouldRenderCheckboxDetails(item CheckboxItem, selected bool) bool {
	return len(item.Details) > 0 && (selected || item.Checked)
}

func renderCheckboxDetails(details []string, width int) []string {
	lines := make([]string, 0, len(details))
	for _, detail := range details {
		for _, line := range WrapLine("      - ", detail, width) {
			lines = append(lines, Muted.Render(line))
		}
	}

	return lines
}
