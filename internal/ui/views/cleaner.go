package views

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

type cleanerState int

const (
	cleanerScope cleanerState = iota
	cleanerRunning
	cleanerResult
)

type cleanerFocus int

const (
	focusScope cleanerFocus = iota
	focusPreview
)

type CleanerPlanFunc func(context.Context, cleaner.Options) (cleaner.Plan, error)

type CleanerRunFunc func(context.Context, cleaner.Options) (cleaner.Report, error)

type CleanerSaveFunc func(string, cleaner.Report) (string, error)

type scopeRow struct {
	id          cleaner.GroupID
	name        string
	help        string
	notUndoable bool
	destructive bool
}

var cleanerScopeRows = []scopeRow{
	{cleaner.GroupCredentials, "Credentials and tokens", "Token and credential files for cloud, Git, package and AI tools. Always on.", true, false},
	{cleaner.GroupSSHKeys, "SSH keys", "Private keys in ~/.ssh, ssh config and known_hosts. Off by default.", true, true},
	{cleaner.GroupShellHistory, "Shell and tool history", "Shell, REPL, database and debugger histories that may hold typed secrets.", true, false},
	{cleaner.GroupBrowserProfiles, "Browser profiles", "Whole browser profiles and caches: sign-ins, cookies, passwords, bookmarks.", true, true},
	{cleaner.GroupFullToolReset, "Full tool reset", "Whole tool folders: runtimes, VMs, IDE data, AI tool data and .gitconfig.", true, true},
	{cleaner.GroupCredentialManager, "Windows Credential Mgr", "Allowlisted dev entries in Windows Credential Manager. Windows only.", true, false},
	{cleaner.GroupForceStop, "Force-stop apps", "Stops running browsers and editors right before deleting. Never in the preview.", false, false},
}

type cleanerKeyMap struct {
	Move       key.Binding
	Toggle     key.Binding
	Delete     key.Binding
	SaveDryRun key.Binding
	ToPreview  key.Binding
	ToScope    key.Binding
	Page       key.Binding
	Info       key.Binding
	Scroll     key.Binding
	Back       key.Binding
	SaveLog    key.Binding
	Cancel     key.Binding
	CancelQuit key.Binding
	PageUp     key.Binding
	PageDown   key.Binding
	Left       key.Binding
	Right      key.Binding
	ScrollUp   key.Binding
	ScrollDown key.Binding
}

func newCleanerKeyMap() cleanerKeyMap {
	return cleanerKeyMap{
		Move:       key.NewBinding(key.WithKeys("up", "down", "k", "j"), key.WithHelp("↑↓", "move")),
		Toggle:     key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
		Delete:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "delete…")),
		SaveDryRun: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "save dry-run log")),
		ToPreview:  key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→", "preview")),
		ToScope:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←", "scope")),
		Page:       key.NewBinding(key.WithKeys("pgup", "pgdown"), key.WithHelp("pgup pgdn", "scroll")),
		Info:       key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "info")),
		Scroll:     key.NewBinding(key.WithKeys("up", "down", "k", "j"), key.WithHelp("↑↓", "scroll")),
		Back:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "back to scope")),
		SaveLog:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "save log")),
		Cancel:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		CancelQuit: key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "cancel and quit")),
		PageUp:     key.NewBinding(key.WithKeys("pgup")),
		PageDown:   key.NewBinding(key.WithKeys("pgdown")),
		Left:       key.NewBinding(key.WithKeys("left", "h")),
		Right:      key.NewBinding(key.WithKeys("right", "l")),
		ScrollUp:   key.NewBinding(key.WithKeys("up", "k")),
		ScrollDown: key.NewBinding(key.WithKeys("down", "j")),
	}
}

type CleanerModel struct {
	plan    CleanerPlanFunc
	run     CleanerRunFunc
	save    CleanerSaveFunc
	keys    cleanerKeyMap
	spinner spinner.Model
	home    string
	width   int
	height  int

	state    cleanerState
	options  cleaner.Options
	cursor   int
	focus    cleanerFocus
	showInfo bool
	confirm  common.Confirm

	preview       *cleaner.Plan
	previewErr    error
	scanSeq       int
	scanning      bool
	previewOffset int

	cancelRun context.CancelFunc
	canceling bool
	startedAt time.Time
	elapsed   time.Duration
	done      int
	total     int
	activity  []cleaner.Entry

	report       *cleaner.Report
	runErr       error
	canceled     bool
	resultOffset int
}

type cleanerPlanMsg struct {
	seq  int
	plan cleaner.Plan
	err  error
}

