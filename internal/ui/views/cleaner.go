package views

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

type optionsFit int

const (
	fitFull optionsFit = iota
	fitWithoutSubtitle
	fitWithoutNotes
	fitShortDetails
	fitScrollList
	fitDialogOnly
)

type ViewState int

const (
	StateSelectingOptions ViewState = iota
	StatePromptingMode
	StateConfirmingExecute
	StateRunning
	StateFinished
)

const (
	logSectionErrors   = "errors"
	logSectionWarnings = "warnings"
	logSectionRemovals = "removals"
	logSectionSkipped  = "skipped"
	logSectionNotes    = "notes"

	cleanerShortTitle           = "Cleaner"
	cleanerShortTitleMaxWidth   = 60
	cleanerTwoColumnMinWidth    = 100
	cleanerOptionsPanelMinWidth = 66
	cleanerShortDetailLines     = 2
	cleanerActivityMinHeight    = 3
	cleanerDefaultBodyHeight    = 21
)

type cleanerKeyMap struct {
	Move          key.Binding
	Toggle        key.Binding
	Continue      key.Binding
	Menu          key.Binding
	Choose        key.Binding
	Select        key.Binding
	Run           key.Binding
	Execute       key.Binding
	ConfirmPrev   key.Binding
	ConfirmNext   key.Binding
	ConfirmChoose key.Binding
	CancelPrompt  key.Binding
	CancelConfirm key.Binding
	Scroll        key.Binding
	ToggleSkipped key.Binding
	BackToOptions key.Binding
}

type cleanerContextualKeyMap struct {
	cleanerKeyMap
	state          ViewState
	canFoldSkipped bool
}

type CleanerRunFunc func(context.Context, cleaner.Options) (cleaner.Report, error)

type CleanerModel struct {
	run           CleanerRunFunc
	spinner       spinner.Model
	keyMap        cleanerKeyMap
	optionsList   common.CheckboxListModel
	logViewer     common.LogViewer
	options       cleaner.Options
	report        *cleaner.Report
	err           error
	canceled      bool
	cancelCleanup context.CancelFunc
	notice        string
	home          string
	width         int
	height        int
	startedAt     time.Time
	state         ViewState
	confirmation  executeConfirmationSelection
	modeSelection cleanupModeSelection
}

type cleanerFinishedMsg struct {
	report   cleaner.Report
	err      error
	canceled bool
}

var _ tea.Model = CleanerModel{}

func newCleanerKeyMap() cleanerKeyMap {
	return cleanerKeyMap{
		Move:          key.NewBinding(key.WithKeys("up", "down", "k", "j"), key.WithHelp("↑↓", "move")),
		Toggle:        key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
		Continue:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "run")),
		Menu:          key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("esc", "menu")),
		Choose:        key.NewBinding(key.WithKeys("up", "down", "k", "j"), key.WithHelp("↑↓", "choose")),
		Select:        key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("enter", "select")),
		Run:           key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "dry-run")),
		Execute:       key.NewBinding(key.WithKeys("e", "x"), key.WithHelp("e", "execute")),
		ConfirmPrev:   key.NewBinding(key.WithKeys("left", "h")),
		ConfirmNext:   key.NewBinding(key.WithKeys("right", "l")),
		ConfirmChoose: key.NewBinding(key.WithKeys("left", "right", "h", "l"), key.WithHelp("←→", "choose")),
		CancelPrompt:  key.NewBinding(key.WithKeys("n", "esc"), key.WithHelp("esc", "back")),
		CancelConfirm: key.NewBinding(key.WithKeys("n", "esc"), key.WithHelp("n/esc", "cancel")),
		Scroll:        key.NewBinding(key.WithKeys("up", "down", "pgup", "pgdown"), key.WithHelp("↑↓", "scroll")),
		ToggleSkipped: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "skipped")),
		BackToOptions: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "options")),
	}
}

