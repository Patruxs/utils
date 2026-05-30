package views

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"utils/internal/core/cleaner"
	"utils/internal/ui/common"
)

type executeConfirmationSelection int

const (
	confirmationCancel executeConfirmationSelection = iota
	confirmationRun
	confirmationCount
)

type cleanupModeSelection int

const (
	cleanupModeDryRun cleanupModeSelection = iota
	cleanupModeExecute
	cleanupModeCancel
	cleanupModeCount
)

type ViewState int

const (
	StateSelectingOptions ViewState = iota
	StatePromptingMode
	StateConfirmingExecute
	StateRunning
	StateFinished
)

type cleanerKeyMap struct {
	Move        key.Binding
	Select      key.Binding
	Toggle      key.Binding
	Continue    key.Binding
	Run         key.Binding
	Execute     key.Binding
	ConfirmPrev key.Binding
	ConfirmNext key.Binding
	CancelRun   key.Binding
	BackToMenu  key.Binding
}

type cleanerContextualKeyMap struct {
	cleanerKeyMap
	state ViewState
}

type CleanerModel struct {
	spinner       spinner.Model
	help          help.Model
	keyMap        cleanerKeyMap
	optionsList   common.CheckboxListModel
	logViewer     LogViewerModel
	options       cleaner.Options
	report        *cleaner.Report
	err           error
	cancelCleanup context.CancelFunc
	notice        string
	layout        common.Layout
	state         ViewState
	confirmation  executeConfirmationSelection
	modeSelection cleanupModeSelection
}

type LogViewerModel struct {
	viewport     viewport.Model
	report       *cleaner.Report
	mouseFocused bool
}

type cleanerFinishedMsg struct {
	report cleaner.Report
	err    error
}

var _ tea.Model = CleanerModel{}

func newCleanerKeyMap() cleanerKeyMap {
	return cleanerKeyMap{
		Move: key.NewBinding(
			key.WithKeys("up", "down", "k", "j"),
			key.WithHelp("up/down:", "choose"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter/space:", "select"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space:", "toggle"),
		),
		Continue: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter:", "cleanup"),
		),
		Run: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "dry-run"),
		),
		Execute: key.NewBinding(
			key.WithKeys("e", "x"),
			key.WithHelp("e/x", "execute"),
		),
		ConfirmPrev: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("left/h", "previous"),
		),
		ConfirmNext: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("right/l", "next"),
		),
		CancelRun: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
			key.WithHelp("esc/ctrl+c", "cancel"),
		),
		BackToMenu: key.NewBinding(
			key.WithKeys("q", "esc"),
			key.WithHelp("q/esc", "main menu"),
		),
	}
}

func (k cleanerKeyMap) contextual(state ViewState) cleanerContextualKeyMap {
	return cleanerContextualKeyMap{
		cleanerKeyMap: k,
		state:         state,
	}
}

func (k cleanerContextualKeyMap) ShortHelp() []key.Binding {
	switch k.state {
	case StateRunning:
		return []key.Binding{k.CancelRun}
	case StateConfirmingExecute:
		return []key.Binding{k.Move, k.Select, common.DefaultKeys.Yes, common.DefaultKeys.No}
	case StatePromptingMode:
		return []key.Binding{k.Move, k.Select, k.Run, k.Execute, common.DefaultKeys.No}
	default:
		return []key.Binding{k.Move, k.Toggle, k.Continue, k.BackToMenu}
	}
}

func (k cleanerContextualKeyMap) FullHelp() [][]key.Binding {
	switch k.state {
	case StateRunning:
		return [][]key.Binding{
			{k.CancelRun},
		}
	case StateConfirmingExecute:
		return [][]key.Binding{
			{k.Move, k.Select},
			{common.DefaultKeys.Yes, common.DefaultKeys.No},
		}
	case StatePromptingMode:
		return [][]key.Binding{
			{k.Move, k.Select},
			{k.Run, k.Execute, common.DefaultKeys.No},
		}
	default:
		return [][]key.Binding{
			{k.Move, k.Toggle, k.Continue, k.BackToMenu},
		}
	}
}