type cleanerProgressMsg struct {
	progress cleaner.Progress
	events   <-chan tea.Msg
}

type cleanerFinishedMsg struct {
	report   cleaner.Report
	err      error
	canceled bool
}

type cleanerLogSavedMsg struct {
	path string
	err  error
}

var _ tea.Model = CleanerModel{}

func NewCleanerModel() CleanerModel {
	return NewCleanerModelWith(cleaner.PlanCleanup, cleaner.Run, cleaner.SaveLog)
}

func NewCleanerModelWith(plan CleanerPlanFunc, run CleanerRunFunc, save CleanerSaveFunc) CleanerModel {
	home, _ := os.UserHomeDir()
	return CleanerModel{
		plan: plan,
		run:  run,
		save: save,
		keys: newCleanerKeyMap(),
		spinner: spinner.New(
			spinner.WithSpinner(trimmedSpinner(spinner.Dot)),
			spinner.WithStyle(common.Accent),
		),
		home:     home,
		scanning: true,
	}
}

func (m CleanerModel) Init() tea.Cmd {
	return scanCleanerCmd(m.plan, m.scanSeq, m.options)
}

func (m CleanerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampScroll()
		return m, nil
	case common.ActivatedMsg:
		if m.state == cleanerScope && !m.confirm.Open() {
			return m.rescan()
		}
		return m, nil
	case cleanerPlanMsg:
		if msg.seq != m.scanSeq {
			return m, nil
		}
		m.preview = &msg.plan
		m.previewErr = msg.err
		m.scanning = false
		m.clampScroll()
		return m, nil
	case cleanerProgressMsg:
		m.done, m.total = msg.progress.Done, msg.progress.Total
		m.activity = append(m.activity, msg.progress.Entry)
		return m, waitCleanerEvent(msg.events)
	case cleanerFinishedMsg:
		return m.finishRun(msg), nil
	case cleanerLogSavedMsg:
		if msg.err != nil {
			return m, common.Notify(common.ToneDanger, "Log not saved · "+rootCause(msg.err))
		}
		return m, common.Notify(common.ToneSuccess, "Log saved · "+msg.path)
	case spinner.TickMsg:
		if m.state != cleanerRunning {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		switch m.state {
		case cleanerRunning:
			return m.updateRunning(msg)
		case cleanerResult:
			return m.updateResult(msg)
		default:
			if m.confirm.Open() {
				return m.updateConfirm(msg)
			}
			return m.updateScope(msg)
		}
	}
	return m, nil
}

func (m CleanerModel) updateScope(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.PageUp):
		m.scrollPreview(-m.previewPage())
	case key.Matches(msg, m.keys.PageDown):
		m.scrollPreview(m.previewPage())
	case key.Matches(msg, m.keys.Left):
		m.focus = focusScope
	case key.Matches(msg, m.keys.Right):
		if m.previewOverflows() {
			m.focus = focusPreview
		}
	case key.Matches(msg, m.keys.ScrollUp):
		if m.focus == focusPreview {
			m.scrollPreview(-1)
		} else {
			m.cursor = wrapIndex(m.cursor-1, len(cleanerScopeRows))
		}
	case key.Matches(msg, m.keys.ScrollDown):
		if m.focus == focusPreview {
			m.scrollPreview(1)
		} else {
			m.cursor = wrapIndex(m.cursor+1, len(cleanerScopeRows))
		}
	case key.Matches(msg, m.keys.Toggle):
		if m.toggle(cleanerScopeRows[m.cursor].id) {
			return m.rescan()
		}
	case key.Matches(msg, m.keys.Delete):
		return m.openConfirm()
	case key.Matches(msg, m.keys.SaveDryRun):
		return m, saveDryRunLogCmd(m.run, m.options, defaultCleanerLogPath(m.home, time.Now()))
	case key.Matches(msg, m.keys.Info):
		if m.stacked() {
			m.showInfo = !m.showInfo
		}
	}
	return m, nil
}

func (m CleanerModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var result common.ConfirmResult
	m.confirm, result = m.confirm.Update(msg)
	switch result {
	case common.ConfirmAccepted:
		return m.startRun()
	case common.ConfirmCanceled:
		return m, common.Notify(common.ToneWarning, "Canceled; nothing was deleted.")
	}
	return m, nil
}

func (m CleanerModel) updateRunning(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !key.Matches(msg, m.keys.Cancel, m.keys.CancelQuit) {
		return m, nil
	}
	if m.cancelRun != nil {
		m.cancelRun()
	}
	m.canceling = true
	return m, common.Notify(common.ToneWarning, cancelingNotice("cleanup", common.IsForceQuit(msg)))
}