func (k cleanerContextualKeyMap) ShortHelp() []key.Binding {
	switch k.state {
	case StateRunning:
		return []key.Binding{common.DefaultKeys.CancelRun, common.DefaultKeys.CancelAndQuit}
	case StateConfirmingExecute:
		return []key.Binding{k.ConfirmChoose, k.Select, common.DefaultKeys.Yes, k.CancelConfirm}
	case StatePromptingMode:
		return []key.Binding{k.Run, k.Execute, k.Choose, k.Select, k.CancelPrompt}
	case StateFinished:
		if k.canFoldSkipped {
			return []key.Binding{k.Scroll, k.ToggleSkipped, k.BackToOptions, k.Menu}
		}
		return []key.Binding{k.Scroll, k.BackToOptions, k.Menu}
	default:
		return []key.Binding{k.Move, k.Toggle, k.Continue, k.Menu}
	}
}

func (k cleanerContextualKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

func NewCleanerModel() CleanerModel {
	return NewCleanerModelWithRunner(cleaner.Run)
}

func NewCleanerModelWithRunner(run CleanerRunFunc) CleanerModel {
	home, _ := os.UserHomeDir()
	model := CleanerModel{
		run: run,
		spinner: spinner.New(
			spinner.WithSpinner(trimmedSpinner(spinner.Dot)),
			spinner.WithStyle(common.Accent),
		),
		keyMap:      newCleanerKeyMap(),
		optionsList: newCleanerOptionsList(),
		logViewer:   common.NewLogViewer(),
		home:        home,
	}
	model.layoutComponents()
	return model
}

func newCleanerOptionsList() common.CheckboxListModel {
	optionsList := common.NewCheckboxList([]common.CheckboxItem{
		{
			ID:    optionSSHKeys,
			Label: "Include SSH keys",
			Details: []string{
				"Adds .ssh/config, known_hosts, id_* key files, and any other file directly in .ssh that starts with a PRIVATE KEY header.",
				"Execute deletes local SSH keys; dry-run only lists those file deletions.",
			},
			FilterText: "ssh keys credentials private public",
		},
		{
			ID:    optionBrowserProfiles,
			Label: "Include browser profiles",
			Details: []string{
				"Adds full Chrome, Edge, Brave, CocCoc, Firefox, and Safari profile folders and their caches.",
				"Execute removes local sign-ins, cookies/sessions, saved passwords, extensions, local storage, history, and bookmarks.",
			},
			FilterText: "browser profiles include caches",
		},
		{
			ID:      optionCredentialManager,
			Label:   "Clean Windows Credential Manager allowlist",
			Tag:     credentialManagerTag(),
			TagTone: common.ToneSubtle,
			Details: []string{
				"Windows only: scans Credential Manager for allowlisted dev entries such as Git, cloud CLIs, Docker, kube, npm, Terraform, Visual Studio, VS Code, Copilot, and AI tools.",
				"Dry-run lists matching entries; execute deletes only those allowlisted matches.",
			},
			FilterText: "windows credential manager allowlist credentials",
		},
		{
			ID:      optionForceStop,
			Label:   "Force stop running target processes",
			Tag:     "also in dry-run",
			TagTone: common.ToneWarning,
			Details: []string{
				"Stops running Chrome, Edge, Firefox, VS Code, and Visual Studio before cleanup so locked auth/profile files can be handled.",
				"This happens in dry-run too. Dry-run still only logs file and Credential Manager deletions.",
			},
			FilterText: "force stop kill running target processes browsers ides ai apps",
		},
		{
			ID:    optionShellHistory,
			Label: "Clean shell and tool history",
			Details: []string{
				"Adds bash, zsh, fish, PowerShell, Python, Node, database, and debugger history files, which may hold typed secrets.",
				"Execute deletes those history files; dry-run only lists them.",
			},
			FilterText: "shell tool history repl database debugger secrets",
		},
		{
			ID:    optionFullToolReset,
			Label: "Full tool reset",
			Details: []string{
				"Adds whole tool folders and settings: .gitconfig, .mongorc.js, .aws/config, .claude, .codex, .gemini, .bun, .deno, .lima, .colima, .minikube, .vagrant.d, .jupyter, cloud CLI folders, VS Code global state (settings database, extension state, sign-ins) and other IDE data, and Copilot extensions.",
				"Execute removes installed runtimes, local VMs, tool settings, and IDE history and backups; dry-run only lists them.",
			},
			FilterText: "full tool reset folders settings runtimes vms ide ai",
		},
	}, 0, 0)
	optionsList.SelectByID(optionBrowserProfiles)
	return optionsList
}

func cleanerOptionSummaries() []struct{ id, summary string } {
	return []struct{ id, summary string }{
		{optionSSHKeys, "SSH keys"},
		{optionBrowserProfiles, "Browser profiles and caches"},
		{optionCredentialManager, "Windows Credential Manager allowlist entries"},
		{optionForceStop, "Force stop running browsers and IDEs first"},
		{optionShellHistory, "Shell and tool histories"},
		{optionFullToolReset, "Full tool reset: whole tool folders and IDE data"},
	}
}

func (m CleanerModel) Init() tea.Cmd {
	return nil
}

func (m CleanerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	laidOut := next.(CleanerModel)
	laidOut.layoutComponents()
	return laidOut, cmd
}

func (m CleanerModel) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if m.state == StateRunning {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case cleanerFinishedMsg:
		if m.cancelCleanup != nil {
			m.cancelCleanup()
			m.cancelCleanup = nil
		}
		m.state = StateFinished
		m.report = &msg.report
		m.err = msg.err
		m.canceled = msg.canceled
		m.notice = ""
		m.logViewer.SetMouseFocused(false)
		m.logViewer.SetSections(cleanerLogSections(msg.report, cleanerUnreportedError(msg.report, msg.err, msg.canceled), m.options.Execute, m.home))
		return m, nil
	case tea.KeyMsg:
		switch m.state {
		case StateRunning:
			if key.Matches(msg, common.DefaultKeys.CancelRun, common.DefaultKeys.CancelAndQuit) {
				return m.cancelRunningCleanup(common.IsForceQuit(msg))
			}
			return m, batch(cmds...)
		case StateConfirmingExecute:
			return m.updateExecuteConfirmation(msg)
		case StatePromptingMode:
			return m.updateModePrompt(msg)
		case StateFinished:
			return m.updateFinished(msg)
		case StateSelectingOptions:
			return m.updateOptions(msg)
		}
	case tea.MouseMsg:
		if m.state == StateFinished {
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				m.logViewer.SetMouseFocused(m.mouseInActivity(msg))
				return m, nil
			}
			if m.logViewer.IsFocusedMouseScrollInput(msg) {
				var cmd tea.Cmd
				m.logViewer, cmd = m.logViewer.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, batch(cmds...)
}

func (m CleanerModel) updateOptions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, common.DefaultKeys.Up):
		m.optionsList = m.optionsList.MoveUp()
	case key.Matches(msg, common.DefaultKeys.Down):
		m.optionsList = m.optionsList.MoveDown()
	case key.Matches(msg, common.DefaultKeys.Space):
		return m.toggleFocusedOption()
	case key.Matches(msg, common.DefaultKeys.Enter):
		return m.openModePrompt()
	}
	return m, nil
}