func NewCleanerModel() CleanerModel {
	optionsList := newCleanerOptionsList()
	optionsList.SetFocused(false)

	return CleanerModel{
		spinner: spinner.New(
			spinner.WithSpinner(spinner.Dot),
			spinner.WithStyle(common.Accent),
		),
		help:        common.NewHelpModel(),
		keyMap:      newCleanerKeyMap(),
		optionsList: optionsList,
		logViewer:   NewLogViewerModel(),
		layout:      common.NewLayout(0, 0),
	}
}

func NewLogViewerModel() LogViewerModel {
	logViewport := viewport.New(0, defaultLogViewportHeight)
	logViewport.KeyMap = viewport.KeyMap{
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("down/j", "scroll down"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("up/k", "scroll up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "d", "ctrl+d"),
			key.WithHelp("pgdn/d", "scroll down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "u", "ctrl+u"),
			key.WithHelp("pgup/u", "scroll up"),
		),
	}

	return LogViewerModel{viewport: logViewport}
}

func newCleanerOptionsList() common.CheckboxListModel {
	optionsList := common.NewCheckboxList([]common.CheckboxItem{
		{
			ID:    optionSSHKeys,
			Label: "Include SSH keys",
			Details: []string{
				"Adds .ssh/config, known_hosts, and id_* private/public key files.",
				"Execute deletes local SSH keys; dry-run only lists those file deletions.",
			},
			FilterText: "ssh keys credentials private public",
		},
		{
			ID:    optionBrowserProfiles,
			Label: "Include browser profiles",
			Details: []string{
				"Adds full Chrome, Edge, Brave, CocCoc, Firefox, and Safari profile folders, not just caches.",
				"Execute removes local sign-ins, cookies/sessions, saved passwords, extensions, local storage, history, and bookmarks.",
			},
			FilterText: "browser profiles include caches",
		},
		{
			ID:    optionCredentialManager,
			Label: "Clean Windows Credential Manager allowlist" + credentialManagerHint(),
			Details: []string{
				"Windows only: scans Credential Manager for allowlisted dev entries such as Git, cloud CLIs, Docker, kube, npm, Terraform, Visual Studio, VS Code, Copilot, and AI tools.",
				"Dry-run lists matching entries; execute deletes only those allowlisted matches.",
			},
			FilterText: "windows credential manager allowlist credentials",
		},
		{
			ID:    optionForceStop,
			Label: "Force stop running target processes",
			Details: []string{
				"Stops running Chrome, Edge, Firefox, VS Code, Visual Studio, Claude, and Codex before cleanup so locked auth/profile files can be handled.",
				"This happens in dry-run too. Dry-run still only logs file and Credential Manager deletions.",
			},
			FilterText: "force stop kill running target processes browsers ides ai apps",
		},
	}, 0, cleanerOptionsMinHeight)
	optionsList.SelectByID(optionBrowserProfiles)
	return optionsList
}

func (m CleanerModel) Init() tea.Cmd {
	return nil
}

func (m CleanerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if m.state == StateRunning {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.layout = common.NewLayout(msg.Width, msg.Height)
		m.layoutComponents()
	case cleanerFinishedMsg:
		if m.cancelCleanup != nil {
			m.cancelCleanup()
			m.cancelCleanup = nil
		}
		m.state = StateFinished
		m.report = &msg.report
		m.err = msg.err
		m.notice = ""
		m.logViewer.SetMouseFocused(false)
		m.logViewer.SetReport(msg.report)
		m.layoutComponents()
		return m, nil
	case tea.KeyMsg:
		switch m.state {
		case StateRunning:
			if key.Matches(msg, m.keyMap.CancelRun) {
				return m.cancelRunningCleanup()
			}
			return m, batch(cmds...)
		case StateConfirmingExecute:
			return m.updateExecuteConfirmation(msg, cmds...)
		case StatePromptingMode:
			return m.updateModePrompt(msg, cmds...)
		case StateFinished:
			if m.logViewer.IsKeyScrollInput(msg) {
				var cmd tea.Cmd
				m.logViewer, cmd = m.logViewer.Update(msg)
				cmds = append(cmds, cmd)
				return m, batch(cmds...)
			}
			fallthrough
		case StateSelectingOptions:
			switch {
			case key.Matches(msg, common.DefaultKeys.Up):
				m.moveOptions(-1)
				return m, nil
			case key.Matches(msg, common.DefaultKeys.Down):
				m.moveOptions(1)
				return m, nil
			case key.Matches(msg, common.DefaultKeys.Space):
				return m.toggleFocusedOption()
			case key.Matches(msg, common.DefaultKeys.Enter):
				return m.openModePrompt()
			}
		}
	case tea.MouseMsg:
		if m.state == StateFinished && m.report != nil {
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				m.logViewer.SetMouseFocused(m.mouseInLogViewer(msg))
				return m, nil
			}
			if m.logViewer.IsFocusedMouseScrollInput(msg) {
				var cmd tea.Cmd
				m.logViewer, cmd = m.logViewer.Update(msg)
				cmds = append(cmds, cmd)
				return m, batch(cmds...)
			}
			return m, nil
		}
	}

	if m.report != nil {
		var cmd tea.Cmd
		m.logViewer, cmd = m.logViewer.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, batch(cmds...)
}

func (m CleanerModel) View() string {
	var b strings.Builder
	layout := m.layout

	b.WriteString(layout.RenderWrapped(cleanerTitle, common.Title.Render))
	b.WriteString("\n")
	b.WriteString(layout.RenderWrapped(cleanerSubtitle, common.Muted.Render))
	b.WriteString("\n\n")

	switch m.state {
	case StateRunning:
		mode := "dry-run"
		if m.options.Execute {
			mode = "execute"
		}
		b.WriteString(layout.RenderWrapped(fmt.Sprintf("%s Running %s cleanup...", m.spinner.View(), mode), func(strs ...string) string {
			return strings.Join(strs, "")
		}))
		b.WriteString("\n\n")
	default:
		m.optionsList.SetFocused(true)

		b.WriteString(layout.RenderWrapped(cleanerBaselineNote, common.Muted.Render))
		b.WriteString("\n\n")
		b.WriteString("Options\n")
		b.WriteString(m.optionsList.View())
		b.WriteString("\n")
		b.WriteString(layout.RenderWrapped(cleanerSafetyNotice, common.Muted.Render))
		b.WriteString("\n\n")
	}

	if m.notice != "" {
		b.WriteString(layout.RenderWrapped(m.notice, common.Warning.Render))
		b.WriteString("\n\n")
	}

	switch m.state {
	case StatePromptingMode:
		b.WriteString(layout.RenderWrapped("Choose cleanup mode for the selected options.", common.Warning.Render))
		b.WriteString("\n")
		b.WriteString(cleanerRow(m.modeSelection == cleanupModeDryRun, "", "Run dry-run cleanup"))
		b.WriteString(cleanerRow(m.modeSelection == cleanupModeExecute, "", "Run execute cleanup"))
		b.WriteString(cleanerRow(m.modeSelection == cleanupModeCancel, "", "Cancel option"))
		b.WriteString("\n")
	case StateConfirmingExecute:
		b.WriteString(layout.RenderWrapped("Execute mode will delete matching local files. Choose an option and press enter.", common.Error.Render))
		b.WriteString("\n")
		b.WriteString(confirmationRow(m.confirmation == confirmationCancel, "Cancel"))
		b.WriteString(confirmationRow(m.confirmation == confirmationRun, "Run execute cleanup"))
		b.WriteString("\n")
	}

	if m.report != nil {
		b.WriteString("\n")
		b.WriteString(m.logViewer.View())
	}

	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(layout.RenderWrapped("Completed with errors: "+m.err.Error(), common.Error.Render))
		b.WriteString("\n")
	}

	helpView := m.renderHelp()
	if helpView != "" {
		b.WriteString("\n")
		b.WriteString(helpView)
		b.WriteString("\n")
	}

	return b.String()
}

func (m *CleanerModel) layoutComponents() {
	m.help.Width = m.layout.Width
	m.optionsList.SetSize(m.layout.Width, m.layout.ListHeight(m.optionsList.Len(), cleanerOptionsReservedHeight, cleanerOptionsMinHeight))
	m.logViewer.SetSize(m.layout.Width, m.layout.ViewportHeight(cleanerLogReservedHeight, cleanerLogMinHeight, cleanerLogMaxHeight))
}

func (m CleanerModel) mouseInLogViewer(msg tea.MouseMsg) bool {
	if m.report == nil {
		return false
	}

	view := m.View()
	index := strings.Index(view, "Recent activity\n")
	if index < 0 {
		return false
	}

	top := strings.Count(view[:index], "\n")
	bottom := top + 1 + m.logViewer.viewport.Height
	return msg.Y >= top && msg.Y <= bottom
}

func (m *CleanerModel) moveOptions(delta int) {
	if m.state != StateFinished {
		m.state = StateSelectingOptions
	}

	if delta > 0 {
		m.optionsList = m.optionsList.MoveDown()
		return
	}

	m.optionsList = m.optionsList.MoveUp()
}

func (m CleanerModel) openModePrompt() (tea.Model, tea.Cmd) {
	m.syncOptionsFromList()

	if !m.hasSelectedOptions() {
		if m.state != StateFinished {
			m.state = StateSelectingOptions
		}
		m.notice = "Select at least one cleanup option before pressing enter."
		return m, nil
	}

	m.state = StatePromptingMode
	m.modeSelection = cleanupModeDryRun
	m.err = nil
	m.notice = ""

	return m, nil
}

func (m CleanerModel) updateExecuteConfirmation(msg tea.KeyMsg, cmds ...tea.Cmd) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, common.DefaultKeys.Yes):
		m.options.Execute = true
		return m.startRun()
	case key.Matches(msg, common.DefaultKeys.No):
		m.state = StateSelectingOptions
		return m, nil
	case key.Matches(msg, common.DefaultKeys.Up, m.keyMap.ConfirmPrev):
		m.confirmation = executeConfirmationSelection(wrapIndex(int(m.confirmation)-1, int(confirmationCount)))
	case key.Matches(msg, common.DefaultKeys.Down, m.keyMap.ConfirmNext):
		m.confirmation = executeConfirmationSelection(wrapIndex(int(m.confirmation)+1, int(confirmationCount)))
	case key.Matches(msg, common.DefaultKeys.Enter, common.DefaultKeys.Space):
		if m.confirmation == confirmationRun {
			m.options.Execute = true
			return m.startRun()
		}
		m.state = StateSelectingOptions
	case key.Matches(msg, m.keyMap.Run):
		m.options.Execute = false
		return m.startRun()
	default:
		return m, batch(cmds...)
	}

	return m, nil
}