func (m CleanerModel) updateResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.ScrollUp):
		m.resultOffset--
	case key.Matches(msg, m.keys.ScrollDown):
		m.resultOffset++
	case key.Matches(msg, m.keys.PageUp):
		m.resultOffset -= m.resultPage()
	case key.Matches(msg, m.keys.PageDown):
		m.resultOffset += m.resultPage()
	case key.Matches(msg, m.keys.Back):
		m.state = cleanerScope
		m.focus = focusScope
		m.previewOffset = 0
		return m.rescan()
	case key.Matches(msg, m.keys.SaveLog):
		if m.report != nil {
			return m, saveReportLogCmd(m.save, defaultCleanerLogPath(m.home, time.Now()), *m.report)
		}
	}
	m.clampScroll()
	return m, nil
}

func (m *CleanerModel) toggle(id cleaner.GroupID) bool {
	switch id {
	case cleaner.GroupSSHKeys:
		m.options.CleanSSHKeys = !m.options.CleanSSHKeys
	case cleaner.GroupShellHistory:
		m.options.CleanShellHistory = !m.options.CleanShellHistory
	case cleaner.GroupBrowserProfiles:
		m.options.IncludeBrowserProfiles = !m.options.IncludeBrowserProfiles
	case cleaner.GroupFullToolReset:
		m.options.FullToolReset = !m.options.FullToolReset
	case cleaner.GroupCredentialManager:
		if !cleaner.GroupAvailable(id) {
			return false
		}
		m.options.CleanCredentialManager = !m.options.CleanCredentialManager
	case cleaner.GroupForceStop:
		m.options.ForceStopProcesses = !m.options.ForceStopProcesses
	default:
		return false
	}
	return true
}

func (m CleanerModel) rescan() (tea.Model, tea.Cmd) {
	m.scanSeq++
	m.scanning = true
	return m, scanCleanerCmd(m.plan, m.scanSeq, m.options)
}

func (m CleanerModel) openConfirm() (tea.Model, tea.Cmd) {
	if m.preview == nil {
		return m, common.Notify(common.ToneWarning, "Still scanning; try again in a moment.")
	}
	files, folders := m.deleteCounts()
	stops := m.stopCount()
	if files+folders == 0 && stops == 0 {
		return m, common.Notify(common.ToneWarning, "Nothing to delete with this scope.")
	}
	title := "Delete " + fileFolderPhrase(files, folders, " and ")
	if files+folders == 0 {
		title = "Force-stop " + plural(stops, "app", "apps")
	}
	m.confirm = common.NewConfirm(title, "Delete", m.confirmLines()...)
	return m, nil
}

func (m CleanerModel) confirmLines() []string {
	var lines []string
	for _, row := range cleanerScopeRows {
		if !m.rowEnabled(row) {
			continue
		}
		group := m.preview.Group(row.id)
		found := group.Found()
		if found == 0 {
			continue
		}
		extra := ""
		switch {
		case row.id == cleaner.GroupForceStop:
			extra = strings.Join(group.Processes, ", ")
		case row.destructive:
			extra = common.ToneDanger.Style().Render("not undoable")
		}
		lines = append(lines, strings.TrimRight(fmt.Sprintf("%-24s %4d   %s", row.name, found, extra), " "))
	}
	return append(lines, "", fmt.Sprintf("Only inside %s. Never through a link that leaves it. No elevation.", m.home))
}

func (m CleanerModel) startRun() (tea.Model, tea.Cmd) {
	m.state = cleanerRunning
	m.canceling = false
	m.report = nil
	m.runErr = nil
	m.canceled = false
	m.done, m.total = 0, 0
	m.activity = nil
	m.startedAt = time.Now()
	m.elapsed = 0

	options := m.options
	options.Execute = true
	options.LogPath = ""
	ctx, cancel := context.WithTimeout(context.Background(), cleanerRunTimeout)
	m.cancelRun = cancel
	return m, batch(m.spinner.Tick, runCleanerCmd(ctx, m.run, options))
}

func (m CleanerModel) finishRun(msg cleanerFinishedMsg) CleanerModel {
	if m.cancelRun != nil {
		m.cancelRun()
		m.cancelRun = nil
	}
	m.state = cleanerResult
	m.canceling = false
	m.report = &msg.report
	m.runErr = msg.err
	m.canceled = msg.canceled
	m.elapsed = time.Since(m.startedAt)
	m.resultOffset = 0
	return m
}

func (m CleanerModel) Running() bool {
	return m.state == cleanerRunning
}

func (m CleanerModel) OwnsKeys() bool {
	return m.Running() || m.confirm.Open()
}