func (m CleanerModel) updateFinished(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.logViewer.IsKeyScrollInput(msg):
		var cmd tea.Cmd
		m.logViewer, cmd = m.logViewer.Update(msg)
		return m, cmd
	case key.Matches(msg, m.keyMap.ToggleSkipped):
		m.logViewer.ToggleFolded(logSectionSkipped)
	case key.Matches(msg, m.keyMap.BackToOptions):
		m.state = StateSelectingOptions
	}
	return m, nil
}

func (m CleanerModel) updateExecuteConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, common.DefaultKeys.Yes):
		m.options.Execute = true
		return m.startRun()
	case key.Matches(msg, m.keyMap.CancelConfirm):
		return m.cancelExecuteConfirmation()
	case key.Matches(msg, common.DefaultKeys.Up, m.keyMap.ConfirmPrev):
		m.confirmation = executeConfirmationSelection(wrapIndex(int(m.confirmation)-1, int(confirmationCount)))
	case key.Matches(msg, common.DefaultKeys.Down, m.keyMap.ConfirmNext):
		m.confirmation = executeConfirmationSelection(wrapIndex(int(m.confirmation)+1, int(confirmationCount)))
	case key.Matches(msg, common.DefaultKeys.Enter, common.DefaultKeys.Space):
		if m.confirmation == confirmationRun {
			m.options.Execute = true
			return m.startRun()
		}
		return m.cancelExecuteConfirmation()
	case key.Matches(msg, m.keyMap.Run):
		m.options.Execute = false
		return m.startRun()
	}
	return m, nil
}