func (m CleanerModel) updateModePrompt(msg tea.KeyMsg, cmds ...tea.Cmd) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keyMap.Run):
		m.options.Execute = false
		return m.startRun()
	case key.Matches(msg, m.keyMap.Execute):
		m.state = StateConfirmingExecute
		m.confirmation = confirmationCancel
		m.err = nil
	case key.Matches(msg, common.DefaultKeys.No):
		m.state = StateSelectingOptions
	case key.Matches(msg, common.DefaultKeys.Up, m.keyMap.ConfirmPrev):
		m.modeSelection = cleanupModeSelection(wrapIndex(int(m.modeSelection)-1, int(cleanupModeCount)))
	case key.Matches(msg, common.DefaultKeys.Down, m.keyMap.ConfirmNext):
		m.modeSelection = cleanupModeSelection(wrapIndex(int(m.modeSelection)+1, int(cleanupModeCount)))
	case key.Matches(msg, common.DefaultKeys.Enter, common.DefaultKeys.Space):
		switch m.modeSelection {
		case cleanupModeDryRun:
			m.options.Execute = false
			return m.startRun()
		case cleanupModeExecute:
			m.state = StateConfirmingExecute
			m.confirmation = confirmationCancel
			m.err = nil
		case cleanupModeCancel:
			m.state = StateSelectingOptions
		}
	default:
		return m, batch(cmds...)
	}

	return m, nil
}