func (m CleanerModel) Modal() string {
	if !m.confirm.Open() {
		return ""
	}
	width, height := m.bodySize()
	return m.confirm.View(width, height)
}

func (m CleanerModel) FooterKeys() help.KeyMap {
	switch {
	case m.confirm.Open():
		return m.confirm.Keys()
	case m.state == cleanerRunning:
		return common.KeyList{m.keys.Cancel, m.keys.CancelQuit}
	case m.state == cleanerResult:
		return common.KeyList{m.keys.Scroll, m.keys.Back, m.keys.SaveLog}
	case m.focus == focusPreview:
		return common.KeyList{m.keys.Scroll, m.keys.ToScope, m.keys.Page, m.keys.Delete, m.keys.SaveDryRun}
	}
	keys := common.KeyList{m.keys.Toggle, m.keys.Delete}
	if m.stacked() {
		keys = append(keys, m.keys.Info)
	}
	keys = append(keys, m.keys.SaveDryRun)
	if m.previewOverflows() {
		keys = append(keys, m.keys.ToPreview, m.keys.Page)
	}
	return keys
}

func (m CleanerModel) FooterStatus() string {
	switch m.state {
	case cleanerRunning:
		if m.canceling {
			return common.Pill("Canceling…", common.ToneWarning)
		}
		return common.Pill("EXECUTING", common.ToneDanger)
	case cleanerResult:
		if m.report == nil {
			return ""
		}
		return fmt.Sprintf("%d deleted", m.report.Deleted)
	}
	if m.preview == nil {
		return "scanning…"
	}
	files, folders := m.deleteCounts()
	return fmt.Sprintf("%d to delete", files+folders)
}

func (m CleanerModel) View() string {
	width, height := m.bodySize()
	switch m.state {
	case cleanerResult:
		return common.FitHeight(m.renderResult(width, height), height)
	case cleanerRunning:
		return common.FitHeight(m.renderPanes(width, height, m.renderProgress), height)
	default:
		return common.FitHeight(m.renderPanes(width, height, m.renderPreview), height)
	}
}

func (m CleanerModel) bodySize() (int, int) {
	width, height := m.width, m.height
	if width <= 0 {
		width = common.DefaultContentWidth - 2*common.MarginX
	}
	if height <= 0 {
		height = cleanerDefaultBodyHeight
	}
	return width, height
}

func (m CleanerModel) stacked() bool {
	width, _ := m.bodySize()
	return width < common.TwoColumnMinWidth
}

type paneRenderer func(width, height int) string

func (m CleanerModel) renderPanes(width, height int, right paneRenderer) string {
	running := m.state == cleanerRunning
	if !m.stacked() {
		left := common.MinInt(cleanerScopeWidth, width/2)
		scope := m.renderScope(left, height, !running)
		return lipgloss.JoinHorizontal(lipgloss.Top, scope, " ", right(width-left-1, height))
	}

	scopeHeight := common.MinInt(len(cleanerScopeRows)+2, height)
	blocks := []string{m.renderScope(width, scopeHeight, false)}
	used := scopeHeight
	if m.showInfo && !running {
		info := m.renderInfo(width)
		blocks = append(blocks, info)
		used += lipgloss.Height(info)
	}
	if rest := height - used; rest >= cleanerPaneMinHeight {
		blocks = append(blocks, right(width, rest))
	}
	return strings.Join(blocks, "\n")
}

func (m CleanerModel) renderScope(width, height int, withDetail bool) string {
	running := m.state == cleanerRunning
	panel := common.Panel{Title: "Scope", Width: width, Height: height}
	if !running && m.focus == focusScope {
		panel.Variant = common.PanelFocused
	}
	inner := panel.InnerWidth()
	room := common.MaxInt(0, height-2)

	start := 0
	if room < len(cleanerScopeRows) {
		start = common.MinInt(common.MaxInt(0, m.cursor-room+1), len(cleanerScopeRows)-room)
	}
	var rows []string
	for i := start; i < len(cleanerScopeRows) && len(rows) < room; i++ {
		rows = append(rows, m.renderScopeRow(i, inner, running))
	}

	var detail []string
	if withDetail {
		detail = m.detailLines(inner)
	}
	if len(detail) == 0 || len(rows)+1+len(detail) > room {
		return panel.Render(strings.Join(rows, "\n"))
	}
	for len(rows)+1+len(detail) < room {
		rows = append(rows, "")
	}
	separatorRow := len(rows) + 1
	rendered := strings.Split(panel.Render(strings.Join(append(append(rows, ""), detail...), "\n")), "\n")
	borderStyle := common.Muted
	if panel.Variant == common.PanelFocused {
		borderStyle = common.Accent
	}
	rendered[separatorRow] = borderStyle.Render("├" + strings.Repeat("─", common.MaxInt(0, width-2)) + "┤")
	return strings.Join(rendered, "\n")
}