func (m CleanerModel) cancelExecuteConfirmation() (tea.Model, tea.Cmd) {
	m.state = StateSelectingOptions
	m.notice = "Canceled; nothing was deleted."
	return m, nil
}

func (m CleanerModel) updateModePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keyMap.Run):
		m.options.Execute = false
		return m.startRun()
	case key.Matches(msg, m.keyMap.Execute):
		m.openExecuteConfirmation()
	case key.Matches(msg, m.keyMap.CancelPrompt):
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
			m.openExecuteConfirmation()
		case cleanupModeCancel:
			m.state = StateSelectingOptions
		}
	}
	return m, nil
}

func (m *CleanerModel) openExecuteConfirmation() {
	m.state = StateConfirmingExecute
	m.confirmation = confirmationCancel
	m.err = nil
}

func (m CleanerModel) openModePrompt() (tea.Model, tea.Cmd) {
	m.syncOptionsFromList()
	m.state = StatePromptingMode
	m.modeSelection = cleanupModeDryRun
	m.err = nil
	m.notice = ""
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
	m.options = m.syncedOptions()
}

func (m CleanerModel) syncedOptions() cleaner.Options {
	options := m.options
	options.CleanSSHKeys = m.optionsList.Checked(optionSSHKeys)
	options.IncludeBrowserProfiles = m.optionsList.Checked(optionBrowserProfiles)
	options.CleanCredentialManager = m.optionsList.Checked(optionCredentialManager)
	options.ForceStopProcesses = m.optionsList.Checked(optionForceStop)
	options.CleanShellHistory = m.optionsList.Checked(optionShellHistory)
	options.FullToolReset = m.optionsList.Checked(optionFullToolReset)
	return options
}

func (m CleanerModel) startRun() (tea.Model, tea.Cmd) {
	m.state = StateRunning
	m.report = nil
	m.err = nil
	m.canceled = false
	m.notice = ""
	m.options = m.syncedOptions()
	m.startedAt = time.Now()

	options := m.options
	ctx, cancel := context.WithTimeout(context.Background(), cleanerRunTimeout)
	m.cancelCleanup = cancel
	return m, batch(m.spinner.Tick, runCleaner(ctx, m.run, options))
}

func (m CleanerModel) cancelRunningCleanup(quitAfter bool) (tea.Model, tea.Cmd) {
	if m.cancelCleanup != nil {
		m.cancelCleanup()
		m.cancelCleanup = nil
	}
	m.notice = cancelingNotice("cleanup", quitAfter)
	return m, nil
}

func (m CleanerModel) Running() bool {
	return m.state == StateRunning
}

func (m CleanerModel) OwnsKeys() bool {
	return m.Running() || m.state == StatePromptingMode || m.state == StateConfirmingExecute
}

func (m CleanerModel) Breadcrumb() []string {
	if m.width > 0 && m.width < cleanerShortTitleMaxWidth {
		return []string{cleanerShortTitle}
	}
	return []string{cleanerTitle}
}

func (m CleanerModel) FooterKeys() help.KeyMap {
	return cleanerContextualKeyMap{
		cleanerKeyMap:  m.keyMap,
		state:          m.state,
		canFoldSkipped: m.report != nil && notPresentCount(*m.report) > 0,
	}
}