func (m CleanerModel) toggleFocusedOption() (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.optionsList, cmd = m.optionsList.ToggleSelected()
	m.syncOptionsFromList()
	m.notice = ""

	return m, cmd
}

func (m *CleanerModel) syncOptionsFromList() {
	m.options.CleanSSHKeys = m.optionsList.Checked(optionSSHKeys)
	m.options.IncludeBrowserProfiles = m.optionsList.Checked(optionBrowserProfiles)
	m.options.CleanCredentialManager = m.optionsList.Checked(optionCredentialManager)
	m.options.ForceStopProcesses = m.optionsList.Checked(optionForceStop)
}

func (m CleanerModel) syncedOptions() cleaner.Options {
	options := m.options
	options.CleanSSHKeys = m.optionsList.Checked(optionSSHKeys)
	options.IncludeBrowserProfiles = m.optionsList.Checked(optionBrowserProfiles)
	options.CleanCredentialManager = m.optionsList.Checked(optionCredentialManager)
	options.ForceStopProcesses = m.optionsList.Checked(optionForceStop)
	return options
}

func (m CleanerModel) hasSelectedOptions() bool {
	return m.optionsList.AnyChecked()
}

func (m CleanerModel) renderHelp() string {
	return m.help.View(m.keyMap.contextual(m.state))
}