func (m CleanerModel) renderScopeRow(index, width int, dimmed bool) string {
	row := cleanerScopeRows[index]
	enabled := m.rowEnabled(row)
	focused := index == m.cursor && !dimmed && m.focus == focusScope

	cursor := " "
	if focused {
		cursor = "▸"
	}
	glyph := "[ ]"
	switch {
	case row.id == cleaner.GroupCredentials:
		glyph = "●  "
	case enabled:
		glyph = "[x]"
	}
	name := row.name
	count := m.rowCount(row)

	if dimmed || !cleaner.GroupAvailable(row.id) {
		return common.Muted.Render(common.SpreadLine(width, cursor+glyph+" "+name, count))
	}
	if focused {
		name = common.Selected.Render(name)
	}
	countStyle := common.Muted
	switch {
	case enabled && row.destructive:
		countStyle = common.ToneDanger.Style()
	case enabled:
		countStyle = lipgloss.NewStyle()
	}
	return common.SpreadLine(width, cursor+glyph+" "+name, countStyle.Render(count))
}

func (m CleanerModel) rowCount(row scopeRow) string {
	if !cleaner.GroupAvailable(row.id) {
		return "n/a"
	}
	if m.preview == nil {
		return "…"
	}
	group := m.preview.Group(row.id)
	if row.id == cleaner.GroupForceStop {
		return fmt.Sprintf("%d running", group.Found())
	}
	return fmt.Sprintf("%d found", group.Found())
}

func (m CleanerModel) rowEnabled(row scopeRow) bool {
	return cleaner.GroupAvailable(row.id) && m.options.Enabled(row.id)
}

func (m CleanerModel) detailLines(width int) []string {
	row := cleanerScopeRows[m.cursor]
	lines := []string{lipgloss.NewStyle().Bold(true).Render(row.name)}
	lines = append(lines, common.WrapPlain(row.help, width)...)
	if row.notUndoable {
		lines = append(lines, "Not undoable.")
	}
	return lines
}

func (m CleanerModel) renderInfo(width int) string {
	panel := common.Panel{Title: cleanerScopeRows[m.cursor].name, Width: width}
	lines := m.detailLines(panel.InnerWidth())
	return panel.Render(strings.Join(lines[1:], "\n"))
}

func (m CleanerModel) renderPreview(width, height int) string {
	panel := common.Panel{Title: m.previewTitle(), Width: width, Height: height}
	if m.focus == focusPreview {
		panel.Variant = common.PanelFocused
	}
	if m.preview == nil {
		return panel.Render(common.Muted.Render("Scanning your profile…"))
	}
	lines := m.previewLines(panel.InnerWidth())
	room := common.MaxInt(1, height-2)
	offset := common.MinInt(m.previewOffset, common.MaxInt(0, len(lines)-room))
	if len(lines) > room {
		panel.Meta = scrollPosition(offset, room, len(lines))
		lines = lines[offset : offset+room]
	}
	if m.scanning {
		panel.Meta = "scanning…"
	}
	return panel.Render(strings.Join(lines, "\n"))
}

func (m CleanerModel) previewTitle() string {
	if m.preview == nil {
		return "Will delete"
	}
	files, folders := m.deleteCounts()
	if files+folders == 0 {
		return "Will delete · nothing"
	}
	return "Will delete · " + fileFolderPhrase(files, folders, " · ")
}