func (m CleanerModel) FooterStatus() string {
	switch m.state {
	case StateSelectingOptions:
		return fmt.Sprintf("%d/%d checked", m.checkedCount(), m.optionsList.Len())
	case StatePromptingMode:
		switch m.modeSelection {
		case cleanupModeDryRun:
			return modePill(false)
		case cleanupModeExecute:
			return modePill(true)
		}
		return ""
	case StateConfirmingExecute:
		return modePill(true)
	default:
		return modePill(m.options.Execute)
	}
}

func modePill(execute bool) string {
	if execute {
		return common.Pill("EXECUTE", common.ToneDanger)
	}
	return common.Pill("DRY-RUN", common.ToneAccent)
}

func (m CleanerModel) checkedCount() int {
	count := 0
	for _, option := range cleanerOptionSummaries() {
		if m.optionsList.Checked(option.id) {
			count++
		}
	}
	return count
}

func (m CleanerModel) bodySize() (int, int) {
	width, height := m.width, m.height
	if width <= 0 {
		width = common.DefaultContentWidth
	}
	if height <= 0 {
		height = cleanerDefaultBodyHeight
	}
	return width, height
}

func (m CleanerModel) View() string {
	width, height := m.bodySize()
	notice := m.renderNotice(width)
	contentHeight := height - lipgloss.Height(notice)
	if notice == "" {
		contentHeight = height
	}

	var content string
	switch m.state {
	case StateRunning:
		content = m.renderRunning(width)
	case StateFinished:
		content = m.renderResult(width)
	default:
		content = m.renderOptionsFitted(width, contentHeight)
	}

	if notice == "" {
		return common.FitHeight(content, height)
	}
	return common.FitHeight(content, contentHeight) + "\n" + notice
}

func (m CleanerModel) renderNotice(width int) string {
	if m.notice == "" {
		return ""
	}
	return common.Notice(width, common.ToneWarning, m.notice)
}

func (m CleanerModel) renderOptionsFitted(width, height int) string {
	var body string
	for fit := fitFull; fit <= fitDialogOnly; fit++ {
		if fit == fitDialogOnly && m.state == StateSelectingOptions {
			break
		}
		body = m.renderOptions(width, height, fit)
		if lipgloss.Height(body) <= height {
			return body
		}
	}
	return body
}

func (m CleanerModel) renderOptions(width, height int, fit optionsFit) string {
	var blocks []string
	if fit < fitWithoutSubtitle {
		blocks = append(blocks, common.RenderWrapped(width, cleanerSubtitle, common.Muted.Render), "")
	}

	if width >= cleanerTwoColumnMinWidth && fit < fitDialogOnly {
		leftWidth := common.MaxInt(width*45/100, cleanerOptionsPanelMinWidth)
		rightWidth := width - leftWidth - 1
		left := m.renderOptionsPanel(leftWidth, 0)
		right := []string{m.renderActionPanel(rightWidth, fit)}
		if fit < fitWithoutNotes {
			right = append(right, common.Panel{Title: "Always", Width: rightWidth}.Render(m.renderNotes(rightWidth-4)))
		}
		blocks = append(blocks, lipgloss.JoinHorizontal(lipgloss.Top, left, " ", lipgloss.JoinVertical(lipgloss.Left, right...)))
		return strings.Join(blocks, "\n")
	}

	action := m.renderActionPanel(width, fit)
	if fit < fitDialogOnly {
		listHeight := 0
		if fit >= fitScrollList {
			used := lipgloss.Height(strings.Join(blocks, "\n")) + lipgloss.Height(action)
			if len(blocks) == 0 {
				used = lipgloss.Height(action)
			}
			listHeight = common.MaxInt(1, height-used-2)
		}
		blocks = append(blocks, m.renderOptionsPanel(width, listHeight))
	}
	blocks = append(blocks, action)
	if fit < fitWithoutNotes {
		blocks = append(blocks, m.renderNotes(width))
	}
	return strings.Join(blocks, "\n")
}

