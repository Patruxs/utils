package common

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type CheckboxItem struct {
	ID         string
	Label      string
	Details    []string
	FilterText string
	Checked    bool
	Tag        string
	TagTone    Tone
	Group      string
}

func (i CheckboxItem) FilterValue() string {
	if i.FilterText != "" {
		return i.FilterText
	}
	return i.Label
}

const checkboxLabelMinWidth = 12

type CheckboxListModel struct {
	list    list.Model
	focused bool
	width   int
	height  int
	offset  int
}

type checkboxRow struct {
	group string
	item  CheckboxItem
	index int
}

func (r checkboxRow) isItem() bool {
	return r.index >= 0
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
	m.keepCursorVisible()
	return m, cmd
}

func (m CheckboxListModel) View() string {
	width := m.width
	if width <= 0 {
		width = DefaultContentWidth
	}

	rows := m.rows()
	start, end, showAbove, showBelow := m.window(rows)
	lines := make([]string, 0, MaxInt(0, m.height))
	if showAbove {
		lines = append(lines, Muted.Render(Truncate(fmt.Sprintf("↑ %d more", countCheckboxItems(rows[:start])), width)))
	}
	for _, row := range rows[start:end] {
		lines = append(lines, m.renderRow(row, width))
	}
	if showBelow {
		lines = append(lines, Muted.Render(Truncate(fmt.Sprintf("↓ %d more", countCheckboxItems(rows[end:])), width)))
	}
	return strings.Join(lines, "\n")
}

func (m CheckboxListModel) Selected() CheckboxItem {
	item, _ := m.list.SelectedItem().(CheckboxItem)
	return item
}

func (m CheckboxListModel) rows() []checkboxRow {
	visible := m.list.VisibleItems()
	rows := make([]checkboxRow, 0, len(visible))
	group := ""
	for index, listItem := range visible {
		item, ok := listItem.(CheckboxItem)
		if !ok {
			continue
		}
		if item.Group != "" && item.Group != group {
			rows = append(rows, checkboxRow{group: item.Group, index: -1})
		}
		group = item.Group
		rows = append(rows, checkboxRow{item: item, index: index})
	}
	return rows
}

func (m CheckboxListModel) window(rows []checkboxRow) (start, end int, showAbove, showBelow bool) {
	total := len(rows)
	if m.height <= 0 || total <= m.height {
		return 0, total, false, false
	}
	offset := MaxInt(0, MinInt(m.offset, total-m.height+1))
	if m.height < 3 {
		return offset, MinInt(total, offset+m.height), false, false
	}

	showAbove = offset > 0
	room := m.height
	if showAbove {
		room--
	}
	if offset+room >= total {
		return offset, total, showAbove, false
	}
	end = offset + room - 1
	if end-offset > 1 && !rows[end-1].isItem() {
		end--
	}
	return offset, end, showAbove, true
}

func (m *CheckboxListModel) keepCursorVisible() {
	rows := m.rows()
	cursor := -1
	for i, row := range rows {
		if row.index == m.list.Index() {
			cursor = i
			break
		}
	}
	if cursor < 0 {
		m.offset = 0
		return
	}

	first := cursor
	if cursor > 0 && !rows[cursor-1].isItem() {
		first = cursor - 1
	}
	if start, _, _, _ := m.window(rows); first < start {
		m.offset = first
	}
	for {
		start, end, _, _ := m.window(rows)
		m.offset = start
		if cursor < end || end >= len(rows) {
			return
		}
		m.offset++
	}
}

func countCheckboxItems(rows []checkboxRow) int {
	count := 0
	for _, row := range rows {
		if row.isItem() {
			count++
		}
	}
	return count
}

func (m CheckboxListModel) renderRow(row checkboxRow, width int) string {
	if !row.isItem() {
		return Muted.Bold(true).Render(Truncate(strings.ToUpper(row.group), width))
	}

	item := row.item
	selected := m.focused && row.index == m.list.Index()
	cursor := "  "
	label := item.Label
	if selected {
		cursor = Accent.Render(ToneAccent.Glyph()) + " "
		label = lipgloss.NewStyle().Bold(true).Render(label)
	}
	marker := "[ ]"
	if item.Checked {
		marker = Success.Render("[x]")
	}

	prefix := cursor + marker + " "
	tag := ""
	labelRoom := width - lipgloss.Width(prefix) - lipgloss.Width(item.Tag) - 1
	if item.Tag != "" && (labelRoom >= lipgloss.Width(item.Label) || labelRoom >= checkboxLabelMinWidth) {
		tag = item.TagTone.Style().Render(item.Tag)
	}
	return SpreadLine(width, prefix+label, tag)
}

func (m *CheckboxListModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width, height)
	m.keepCursorVisible()
}

func (m *CheckboxListModel) SetFocused(focused bool) {
	m.focused = focused
}

func (m *CheckboxListModel) SelectByID(id string) {
	for index, item := range m.list.Items() {
		if checkboxItem, ok := item.(CheckboxItem); ok && checkboxItem.ID == id {
			m.list.Select(index)
			m.keepCursorVisible()
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

func (m CheckboxListModel) AtStart() bool {
	return m.list.Index() == 0
}

func (m CheckboxListModel) AtEnd() bool {
	return m.list.Index() >= len(m.list.VisibleItems())-1
}

func (m CheckboxListModel) GoToStart() CheckboxListModel {
	m.list.GoToStart()
	m.keepCursorVisible()
	return m
}

func (m CheckboxListModel) GoToEnd() CheckboxListModel {
	m.list.GoToEnd()
	m.keepCursorVisible()
	return m
}

func (m CheckboxListModel) MoveUp() CheckboxListModel {
	m.list.CursorUp()
	m.keepCursorVisible()
	return m
}

func (m CheckboxListModel) MoveDown() CheckboxListModel {
	m.list.CursorDown()
	m.keepCursorVisible()
	return m
}

func (m CheckboxListModel) Len() int {
	return len(m.list.Items())
}

type checkboxDelegate struct{}

func (d checkboxDelegate) Height() int {
	return 1
}

func (d checkboxDelegate) Spacing() int {
	return 0
}

func (d checkboxDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (d checkboxDelegate) Render(io.Writer, list.Model, int, list.Item) {}