func (m CleanerModel) previewLines(width int) []string {
	var lines []string
	if m.previewErr != nil {
		lines = append(lines, common.Notice(width, common.ToneWarning, "Scan incomplete: "+m.previewErr.Error()), "")
	}

	if m.options.ForceStopProcesses {
		if processes := m.preview.Group(cleaner.GroupForceStop).Processes; len(processes) > 0 {
			lines = append(lines, common.ToneDanger.Style().Bold(true).Render(fmt.Sprintf("WILL FORCE-STOP · %d", len(processes))))
			for _, name := range processes {
				lines = append(lines, "  "+name)
			}
			lines = append(lines, "")
		}
	}

	var refused []cleaner.PlanEntry
	for _, row := range cleanerScopeRows {
		if row.id == cleaner.GroupForceStop || !m.rowEnabled(row) {
			continue
		}
		var entries []string
		for _, entry := range m.preview.Group(row.id).Entries {
			if entry.Refused != "" {
				refused = append(refused, entry)
				continue
			}
			entries = append(entries, m.previewEntry(entry, width))
		}
		if len(entries) == 0 {
			continue
		}
		lines = append(lines, common.Muted.Bold(true).Render(strings.ToUpper(row.name)))
		lines = append(append(lines, entries...), "")
	}

	if len(refused) > 0 {
		lines = append(lines, common.ToneWarning.Style().Bold(true).Render(fmt.Sprintf("⚠ %d refused", len(refused))))
		for _, entry := range refused {
			path := displayPath(m.home, entry.Path)
			if entry.LinkTarget != "" {
				path += " → " + displayPath(m.home, entry.LinkTarget)
			}
			lines = append(lines, common.SpreadLine(width, "  "+path, common.Muted.Render(entry.Refused)))
		}
		lines = append(lines, "")
	}

	if len(lines) == 0 {
		return []string{common.Muted.Render("Nothing to delete with this scope.")}
	}
	return lines[:len(lines)-1]
}

func (m CleanerModel) previewEntry(entry cleaner.PlanEntry, width int) string {
	path := displayPath(m.home, entry.Path)
	tag := ""
	switch {
	case entry.LinkTarget != "":
		tag = "link only"
		path += " → " + displayPath(m.home, entry.LinkTarget)
	case entry.IsDir:
		tag = "folder"
		path += string(filepath.Separator)
	}
	return common.SpreadLine(width, "  "+path, common.Muted.Render(tag))
}

func (m CleanerModel) deleteCounts() (int, int) {
	if m.preview == nil {
		return 0, 0
	}
	files, folders := 0, 0
	for _, row := range cleanerScopeRows {
		if row.id == cleaner.GroupForceStop || !m.rowEnabled(row) {
			continue
		}
		for _, entry := range m.preview.Group(row.id).Entries {
			switch {
			case entry.Refused != "":
			case entry.IsDir:
				folders++
			default:
				files++
			}
		}
	}
	return files, folders
}

func (m CleanerModel) stopCount() int {
	if m.preview == nil || !m.options.ForceStopProcesses {
		return 0
	}
	return len(m.preview.Group(cleaner.GroupForceStop).Processes)
}

func (m CleanerModel) previewRoom() int {
	width, height := m.bodySize()
	if !m.stacked() {
		return common.MaxInt(1, height-2)
	}
	used := common.MinInt(len(cleanerScopeRows)+2, height)
	if m.showInfo {
		used += lipgloss.Height(m.renderInfo(width))
	}
	return common.MaxInt(1, height-used-2)
}

func (m CleanerModel) previewLineCount() int {
	if m.preview == nil {
		return 0
	}
	width, _ := m.bodySize()
	if !m.stacked() {
		width -= common.MinInt(cleanerScopeWidth, width/2) + 1
	}
	return len(m.previewLines(common.MaxInt(1, width-4)))
}

func (m CleanerModel) previewOverflows() bool {
	return m.previewLineCount() > m.previewRoom()
}

func (m CleanerModel) previewPage() int {
	return common.MaxInt(1, m.previewRoom()-1)
}

func (m *CleanerModel) scrollPreview(delta int) {
	m.previewOffset += delta
	m.clampScroll()
}

func (m *CleanerModel) clampScroll() {
	m.previewOffset = common.MaxInt(0, common.MinInt(m.previewOffset, m.previewLineCount()-m.previewRoom()))
	if !m.previewOverflows() {
		m.focus = focusScope
	}
	m.resultOffset = common.MaxInt(0, common.MinInt(m.resultOffset, len(m.resultLines(m.resultWidth()))-m.resultPage()))
}

func (m CleanerModel) renderProgress(width, height int) string {
	panel := common.Panel{Title: "Deleting", Variant: common.PanelFocused, Width: width, Height: height}
	inner := panel.InnerWidth()
	status := fmt.Sprintf("  %d/%d · %s %s", m.done, m.total, formatTenths(time.Since(m.startedAt)), m.spinner.View())
	barWidth := common.MaxInt(cleanerBarMinWidth, inner-lipgloss.Width(status))
	lines := []string{progressBar(m.done, m.total, barWidth) + status}

	room := common.MaxInt(0, height-3)
	activity := m.activity
	if len(activity) > room {
		activity = activity[len(activity)-room:]
	}
	for _, entry := range activity {
		lines = append(lines, m.activityLine(entry, inner))
	}
	return panel.Render(strings.Join(lines, "\n"))
}