func (m CleanerModel) renderOptionsPanel(width, listHeight int) string {
	variant := common.PanelNormal
	if m.state == StateSelectingOptions {
		variant = common.PanelFocused
	}
	panel := common.Panel{
		Title:   "Options",
		Meta:    fmt.Sprintf("%d/%d", m.checkedCount(), m.optionsList.Len()),
		Variant: variant,
		Width:   width,
	}
	list := m.optionsList
	if listHeight <= 0 {
		listHeight = list.Len()
	}
	list.SetSize(panel.InnerWidth(), listHeight)
	list.SetFocused(m.state == StateSelectingOptions)
	return panel.Render(list.View())
}

func (m CleanerModel) renderActionPanel(width int, fit optionsFit) string {
	switch m.state {
	case StatePromptingMode:
		return m.renderModePanel(width)
	case StateConfirmingExecute:
		return m.renderExecuteConfirmation(width)
	default:
		return m.renderDetailPanel(width, fit >= fitShortDetails)
	}
}

func (m CleanerModel) renderDetailPanel(width int, short bool) string {
	selected := m.optionsList.Selected()
	panel := common.Panel{Title: selected.Label, Width: width}
	var lines []string
	for _, detail := range selected.Details {
		lines = append(lines, common.WrapPlain(detail, panel.InnerWidth())...)
	}
	if short && len(lines) > cleanerShortDetailLines {
		lines = lines[:cleanerShortDetailLines]
		lines[cleanerShortDetailLines-1] = common.Truncate(lines[cleanerShortDetailLines-1]+"…", panel.InnerWidth())
	}
	return panel.Render(strings.Join(lines, "\n"))
}

func (m CleanerModel) renderModePanel(width int) string {
	panel := common.Panel{Title: "Run cleanup", Variant: common.PanelFocused, Width: width}
	inner := panel.InnerWidth()
	lines := []string{
		common.RenderPlainWrapped(inner, "Choose cleanup mode for the selected options."),
		common.RenderPlainWrapped(inner, "Selected: "+m.selectedSummary()),
	}
	if m.optionsList.Checked(optionForceStop) {
		lines = append(lines, common.Notice(inner, common.ToneWarning, "Force stop is on: running browsers and IDEs are stopped in dry-run too."))
	}
	lines = append(lines,
		"",
		common.RenderChoices(inner, []common.Choice{
			{Label: "Dry-run", Detail: "only list what would be deleted"},
			{Label: "Execute", Detail: "delete matching files (asks again)"},
			{Label: "Cancel", Detail: "back to options"},
		}, int(m.modeSelection)),
	)
	return panel.Render(strings.Join(lines, "\n"))
}

func (m CleanerModel) renderExecuteConfirmation(width int) string {
	lines := []string{"Execute mode will delete matching local files. Choose an option and press enter."}
	for _, item := range m.executeItems() {
		lines = append(lines, "• "+item)
	}
	lines = append(lines, cleanerSafetyNotice, "Not undoable. Run a dry-run first to see the exact list.")
	return common.ConfirmDialog(width, "Delete files", lines, []string{"Cancel", "Run execute cleanup"}, int(m.confirmation))
}

func (m CleanerModel) executeItems() []string {
	items := []string{"Baseline credential and token files"}
	for _, option := range cleanerOptionSummaries() {
		if m.optionsList.Checked(option.id) {
			items = append(items, option.summary)
		}
	}
	return items
}

func (m CleanerModel) selectedSummary() string {
	var selected []string
	for _, option := range cleanerOptionSummaries() {
		if m.optionsList.Checked(option.id) {
			selected = append(selected, strings.ToLower(option.summary[:1])+option.summary[1:])
		}
	}
	if len(selected) == 0 {
		return "baseline only"
	}
	return "baseline, " + strings.Join(selected, ", ")
}

func (m CleanerModel) renderNotes(width int) string {
	return common.RenderWrapped(width, cleanerBaselineNote+" "+cleanerSafetyNotice, common.Muted.Render)
}

func (m CleanerModel) renderRunning(width int) string {
	label := "Running dry-run cleanup…"
	note := "Dry-run only lists deletions; nothing is deleted."
	if m.options.Execute {
		label = "Running execute cleanup…"
		note = "Execute deletes matching files inside your user profile."
	}
	return common.RunStatus(width, m.spinner.View(), label, note, m.elapsed())
}

