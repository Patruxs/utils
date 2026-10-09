package common

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	logLineIndent      = "  "
	logDetailGap       = "  "
	logTextColumnShare = 45
)

type LogLine struct {
	Tone   Tone
	Text   string
	Detail string
}

type LogSection struct {
	ID    string
	Title string
	Glyph string
	Tone  Tone
	Lines []LogLine
}

type LogViewer struct {
	viewport viewport.Model
	sections []LogSection
}

func NewLogViewer() LogViewer {
	logViewport := viewport.New(DefaultContentWidth, 1)
	logViewport.KeyMap = viewport.KeyMap{
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓", "scroll down")),
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑", "scroll up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "d", "ctrl+d"), key.WithHelp("pgdn", "page down")),
		PageUp:   key.NewBinding(key.WithKeys("pgup", "u", "ctrl+u"), key.WithHelp("pgup", "page up")),
	}
	return LogViewer{viewport: logViewport}
}

func (m *LogViewer) SetSections(sections []LogSection) {
	m.sections = append([]LogSection(nil), sections...)
	m.refresh()
	m.viewport.GotoTop()
}

func (m *LogViewer) SetSize(width, height int) {
	if width <= 0 {
		width = DefaultContentWidth
	}
	height = MaxInt(1, height)
	widthChanged := width != m.viewport.Width
	m.viewport.Width = width
	m.viewport.Height = height
	if widthChanged {
		m.refresh()
	}
	m.viewport.SetYOffset(m.viewport.YOffset)
}

func (m LogViewer) Update(msg tea.Msg) (LogViewer, tea.Cmd) {
	if !m.HasContent() {
		return m, nil
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m LogViewer) IsKeyScrollInput(msg tea.KeyMsg) bool {
	if !m.HasContent() {
		return false
	}
	keys := m.viewport.KeyMap
	return key.Matches(msg, keys.Up, keys.Down, keys.PageUp, keys.PageDown)
}

func (m LogViewer) View() string {
	if !m.HasContent() {
		return ""
	}
	return m.viewport.View()
}

func (m LogViewer) Position() string {
	total := m.viewport.TotalLineCount()
	if !m.HasContent() || total == 0 {
		return ""
	}
	first := m.viewport.YOffset + 1
	last := MinInt(total, m.viewport.YOffset+m.viewport.Height)
	return fmt.Sprintf("%d–%d of %d", first, last, total)
}

func (m LogViewer) HasContent() bool {
	for _, section := range m.sections {
		if len(section.Lines) > 0 {
			return true
		}
	}
	return false
}

func (m *LogViewer) refresh() {
	width := m.viewport.Width
	var rows []string
	for _, section := range m.sections {
		if len(section.Lines) == 0 {
			continue
		}
		textColumn := logTextColumn(section, width)
		if section.Title != "" {
			rows = append(rows, renderLogHeader(section, width))
		}
		for _, line := range section.Lines {
			rows = append(rows, renderLogLine(line, width, textColumn)...)
		}
	}
	m.viewport.SetContent(strings.Join(rows, "\n"))
}

func renderLogHeader(section LogSection, width int) string {
	glyph := section.Glyph
	if glyph == "" {
		glyph = section.Tone.Glyph()
	}
	header := ""
	if glyph != "" {
		header = section.Tone.Style().Render(glyph) + " "
	}
	header += section.Tone.Style().Bold(true).Render(section.Title)
	header += "  " + fmt.Sprint(len(section.Lines))
	return Truncate(header, width)
}

func logLinePrefix(line LogLine) string {
	if glyph := line.Tone.Glyph(); glyph != "" {
		return logLineIndent + glyph + " "
	}
	return logLineIndent
}

func logTextColumn(section LogSection, width int) int {
	column := 0
	for _, line := range section.Lines {
		if line.Detail != "" {
			column = MaxInt(column, lipgloss.Width(logLinePrefix(line)+line.Text))
		}
	}
	return MinInt(column, width*logTextColumnShare/100)
}

func renderLogLine(line LogLine, width, textColumn int) []string {
	prefix := logLinePrefix(line)
	style := line.Tone.Style()
	if line.Detail != "" {
		head := PadRight(prefix+line.Text, textColumn) + logDetailGap
		if lipgloss.Width(head)+lipgloss.Width(line.Detail) <= width {
			return []string{style.Render(PadRight(prefix+line.Text, textColumn)) + logDetailGap + Muted.Render(line.Detail)}
		}
	}

	var rows []string
	for _, row := range WrapLine(prefix, line.Text, width) {
		rows = append(rows, style.Render(Truncate(row, width)))
	}
	if line.Detail != "" {
		for _, row := range WrapLine(strings.Repeat(" ", lipgloss.Width(prefix)+2), line.Detail, width) {
			rows = append(rows, Muted.Render(Truncate(row, width)))
		}
	}
	return rows
}