func (m CleanerModel) activityLine(entry cleaner.Entry, width int) string {
	switch entry.Level {
	case cleaner.LevelError:
		return common.Truncate(common.ToneDanger.Style().Render("✗ ")+m.entryLabel(entry)+"   "+common.Muted.Render(m.failureDetail(entry)), width)
	case cleaner.LevelDelete, cleaner.LevelWarn:
		return common.Truncate(common.ToneSuccess.Style().Render("✓ ")+m.entryLabel(entry), width)
	default:
		return common.Truncate(common.Muted.Render("· "+m.entryLabel(entry)+"   skipped"), width)
	}
}

func (m CleanerModel) entryLabel(entry cleaner.Entry) string {
	if entry.Path == "" {
		return entry.Message
	}
	label := displayPath(m.home, entry.Path)
	if entry.LinkTarget != "" {
		label += " → " + displayPath(m.home, entry.LinkTarget) + " kept"
	}
	return label
}

func (m CleanerModel) failureDetail(entry cleaner.Entry) string {
	detail := entry.Message
	if entry.Path != "" {
		if _, rest, ok := strings.Cut(detail, entry.Path+": "); ok {
			detail = rest
		}
	}
	if m.home != "" {
		detail = strings.ReplaceAll(detail, m.home, "~")
	}
	return detail
}

type resultLine struct {
	text   string
	detail string
	style  lipgloss.Style
}

func (m CleanerModel) resultWidth() int {
	width, _ := m.bodySize()
	return common.MaxInt(1, width-4)
}

func (m CleanerModel) resultPage() int {
	_, height := m.bodySize()
	return common.MaxInt(1, height-2)
}

func (m CleanerModel) renderResult(width, height int) string {
	panel := common.Panel{Title: m.resultTitle(), Variant: common.PanelFocused, Width: width, Height: height}
	lines := m.resultLines(panel.InnerWidth())
	room := common.MaxInt(1, height-2)
	offset := common.MinInt(m.resultOffset, common.MaxInt(0, len(lines)-room))
	if len(lines) > room {
		panel.Meta = scrollPosition(offset, room, len(lines))
		lines = lines[offset : offset+room]
	}
	return panel.Render(strings.Join(lines, "\n"))
}

func (m CleanerModel) resultTitle() string {
	if m.report == nil {
		return ""
	}
	chips := []string{fmt.Sprintf("✓ Deleted %d", m.report.Deleted)}
	if m.canceled {
		chips = append([]string{"⚠ Canceled"}, chips...)
	}
	if failed := len(m.failures()); failed > 0 {
		chips = append(chips, fmt.Sprintf("✗ %d failed", failed))
	}
	chips = append(chips, formatTenths(m.elapsed))
	return strings.Join(chips, " · ")
}

func (m CleanerModel) failures() []resultLine {
	if m.report == nil {
		return nil
	}
	var failed []resultLine
	for _, entry := range m.report.Entries {
		if entry.Level == cleaner.LevelError {
			failed = append(failed, resultLine{text: m.entryLabel(entry), detail: m.failureDetail(entry)})
		}
	}
	if m.runErr != nil && !m.canceled && len(failed) == 0 {
		for _, line := range strings.Split(m.runErr.Error(), "\n") {
			failed = append(failed, resultLine{text: line})
		}
	}
	return failed
}

func (m CleanerModel) resultLines(width int) []string {
	if m.report == nil {
		return nil
	}
	var deleted, stopped, refused []resultLine
	for _, entry := range m.report.Entries {
		switch {
		case entry.Level == cleaner.LevelDelete:
			deleted = append(deleted, resultLine{text: m.entryLabel(entry)})
		case entry.Level == cleaner.LevelWarn && strings.HasPrefix(entry.Message, cleanerStoppedPrefix):
			stopped = append(stopped, resultLine{text: strings.TrimPrefix(entry.Message, cleanerStoppedPrefix)})
		case entry.Level == cleaner.LevelSkip && entry.Path != "" && !entry.NotPresent:
			refused = append(refused, resultLine{text: displayPath(m.home, entry.Path), detail: cleaner.RefusedOutsideProfile})
		}
	}

	var lines []string
	section := func(title string, style lipgloss.Style, items []resultLine) {
		if len(items) == 0 {
			return
		}
		lines = append(lines, style.Bold(true).Render(fmt.Sprintf("%s · %d", title, len(items))))
		for _, item := range items {
			text := item.text
			if item.detail != "" {
				text += "   " + item.detail
			}
			for _, wrapped := range common.WrapLine("  ", text, width) {
				lines = append(lines, item.style.Render(wrapped))
			}
		}
	}

	section("FAILED", common.ToneDanger.Style(), m.failures())
	if m.canceled {
		left := common.MaxInt(0, m.total-m.done)
		section("CANCELED", common.ToneWarning.Style(), []resultLine{{text: fmt.Sprintf("Stopped before finishing · %d of %d not done", left, m.total)}})
	}
	section("DELETED", common.ToneSuccess.Style(), deleted)
	section("STOPPED", common.ToneSuccess.Style(), stopped)
	section("REFUSED", common.ToneWarning.Style(), refused)
	if kept := m.keptNames(); kept != "" {
		lines = append(lines, common.Muted.Bold(true).Render("KEPT"))
		lines = append(lines, common.WrapLine("  ", kept+" (options off)", width)...)
	}
	lines = append(lines, "")
	return append(lines, common.WrapLine("ⓘ ", cleanerReminder, width)...)
}