func (m CleanerModel) startRun() (tea.Model, tea.Cmd) {
	if !m.hasSelectedOptions() {
		m.state = StateSelectingOptions
		m.notice = "Select at least one cleanup option before running cleanup."
		return m, nil
	}

	m.state = StateRunning
	m.report = nil
	m.err = nil
	m.notice = ""
	m.options = m.syncedOptions()

	options := m.options
	ctx, cancel := context.WithTimeout(context.Background(), cleanerRunTimeout)
	m.cancelCleanup = cancel
	return m, batch(m.spinner.Tick, runCleaner(ctx, options))
}

func (m CleanerModel) cancelRunningCleanup() (tea.Model, tea.Cmd) {
	if m.cancelCleanup != nil {
		m.cancelCleanup()
		m.cancelCleanup = nil
	}
	m.notice = "Canceling cleanup..."
	return m, nil
}

func (m *LogViewerModel) SetSize(width, height int) {
	if width <= 0 {
		width = common.DefaultContentWidth
	}
	if height <= 0 {
		height = defaultLogViewportHeight
	}

	m.viewport.Width = width
	m.viewport.Height = height
	m.refreshContent()
}

func (m *LogViewerModel) SetReport(report cleaner.Report) {
	m.report = &report
	m.refreshContent()
	m.viewport.GotoBottom()
}

func (m *LogViewerModel) SetMouseFocused(focused bool) {
	m.mouseFocused = focused
}

func (m *LogViewerModel) refreshContent() {
	if m.report == nil {
		return
	}

	m.viewport.SetContent(renderActivity(*m.report, m.viewport.Width))
}