func (m CleanerModel) elapsed() time.Duration {
	if m.startedAt.IsZero() {
		return 0
	}
	return time.Since(m.startedAt)
}

func (m CleanerModel) renderResult(width int) string {
	header := m.renderResultHeader(width)
	panel := common.Panel{Title: "Activity", Meta: m.logViewer.Position(), Width: width}
	if !m.logViewer.HasContent() {
		return header + "\n" + panel.Render(common.Muted.Italic(true).Render("No activity"))
	}
	panel.Height = common.MinInt(m.logViewer.Height(), m.logViewer.TotalLines()) + 2
	return header + "\n" + panel.Render(m.logViewer.View())
}

func (m CleanerModel) renderResultHeader(width int) string {
	if m.report == nil {
		return ""
	}
	report := *m.report
	lines := []string{m.renderResultSummary(width), common.RenderCounts(width, cleanerCounts(report, m.options.Execute))}
	if report.LogPath != "" {
		lines = append(lines, strings.Join(common.WrapLine("log ", displayPath(m.home, report.LogPath), width), "\n"))
	}
	return strings.Join(lines, "\n")
}

func (m CleanerModel) renderResultSummary(width int) string {
	switch {
	case m.canceled:
		return common.Notice(width, common.ToneWarning, "Canceled. The run stopped early; the activity below shows what it did before it stopped.")
	case m.err != nil:
		return common.Notice(width, common.ToneDanger, "Completed with errors · see ERRORS below")
	case m.options.Execute:
		return common.Notice(width, common.ToneSuccess, fmt.Sprintf("Execute finished · %d deleted", m.report.Deleted))
	default:
		return common.Notice(width, common.ToneSuccess, "Dry-run finished · nothing was deleted")
	}
}

func (m *CleanerModel) layoutComponents() {
	width, height := m.bodySize()
	inner := common.MaxInt(1, width-4)
	activityHeight := height - 2 - lipgloss.Height(m.renderResultHeader(width))
	if m.notice != "" {
		activityHeight -= lipgloss.Height(m.renderNotice(width))
	}
	m.logViewer.SetSize(inner, common.MaxInt(cleanerActivityMinHeight, activityHeight))
}

func (m CleanerModel) mouseInActivity(msg tea.MouseMsg) bool {
	if m.report == nil {
		return false
	}
	width, _ := m.bodySize()
	top := lipgloss.Height(m.renderResultHeader(width))
	bottom := top + m.logViewer.Height() + 1
	return msg.Y >= top && msg.Y <= bottom
}

func cleanerCounts(report cleaner.Report, execute bool) []common.Count {
	removals := common.Count{Label: "would delete", N: report.DryRuns, Tone: common.ToneAccent}
	if execute {
		removals = common.Count{Label: "deleted", N: report.Deleted, Tone: common.ToneSuccess}
	}
	return []common.Count{
		removals,
		{Label: "skipped", N: report.Skipped, Tone: common.ToneSubtle},
		{Label: "warnings", N: report.Warnings, Tone: common.ToneWarning},
		{Label: "errors", N: report.Errors, Tone: common.ToneDanger},
	}
}

func cleanerUnreportedError(report cleaner.Report, err error, canceled bool) error {
	if err == nil || canceled || report.Errors > 0 {
		return nil
	}
	return err
}