func (m CleanerModel) keptNames() string {
	var names []string
	for _, row := range cleanerScopeRows {
		if row.id == cleaner.GroupCredentials || row.id == cleaner.GroupForceStop || !cleaner.GroupAvailable(row.id) || m.options.Enabled(row.id) {
			continue
		}
		names = append(names, row.name)
	}
	if len(names) == 0 {
		return ""
	}
	joined := strings.Join(names, ", ")
	return joined[:1] + strings.ToLower(joined[1:])
}

func scanCleanerCmd(plan CleanerPlanFunc, seq int, options cleaner.Options) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), cleanerScanTimeout)
		defer cancel()
		result, err := plan(ctx, options)
		return cleanerPlanMsg{seq: seq, plan: result, err: err}
	}
}

func runCleanerCmd(ctx context.Context, run CleanerRunFunc, options cleaner.Options) tea.Cmd {
	return func() tea.Msg {
		events := make(chan tea.Msg, cleanerEventBuffer)
		options.Progress = func(progress cleaner.Progress) {
			events <- cleanerProgressMsg{progress: progress, events: events}
		}
		go func() {
			report, err := run(ctx, options)
			events <- cleanerFinishedMsg{report: report, err: err, canceled: runWasCanceled(ctx, err)}
		}()
		return <-events
	}
}

func waitCleanerEvent(events <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-events
	}
}

func saveDryRunLogCmd(run CleanerRunFunc, options cleaner.Options, path string) tea.Cmd {
	options.Execute = false
	options.ForceStopProcesses = false
	options.Progress = nil
	options.LogPath = path
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), cleanerRunTimeout)
		defer cancel()
		report, err := run(ctx, options)
		if report.LogPath != "" {
			path = report.LogPath
		}
		if _, statErr := os.Stat(path); statErr == nil {
			return cleanerLogSavedMsg{path: path}
		}
		if err == nil {
			err = errors.New("the log file was not written")
		}
		return cleanerLogSavedMsg{path: path, err: err}
	}
}

func saveReportLogCmd(save CleanerSaveFunc, path string, report cleaner.Report) tea.Cmd {
	return func() tea.Msg {
		saved, err := save(path, report)
		return cleanerLogSavedMsg{path: saved, err: err}
	}
}

func rootCause(err error) string {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return err.Error()
}

func defaultCleanerLogPath(home string, now time.Time) string {
	return filepath.Join(home, cleanerLogPrefix+now.Format(cleanerLogTimeLayout)+cleanerLogSuffix)
}

func progressBar(done, total, width int) string {
	filled := 0
	if total > 0 {
		filled = common.MinInt(width, done*width/total)
	}
	return common.Accent.Render(strings.Repeat("█", filled)) + common.Muted.Render(strings.Repeat("░", width-filled))
}

func formatTenths(elapsed time.Duration) string {
	tenths := int(elapsed / (100 * time.Millisecond))
	return fmt.Sprintf("%d:%02d.%d", tenths/600, tenths/10%60, tenths%10)
}

func scrollPosition(offset, room, total int) string {
	return fmt.Sprintf("%d-%d/%d", offset+1, common.MinInt(offset+room, total), total)
}

func fileFolderPhrase(files, folders int, join string) string {
	var parts []string
	if files > 0 {
		parts = append(parts, plural(files, "file", "files"))
	}
	if folders > 0 {
		parts = append(parts, plural(folders, "folder", "folders"))
	}
	return strings.Join(parts, join)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
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

func runWasCanceled(ctx context.Context, err error) bool {
	return err != nil && errors.Is(ctx.Err(), context.Canceled)
}

func cancelingNotice(subject string, quitAfter bool) string {
	if quitAfter {
		return "Canceling " + subject + "; UTILS quits when it stops. Press ctrl+c again to quit now and leave the current step unfinished."
	}
	return "Canceling " + subject + "..."
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