func (m LogViewerModel) Update(msg tea.Msg) (LogViewerModel, tea.Cmd) {
	if m.report == nil {
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m LogViewerModel) IsKeyScrollInput(msg tea.KeyMsg) bool {
	if m.report == nil {
		return false
	}

	switch {
	case key.Matches(msg, m.viewport.KeyMap.Up, m.viewport.KeyMap.PageUp, m.viewport.KeyMap.HalfPageUp):
		return true
	case key.Matches(msg, m.viewport.KeyMap.Down, m.viewport.KeyMap.PageDown, m.viewport.KeyMap.HalfPageDown):
		return true
	}

	return false
}

func (m LogViewerModel) IsFocusedMouseScrollInput(msg tea.MouseMsg) bool {
	if m.report == nil || !m.mouseFocused || !m.viewport.MouseWheelEnabled || msg.Action != tea.MouseActionPress {
		return false
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
		return true
	default:
		return false
	}
}

func (m LogViewerModel) View() string {
	if m.report == nil {
		return ""
	}

	var b strings.Builder
	report := *m.report
	width := m.viewport.Width
	if width <= 0 {
		width = common.DefaultContentWidth
	}

	b.WriteString(common.Success.Render("Last run"))
	b.WriteString("\n")
	if report.LogPath != "" {
		for _, line := range common.WrapLine("  log: ", report.LogPath, width) {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	stats := fmt.Sprintf("dry-run: %d  deleted: %d  skipped: %d  warnings: %d  errors: %d",
		report.DryRuns,
		report.Deleted,
		report.Skipped,
		report.Warnings,
		report.Errors,
	)
	for _, line := range common.WrapLine("  ", stats, width) {
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString("Recent activity\n")
	b.WriteString(m.viewport.View())

	return b.String()
}

func runCleaner(ctx context.Context, options cleaner.Options) tea.Cmd {
	return func() tea.Msg {
		report, err := cleaner.Run(ctx, options)
		return cleanerFinishedMsg{
			report: report,
			err:    err,
		}
	}
}

func renderActivity(report cleaner.Report, width int) string {
	if len(report.Entries) == 0 {
		return common.Muted.Render("  No activity")
	}

	if width <= 0 {
		width = common.DefaultContentWidth
	}

	var b strings.Builder
	for _, entry := range report.Entries {
		prefix := fmt.Sprintf("  [%s] ", entry.Level)
		for _, line := range common.WrapLine(prefix, entry.Message, width) {
			b.WriteString(styleActivityLine(entry.Level, line))
			b.WriteString("\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func styleActivityLine(level cleaner.Level, line string) string {
	switch level {
	case cleaner.LevelWarn:
		return common.Warning.Render(line)
	case cleaner.LevelError:
		return common.Error.Render(line)
	case cleaner.LevelDelete:
		return common.Success.Render(line)
	case cleaner.LevelDryRun:
		return common.Accent.Render(line)
	default:
		return common.Muted.Render(line)
	}
}

func cleanerRow(selected bool, marker, label string) string {
	cursor := " "
	renderedLabel := label
	if selected {
		cursor = common.Selected.Render(">")
		renderedLabel = common.Selected.Render(label)
	}

	text := strings.TrimSpace(strings.Join([]string{marker, renderedLabel}, " "))
	return fmt.Sprintf("  %s %s\n", cursor, text)
}

func confirmationRow(selected bool, label string) string {
	return cleanerRow(selected, "", label)
}

func credentialManagerHint() string {
	if runtime.GOOS != osWindows {
		return " (Windows only)"
	}
	return ""
}

func batch(cmds ...tea.Cmd) tea.Cmd {
	filtered := make([]tea.Cmd, 0, len(cmds))
	for _, cmd := range cmds {
		if cmd != nil {
			filtered = append(filtered, cmd)
		}
	}

	if len(filtered) == 0 {
		return nil
	}

	return tea.Batch(filtered...)
}

func wrapIndex(value, count int) int {
	if count <= 0 {
		return 0
	}

	value %= count
	if value < 0 {
		value += count
	}

	return value
}