func cleanerLogSections(report cleaner.Report, unreported error, execute bool, home string) []common.LogSection {
	errorsSection := common.LogSection{ID: logSectionErrors, Title: "ERRORS", Tone: common.ToneDanger}
	warnings := common.LogSection{ID: logSectionWarnings, Title: "WARNINGS", Tone: common.ToneWarning}
	removals := common.LogSection{ID: logSectionRemovals, Title: "WOULD DELETE", Glyph: "○", Tone: common.ToneAccent}
	if execute {
		removals.Title = "DELETED"
		removals.Glyph = "−"
		removals.Tone = common.ToneSuccess
	}
	skipped := common.LogSection{ID: logSectionSkipped, Title: "SKIPPED", Glyph: "·", Tone: common.ToneSubtle}
	notes := common.LogSection{ID: logSectionNotes, Title: "NOTES", Glyph: "i", Tone: common.ToneSubtle}

	for _, entry := range report.Entries {
		switch entry.Level {
		case cleaner.LevelError:
			errorsSection.Lines = append(errorsSection.Lines, common.LogLine{Tone: common.ToneDanger, Text: entry.Message})
		case cleaner.LevelWarn:
			warnings.Lines = append(warnings.Lines, common.LogLine{Tone: common.ToneWarning, Text: entry.Message})
		case cleaner.LevelDelete, cleaner.LevelDryRun:
			removals.Lines = append(removals.Lines, removalLine(entry, home))
		case cleaner.LevelSkip:
			if entry.NotPresent {
				skipped.Folded = append(skipped.Folded, common.LogLine{Tone: common.ToneSubtle, Text: entry.Target, Detail: displayPath(home, entry.Path)})
				continue
			}
			skipped.Lines = append(skipped.Lines, common.LogLine{Tone: common.ToneSubtle, Text: entry.Message})
		default:
			notes.Lines = append(notes.Lines, common.LogLine{Tone: common.ToneSubtle, Text: entry.Message})
		}
	}
	if unreported != nil {
		for _, line := range strings.Split(unreported.Error(), "\n") {
			errorsSection.Lines = append(errorsSection.Lines, common.LogLine{Tone: common.ToneDanger, Text: line})
		}
	}
	if len(skipped.Folded) > 0 {
		skipped.FoldedSummary = fmt.Sprintf("%d not present (s to show)", len(skipped.Folded))
		if len(skipped.Lines) > 0 {
			skipped.FoldedSummary += fmt.Sprintf(", %d shown below", len(skipped.Lines))
		}
	}

	var sections []common.LogSection
	for _, section := range []common.LogSection{errorsSection, warnings, removals, skipped, notes} {
		if len(section.Lines) > 0 || len(section.Folded) > 0 {
			sections = append(sections, section)
		}
	}
	return sections
}

func removalLine(entry cleaner.Entry, home string) common.LogLine {
	if entry.Target == "" {
		return common.LogLine{Text: entry.Message}
	}
	if entry.LinkTarget != "" {
		return common.LogLine{Text: entry.Target + " (link only)", Detail: displayPath(home, entry.Path) + " → " + displayPath(home, entry.LinkTarget) + " kept"}
	}
	return common.LogLine{Text: entry.Target, Detail: displayPath(home, entry.Path)}
}

func notPresentCount(report cleaner.Report) int {
	count := 0
	for _, entry := range report.Entries {
		if entry.NotPresent {
			count++
		}
	}
	return count
}

func displayPath(home, path string) string {
	if home == "" {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || !filepath.IsLocal(rel) {
		return path
	}
	return filepath.Join("~", rel)
}

func trimmedSpinner(base spinner.Spinner) spinner.Spinner {
	frames := make([]string, len(base.Frames))
	for i, frame := range base.Frames {
		frames[i] = strings.TrimSpace(frame)
	}
	return spinner.Spinner{Frames: frames, FPS: base.FPS}
}

func runCleaner(ctx context.Context, run CleanerRunFunc, options cleaner.Options) tea.Cmd {
	return func() tea.Msg {
		report, err := run(ctx, options)
		return cleanerFinishedMsg{
			report:   report,
			err:      err,
			canceled: runWasCanceled(ctx, err),
		}
	}
}

func runWasCanceled(ctx context.Context, err error) bool {
	return err != nil && errors.Is(ctx.Err(), context.Canceled)
}

func cancelingNotice(subject string, quitAfter bool) string {
	if quitAfter {
		return "Canceling " + subject + "; UTILS quits when it stops. Press ctrl+c again to quit now and leave the current step unfinished."
	}
	return "Canceling " + subject + "..."
}

func credentialManagerTag() string {
	if runtime.GOOS != osWindows {
		return "Windows only"
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
