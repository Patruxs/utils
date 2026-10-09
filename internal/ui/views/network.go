package views

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	corenetwork "utils/internal/core/network"
	"utils/internal/ui/common"
)

type networkActionID int

const (
	networkActionViewConfig networkActionID = iota
	networkActionDiagnostics
	networkActionApplyConfig
	networkActionSetCloudflareDNS
	networkActionSetGoogleDNS
	networkActionSetOpenDNS
	networkActionSetQuad9DNS
	networkActionFlushDNS
	networkActionEnableDoH
	networkActionDisableDoH
	networkActionOptimize
	networkActionResetOptimizations
	networkActionResetDNS
	networkActionResetDefaults
	networkActionHostsView
	networkActionHostsBackup
	networkActionHostsAdd
	networkActionHostsRemoveCustom
	networkActionHostsRestore
	networkActionBrowserChrome
	networkActionBrowserFirefox
	networkActionBrowserEdge
	networkActionBrowserBrave
	networkActionBrowserOpera
	networkActionBrowserAll
	networkActionPersistentStatus
	networkActionTogglePersistent
	networkActionApplyPersistent
	networkActionClearPersistent
)

type networkViewState int

const (
	networkStateSelectingOptions networkViewState = iota
	networkStateEditingHostsAdd
	networkStateConfirmingWrite
	networkStateRunning
	networkStateFinished
)

type networkKeyMap struct {
	Move          key.Binding
	Toggle        key.Binding
	Run           key.Binding
	BackToMenu    key.Binding
	NextField     key.Binding
	AddEntry      key.Binding
	LeaveForm     key.Binding
	Choose        key.Binding
	Select        key.Binding
	Confirm       key.Binding
	Cancel        key.Binding
	Scroll        key.Binding
	BackToActions key.Binding
}

type networkFooterKeys []key.Binding

func (k networkFooterKeys) ShortHelp() []key.Binding {
	return k
}

func (k networkFooterKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{k}
}

type NetworkModel struct {
	spinner          spinner.Model
	keyMap           networkKeyMap
	logViewer        NetworkLogViewerModel
	manager          corenetwork.NetworkManager
	results          []networkActionResult
	err              error
	canceled         bool
	cancelNetwork    context.CancelFunc
	notice           string
	width            int
	height           int
	state            networkViewState
	action           int
	checkedActions   map[networkActionID]bool
	persistentMode   bool
	hostDomainInput  textinput.Model
	hostIPInput      textinput.Model
	focusedHostField int
	hostsError       string
	pendingRun       networkRunOptions
	confirmRun       bool
	running          []networkActionID
	runStarted       time.Time
}

type NetworkLogViewerModel struct {
	viewport     viewport.Model
	report       *corenetwork.Report
	mouseFocused bool
}

type networkFinishedMsg struct {
	results  []networkActionResult
	err      error
	canceled bool
}

type networkActionItem struct {
	id      networkActionID
	group   string
	title   string
	details []string
}

type networkActionResult struct {
	action networkActionID
	report corenetwork.Report
	err    error
}

type networkRunOptions struct {
	persistentMode bool
	hosts          corenetwork.HostsOptions
	actions        []networkActionID
}

var _ tea.Model = NetworkModel{}

const (
	networkTitle       = "Network & Diagnostics Manager"
	networkShortTitle  = "Network"
	networkRunTimeout  = 2 * time.Minute
	networkHostsIntro  = "Add a hosts entry. Blank IP defaults to 127.0.0.1."
	networkCanceledRun = "Canceled; nothing was changed."
	networkDefaultIP   = "127.0.0.1"

	defaultNetworkLogViewportHeight = 12
	networkWideMinWidth             = 100
	networkNarrowCrumbWidth         = 60
	networkListMinRows              = 3
	networkDetailMaxRows            = 4
	networkDetailTightRows          = 2
	networkSessionMinListRows       = 8
)

const (
	networkGroupInspect    = "INSPECT"
	networkGroupDNS        = "DNS & CONFIG"
	networkGroupHosts      = "HOSTS"
	networkGroupBrowser    = "BROWSER CACHES"
	networkGroupPersistent = "PERSISTENT DNS"
)

var networkActions = []networkActionItem{
	{networkActionViewConfig, networkGroupInspect, "View Current Network Config", []string{"Reads adapter, DNS, IP, MTU, DoH, hosts, and ping information without requesting elevation."}},
	{networkActionDiagnostics, networkGroupInspect, "Run Network Diagnostics", []string{"Checks connectivity, DNS resolution, and ping quality for google.com, cloudflare.com, and github.com."}},
	{networkActionApplyConfig, networkGroupDNS, "Apply Network Config (DNS, DoH, MTU)", []string{"Applies Cloudflare DNS, Windows DoH templates where supported, and MTU 1500."}},
	{networkActionSetCloudflareDNS, networkGroupDNS, "Set Cloudflare DNS (1.1.1.1)", []string{"Fast and secure preset from tool.ps1: 1.1.1.1 and 1.0.0.1."}},
	{networkActionSetGoogleDNS, networkGroupDNS, "Set Google DNS (8.8.8.8)", []string{"Reliable preset from tool.ps1: 8.8.8.8 and 8.8.4.4."}},
	{networkActionSetOpenDNS, networkGroupDNS, "Set OpenDNS (208.67.222.222)", []string{"Family-safe preset from tool.ps1: 208.67.222.222 and 208.67.220.220."}},
	{networkActionSetQuad9DNS, networkGroupDNS, "Set Quad9 DNS (9.9.9.9)", []string{"Malware-protection preset from tool.ps1: 9.9.9.9 and 149.112.112.112."}},
	{networkActionFlushDNS, networkGroupDNS, "Flush DNS Cache", []string{"Flushes OS DNS caches using platform-specific commands."}},
	{networkActionEnableDoH, networkGroupDNS, "Enable DNS over HTTPS (DoH)", []string{"Registers Windows DoH templates for Cloudflare, Google, and Quad9; warns on platforms without a generic OS DoH CLI."}},
	{networkActionDisableDoH, networkGroupDNS, "Disable DNS over HTTPS (DoH)", []string{"Removes Windows DoH server entries; warns on platforms without generic OS DoH state."}},
	{networkActionOptimize, networkGroupDNS, "Optimize Network Settings", []string{"Applies Windows TCP optimizations from tool.ps1 and best-effort MTU/TCP equivalents on macOS/Linux."}},
	{networkActionResetOptimizations, networkGroupDNS, "Reset Network Optimizations", []string{"Runs Windows TCP/Winsock reset or best-effort platform reset commands."}},
	{networkActionResetDNS, networkGroupDNS, "Reset DNS to Automatic", []string{"Resets DNS to DHCP/automatic/default resolver behavior and flushes caches where supported."}},
	{networkActionResetDefaults, networkGroupDNS, "Reset Network Settings to Defaults", []string{"Resets DNS, disables DoH where supported, and clears persistent DNS settings."}},
	{networkActionHostsView, networkGroupHosts, "Hosts: View File", []string{"Reads the hosts file without elevation."}},
	{networkActionHostsBackup, networkGroupHosts, "Hosts: Backup File", []string{"Copies hosts to a timestamped hosts.backup-<time> file."}},
	{networkActionHostsAdd, networkGroupHosts, "Hosts: Add Entry", []string{"Prompts for domain and IP, then appends IP<TAB>domain<TAB>" + corenetwork.HostsManagedMarker + " with per-command elevation."}},
	{networkActionHostsRemoveCustom, networkGroupHosts, "Hosts: Remove Managed Entries", []string{"Removes only lines tagged " + corenetwork.HostsManagedMarker + " and lists the unmanaged lines it left alone."}},
	{networkActionHostsRestore, networkGroupHosts, "Hosts: Restore Newest Backup", []string{"Saves the current hosts as hosts.before-restore-<time>, then restores the newest hosts.backup-<time>."}},
	{networkActionBrowserChrome, networkGroupBrowser, "Clear Chrome/Chromium Cache", []string{"Clears common Chrome and Chromium cache/code-cache paths for the current user."}},
	{networkActionBrowserFirefox, networkGroupBrowser, "Clear Firefox Cache", []string{"Clears Firefox profile cache2 folders for the current user."}},
	{networkActionBrowserEdge, networkGroupBrowser, "Clear Edge Cache", []string{"Clears common Microsoft Edge cache/code-cache paths for the current user."}},
	{networkActionBrowserBrave, networkGroupBrowser, "Clear Brave Cache", []string{"Clears common Brave cache/code-cache paths for the current user."}},
	{networkActionBrowserOpera, networkGroupBrowser, "Clear Opera Cache", []string{"Clears common Opera cache/code-cache paths for the current user."}},
	{networkActionBrowserAll, networkGroupBrowser, "Clear All Browser Caches", []string{"Runs all browser cache cleaners from the original script scope."}},
	{networkActionPersistentStatus, networkGroupPersistent, "Persistent DNS: View Status", []string{"Shows saved persistent DNS mode and preset values."}},
	{networkActionTogglePersistent, networkGroupPersistent, "Persistent DNS: Toggle Mode", []string{"Turns persistent DNS mode on or off for future DNS preset actions."}},
	{networkActionApplyPersistent, networkGroupPersistent, "Persistent DNS: Apply Saved Settings", []string{"Loads saved DNS preset values and applies them with per-command elevation."}},
	{networkActionClearPersistent, networkGroupPersistent, "Persistent DNS: Clear Settings", []string{"Removes saved persistent DNS settings."}},
}

func newNetworkKeyMap() networkKeyMap {
	return networkKeyMap{
		Move:          key.NewBinding(key.WithKeys("up", "down", "k", "j"), key.WithHelp("↑↓", "move")),
		Toggle:        key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
		Run:           key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "run")),
		BackToMenu:    key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "menu")),
		NextField:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field")),
		AddEntry:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "add entry")),
		LeaveForm:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Choose:        key.NewBinding(key.WithKeys("left", "right", "up", "down", "tab", "h", "l", "k", "j"), key.WithHelp("←→", "choose")),
		Select:        key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("enter", "select")),
		Confirm:       key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "confirm")),
		Cancel:        key.NewBinding(key.WithKeys("n", "esc"), key.WithHelp("n/esc", "cancel")),
		Scroll:        key.NewBinding(key.WithKeys("up", "down", "pgup", "pgdown"), key.WithHelp("↑↓", "scroll")),
		BackToActions: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "actions")),
	}
}

func (m NetworkModel) FooterKeys() help.KeyMap {
	k := m.keyMap
	switch m.state {
	case networkStateRunning:
		return networkFooterKeys{common.DefaultKeys.CancelRun, common.DefaultKeys.CancelAndQuit}
	case networkStateEditingHostsAdd:
		return networkFooterKeys{k.NextField, k.AddEntry, k.LeaveForm}
	case networkStateConfirmingWrite:
		return networkFooterKeys{k.Choose, k.Select, k.Confirm, k.Cancel}
	case networkStateFinished:
		return networkFooterKeys{k.Scroll, k.BackToActions, k.BackToMenu}
	default:
		return networkFooterKeys{k.Move, k.Toggle, k.Run, k.BackToMenu}
	}
}

func (m NetworkModel) FooterStatus() string {
	switch m.state {
	case networkStateConfirmingWrite:
		return common.Pill("WRITES", common.ToneDanger)
	case networkStateRunning:
		if runChangesSystem(m.running) {
			return common.Pill("WRITES", common.ToneWarning)
		}
		return common.Muted.Render("read-only")
	case networkStateSelectingOptions:
		return m.listPosition()
	default:
		return ""
	}
}

func (m NetworkModel) Breadcrumb() []string {
	title := networkTitle
	if m.width > 0 && m.width < networkNarrowCrumbWidth {
		title = networkShortTitle
	}
	if m.state == networkStateEditingHostsAdd {
		return []string{title, actionTitle(networkActionHostsAdd)}
	}
	return []string{title}
}

func NewNetworkModel() NetworkModel {
	return NewNetworkModelWithManager(corenetwork.NewNetworkManager(nil))
}

func NewNetworkModelWithManager(manager corenetwork.NetworkManager) NetworkModel {
	domainInput := textinput.New()
	domainInput.Placeholder = "example.local"
	domainInput.Prompt = ""
	domainInput.CharLimit = 253
	domainInput.SetValue("example.local")
	domainInput.Focus()

	ipInput := textinput.New()
	ipInput.Placeholder = networkDefaultIP
	ipInput.Prompt = ""
	ipInput.CharLimit = 64
	ipInput.SetValue(networkDefaultIP)
	ipInput.Blur()

	model := NetworkModel{
		spinner: spinner.New(
			spinner.WithSpinner(spinner.Dot),
			spinner.WithStyle(common.Accent),
		),
		keyMap:          newNetworkKeyMap(),
		logViewer:       NewNetworkLogViewerModel(),
		manager:         manager,
		checkedActions:  make(map[networkActionID]bool),
		hostDomainInput: domainInput,
		hostIPInput:     ipInput,
	}
	model.layoutComponents()
	return model
}

func NewNetworkLogViewerModel() NetworkLogViewerModel {
	logViewport := viewport.New(0, defaultNetworkLogViewportHeight)
	logViewport.KeyMap = viewport.KeyMap{
		Down:     key.NewBinding(key.WithKeys("down", "j")),
		Up:       key.NewBinding(key.WithKeys("up", "k")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "d", "ctrl+d")),
		PageUp:   key.NewBinding(key.WithKeys("pgup", "u", "ctrl+u")),
	}

	return NetworkLogViewerModel{viewport: logViewport}
}

func (m NetworkModel) Init() tea.Cmd {
	return nil
}

func (m NetworkModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if m.state == networkStateRunning {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layoutComponents()
	case networkFinishedMsg:
		if m.cancelNetwork != nil {
			m.cancelNetwork()
			m.cancelNetwork = nil
		}
		m.state = networkStateFinished
		m.results = msg.results
		m.err = msg.err
		m.canceled = msg.canceled
		m.notice = ""
		m.logViewer.SetMouseFocused(false)
		m.logViewer.SetReport(combinedNetworkReport(msg.results))
		m.layoutComponents()
		return m, nil
	case tea.KeyMsg:
		switch m.state {
		case networkStateRunning:
			if key.Matches(msg, common.DefaultKeys.CancelRun, common.DefaultKeys.CancelAndQuit) {
				return m.cancelRunningNetwork(common.IsForceQuit(msg))
			}
			return m, batch(cmds...)
		case networkStateEditingHostsAdd:
			return m.updateHostsAddForm(msg, cmds...)
		case networkStateConfirmingWrite:
			return m.updateWriteConfirmation(msg)
		case networkStateFinished:
			if m.logViewer.IsKeyScrollInput(msg) {
				var cmd tea.Cmd
				m.logViewer, cmd = m.logViewer.Update(msg)
				return m, cmd
			}
			if key.Matches(msg, m.keyMap.BackToActions) {
				m.state = networkStateSelectingOptions
			}
			return m, nil
		case networkStateSelectingOptions:
			switch {
			case key.Matches(msg, common.DefaultKeys.Up):
				m.moveActions(-1)
			case key.Matches(msg, common.DefaultKeys.Down):
				m.moveActions(1)
			case key.Matches(msg, m.keyMap.Toggle):
				m.toggleCurrentAction()
			case key.Matches(msg, m.keyMap.Run):
				return m.startSelectedAction()
			}
			return m, nil
		}
	case tea.MouseMsg:
		if m.state == networkStateFinished {
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				m.logViewer.SetMouseFocused(m.mouseInLogViewer(msg))
				return m, nil
			}
			if m.logViewer.IsFocusedMouseScrollInput(msg) {
				var cmd tea.Cmd
				m.logViewer, cmd = m.logViewer.Update(msg)
				return m, cmd
			}
		}
		return m, nil
	}

	return m, batch(cmds...)
}

func (m NetworkModel) bodySize() (int, int) {
	width, height := m.width, m.height
	if width <= 0 {
		width = common.DefaultContentWidth - 2*common.MarginX
	}
	if height <= 0 {
		height = common.DefaultContentHeight - common.HeaderHeight(common.DefaultContentHeight) - 1
	}
	return width, height
}

func (m NetworkModel) View() string {
	width, height := m.bodySize()
	notice := ""
	if m.notice != "" {
		notice = common.Notice(width, common.ToneWarning, m.notice)
		height = common.MaxInt(1, height-lipgloss.Height(notice))
	}

	var body string
	switch m.state {
	case networkStateRunning:
		body = m.runningView(width)
	case networkStateEditingHostsAdd:
		body = m.hostsFormView(width)
	case networkStateConfirmingWrite:
		body = m.confirmView(width)
	case networkStateFinished:
		body = m.resultView(width, height)
	default:
		body = m.actionsView(width, height)
	}

	if notice == "" {
		return body
	}
	return common.FitHeight(body, height) + "\n" + notice
}

func (m *NetworkModel) layoutComponents() {
	width, height := m.bodySize()
	inputWidth := common.MaxInt(10, width-len("│ ▸ domain  ")-4)
	m.hostDomainInput.Width = inputWidth
	m.hostIPInput.Width = inputWidth
	_, logHeight := m.resultLayout(width, height)
	m.logViewer.SetSize(common.MaxInt(1, width-4), logHeight)
}

func (m NetworkModel) actionsView(width, height int) string {
	if width >= networkWideMinWidth {
		return m.wideActionsView(width, height)
	}

	detailRows := networkDetailMaxRows
	sessionRows := 1
	listRows := func() int { return height - (detailRows + 2) - sessionRows - 2 }
	if listRows() < networkSessionMinListRows {
		sessionRows = 0
	}
	if listRows() < networkSessionMinListRows {
		detailRows = networkDetailTightRows
	}

	rows := []string{m.actionsPanel(width, common.MaxInt(networkListMinRows, listRows()))}
	detail := m.actionDetailLines(width - 4)
	rows = append(rows, common.Panel{Title: m.currentAction().title, Width: width}.Render(strings.Join(firstLines(detail, detailRows), "\n")))
	if sessionRows > 0 {
		rows = append(rows, common.Truncate(common.Muted.Render(m.persistentModeLine()), width))
	}
	return strings.Join(rows, "\n")
}

func (m NetworkModel) wideActionsView(width, height int) string {
	leftWidth := width * 45 / 100
	rightWidth := width - leftWidth - 1

	detail := common.Panel{Title: m.currentAction().title, Width: rightWidth}.Render(strings.Join(m.actionDetailLines(rightWidth-4), "\n"))
	session := common.Panel{Title: "Session", Width: rightWidth}.Render(strings.Join(m.sessionLines(rightWidth-4), "\n"))
	right := detail
	if lipgloss.Height(detail)+lipgloss.Height(session) <= height {
		right += "\n" + session
	}

	left := m.actionsPanel(leftWidth, common.MaxInt(networkListMinRows, height-2))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (m NetworkModel) actionsPanel(width, rows int) string {
	panel := common.Panel{Title: "Actions", Meta: m.listPosition(), Variant: common.PanelFocused, Width: width}
	return panel.Render(m.renderActions(rows))
}

func (m NetworkModel) listPosition() string {
	return fmt.Sprintf("%d/%d · %d checked", m.action+1, len(networkActions), len(m.selectedActionIDs()))
}

func (m NetworkModel) actionDetailLines(width int) []string {
	action := m.currentAction()
	lines := []string{common.Muted.Render("read-only")}
	if !isReadOnlyNetworkAction(action.id) {
		lines = []string{common.Warning.Render("● writes system settings")}
	}
	for _, detail := range action.details {
		lines = append(lines, common.WrapPlain(detail, width)...)
	}
	if !isReadOnlyNetworkAction(action.id) {
		change := m.networkActionChange(action.id, corenetwork.HostsOptions{IP: "<ip>", Domain: "<domain>"})
		lines = append(lines, common.WrapLine("Changes: ", change, width)...)
	}
	return lines
}

func (m NetworkModel) sessionLines(width int) []string {
	lines := []string{m.persistentModeLine()}
	for _, line := range common.WrapPlain(corenetwork.ElevationNote(), width) {
		lines = append(lines, common.Muted.Render(line))
	}
	return lines
}

func (m NetworkModel) persistentModeLine() string {
	if m.persistentMode {
		return "Persistent DNS mode: on"
	}
	return "Persistent DNS mode: off"
}

func firstLines(lines []string, count int) []string {
	if len(lines) <= count {
		return lines
	}
	return lines[:count]
}

func (m NetworkModel) runningView(width int) string {
	label := "Running: " + actionTitle(m.currentAction().id)
	if len(m.running) == 1 {
		label = "Running: " + actionTitle(m.running[0])
	} else if len(m.running) > 1 {
		label = fmt.Sprintf("Running %d actions", len(m.running))
	}
	note := "Read-only"
	if runChangesSystem(m.running) {
		note = "Writes system settings"
	}
	return common.RunStatus(width, m.spinner.View(), label, note, time.Since(m.runStarted))
}

func (m NetworkModel) hostsFormView(width int) string {
	panel := common.Panel{Title: actionTitle(networkActionHostsAdd), Variant: common.PanelFocused, Width: width}
	lines := []string{
		common.Muted.Render(networkHostsIntro),
		"",
		hostsFieldRow(m.focusedHostField == 0, "domain", m.hostDomainInput.View()),
		hostsFieldRow(m.focusedHostField == 1, "ip", m.hostIPInput.View()),
	}
	if m.hostsError != "" {
		lines = append(lines, "")
		lines = append(lines, common.WrapStyled(common.Error.Render(common.ToneDanger.Glyph()+" "+m.hostsError), panel.InnerWidth())...)
	}
	return panel.Render(strings.Join(lines, "\n"))
}

func hostsFieldRow(focused bool, label, input string) string {
	cursor := "  "
	label = fmt.Sprintf("%-6s", label)
	if focused {
		cursor = common.Accent.Render(common.ToneAccent.Glyph()) + " "
		label = lipgloss.NewStyle().Bold(true).Render(label)
	}
	return cursor + label + "  " + input
}

func (m NetworkModel) confirmView(width int) string {
	lines := []string{"These actions change your system:"}
	for _, change := range m.pendingChanges() {
		lines = append(lines, "• "+change)
	}
	lines = append(lines, common.Muted.Render(corenetwork.ElevationNote()))
	selected := 0
	if m.confirmRun {
		selected = 1
	}
	return common.ConfirmDialog(width, "Change system settings", lines, []string{"Cancel", "Run these changes"}, selected)
}

func (m NetworkModel) resultSummary(width int) string {
	operation := m.resultOperation()
	summary := common.Success.Render(common.ToneSuccess.Glyph() + " " + operation + " finished")
	switch {
	case m.canceled:
		summary = common.Warning.Render("⚠ Canceled: " + operation + " stopped early; the activity shows what ran")
	case m.err != nil:
		summary = common.Error.Render(common.ToneDanger.Glyph() + " Completed with errors: " + operation)
	}

	warnings, errorCount := 0, 0
	for _, result := range m.results {
		warnings += result.report.Warnings
		errorCount += result.report.Errors
	}
	counts := common.RenderCounts(width, []common.Count{
		{Label: "warnings", N: warnings, Tone: common.ToneWarning},
		{Label: "errors", N: errorCount, Tone: common.ToneDanger},
	})
	if lipgloss.Width(summary)+3+lipgloss.Width(counts) <= width {
		return summary + common.Muted.Render(" · ") + counts
	}
	return strings.Join(append(common.WrapStyled(summary, width), counts), "\n")
}

func (m NetworkModel) resultOperation() string {
	if len(m.results) == 1 {
		return actionTitle(m.results[0].action)
	}
	return fmt.Sprintf("%d actions", len(m.results))
}

func (m NetworkModel) resultLayout(width, height int) (int, int) {
	top := lipgloss.Height(m.resultSummary(width))
	return top, common.MaxInt(1, height-top-2)
}

func (m NetworkModel) resultView(width, height int) string {
	top, logHeight := m.resultLayout(width, height)
	_ = top
	panel := common.Panel{Title: "Activity", Width: width, Height: logHeight + 2}
	if m.logViewer.mouseFocused {
		panel.Variant = common.PanelFocused
	}
	return m.resultSummary(width) + "\n" + panel.Render(m.logViewer.viewport.View())
}

func (m NetworkModel) mouseInLogViewer(msg tea.MouseMsg) bool {
	width, height := m.bodySize()
	top, logHeight := m.resultLayout(width, height)
	return msg.Y > top && msg.Y <= top+logHeight
}

func (m *NetworkModel) moveActions(delta int) {
	m.action = wrapIndex(m.action+delta, len(networkActions))
	m.notice = ""
}

func (m *NetworkModel) toggleCurrentAction() {
	action := m.currentAction().id
	if m.checkedActions == nil {
		m.checkedActions = make(map[networkActionID]bool)
	}
	if m.checkedActions[action] {
		delete(m.checkedActions, action)
		return
	}
	m.checkedActions[action] = true
}

func (m NetworkModel) startSelectedAction() (tea.Model, tea.Cmd) {
	actions := m.selectedOrCurrentActionIDs()
	if actionIDsContain(actions, networkActionHostsAdd) {
		m.state = networkStateEditingHostsAdd
		m.notice = ""
		m.hostsError = ""
		m.focusHostField(0)
		return m, nil
	}

	return m.requestRun(networkRunOptions{actions: actions})
}

func (m NetworkModel) requestRun(options networkRunOptions) (tea.Model, tea.Cmd) {
	if runChangesSystem(options.actions) {
		m.state = networkStateConfirmingWrite
		m.pendingRun = options
		m.confirmRun = false
		m.notice = ""
		return m, nil
	}
	return m.startRun(options)
}

func runChangesSystem(actions []networkActionID) bool {
	for _, action := range actions {
		if !isReadOnlyNetworkAction(action) {
			return true
		}
	}
	return false
}

func (m NetworkModel) updateWriteConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keyMap.Confirm):
		return m.startRun(m.pendingRun)
	case key.Matches(msg, m.keyMap.Cancel):
		return m.cancelWriteConfirmation()
	case key.Matches(msg, m.keyMap.Choose):
		m.confirmRun = !m.confirmRun
	case key.Matches(msg, m.keyMap.Select):
		if m.confirmRun {
			return m.startRun(m.pendingRun)
		}
		return m.cancelWriteConfirmation()
	}
	return m, nil
}

func (m NetworkModel) cancelWriteConfirmation() (tea.Model, tea.Cmd) {
	m.state = networkStateSelectingOptions
	m.pendingRun = networkRunOptions{}
	m.notice = networkCanceledRun
	return m, nil
}

func (m NetworkModel) pendingChanges() []string {
	changes := make([]string, 0, len(m.pendingRun.actions))
	for _, action := range m.pendingRun.actions {
		if isReadOnlyNetworkAction(action) {
			continue
		}
		changes = append(changes, actionTitle(action)+": "+m.networkActionChange(action, m.pendingRun.hosts))
	}
	return changes
}

func isReadOnlyNetworkAction(action networkActionID) bool {
	switch action {
	case networkActionViewConfig, networkActionDiagnostics, networkActionHostsView, networkActionPersistentStatus:
		return true
	default:
		return false
	}
}

func (m NetworkModel) networkActionChange(action networkActionID, hosts corenetwork.HostsOptions) string {
	hostsPath := corenetwork.HostsPath()
	switch action {
	case networkActionApplyConfig:
		preset := corenetwork.DefaultConfigOptions()
		return fmt.Sprintf("replaces the DNS servers of every active connection with %s and %s, registers Windows DoH templates, and sets MTU %d on every active interface.", preset.DNSPrimary, preset.DNSSecondary, preset.MTU)
	case networkActionSetCloudflareDNS:
		return dnsPresetChange(corenetwork.CloudflareDNSOptions())
	case networkActionSetGoogleDNS:
		return dnsPresetChange(corenetwork.GoogleDNSOptions())
	case networkActionSetOpenDNS:
		return dnsPresetChange(corenetwork.OpenDNSOptions())
	case networkActionSetQuad9DNS:
		return dnsPresetChange(corenetwork.Quad9DNSOptions())
	case networkActionFlushDNS:
		return "flushes the operating system DNS cache."
	case networkActionEnableDoH:
		return "registers DoH templates for Cloudflare, Google and Quad9 (Windows only)."
	case networkActionDisableDoH:
		return "removes every registered DoH template, including Windows built-in ones (Windows only)."
	case networkActionOptimize:
		return "changes TCP settings and sets MTU 1500 on every active interface."
	case networkActionResetOptimizations:
		return "Windows: runs netsh int tcp reset and netsh winsock reset (restart needed). macOS/Linux: resets TCP sysctls and MTU on every active interface."
	case networkActionResetDNS:
		return "clears manually set DNS servers on every active connection and returns to automatic DNS."
	case networkActionResetDefaults:
		return "resets DNS to automatic, removes Windows DoH templates, and deletes the saved persistent DNS settings."
	case networkActionHostsBackup:
		return fmt.Sprintf("copies %s to %s.backup-<time>.", hostsPath, hostsPath)
	case networkActionHostsAdd:
		return fmt.Sprintf("appends \"%s\t%s\t%s\" to %s.", hosts.IP, hosts.Domain, corenetwork.HostsManagedMarker, hostsPath)
	case networkActionHostsRemoveCustom:
		return fmt.Sprintf("removes lines tagged %s from %s; all other lines stay.", corenetwork.HostsManagedMarker, hostsPath)
	case networkActionHostsRestore:
		return fmt.Sprintf("saves %s as %s.before-restore-<time>, then overwrites it with the newest %s.backup-<time>.", hostsPath, hostsPath, hostsPath)
	case networkActionBrowserChrome:
		return browserCacheChange("Chrome and Chromium")
	case networkActionBrowserFirefox:
		return browserCacheChange("Firefox")
	case networkActionBrowserEdge:
		return browserCacheChange("Edge")
	case networkActionBrowserBrave:
		return browserCacheChange("Brave")
	case networkActionBrowserOpera:
		return browserCacheChange("Opera")
	case networkActionBrowserAll:
		return browserCacheChange("Chrome, Chromium, Firefox, Edge, Brave and Opera")
	case networkActionTogglePersistent:
		if m.persistentMode {
			return "turns persistent DNS mode off and deletes the saved DNS settings."
		}
		preset := corenetwork.DefaultConfigOptions()
		return fmt.Sprintf("turns persistent DNS mode on and saves %s (%s, %s) as the persistent DNS.", preset.DNSName, preset.DNSPrimary, preset.DNSSecondary)
	case networkActionApplyPersistent:
		return "replaces the DNS servers of every active connection with the saved persistent DNS."
	case networkActionClearPersistent:
		return "deletes the saved persistent DNS settings."
	default:
		return "changes system settings."
	}
}

func dnsPresetChange(preset corenetwork.ConfigOptions) string {
	return fmt.Sprintf("replaces the DNS servers of every active connection with %s and %s.", preset.DNSPrimary, preset.DNSSecondary)
}

func browserCacheChange(browsers string) string {
	return "deletes the " + browsers + " cache folders of the current user."
}

func (m NetworkModel) updateHostsAddForm(msg tea.KeyMsg, cmds ...tea.Cmd) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keyMap.LeaveForm):
		m.state = networkStateSelectingOptions
		m.notice = ""
		m.hostsError = ""
		return m, nil
	case key.Matches(msg, m.keyMap.NextField):
		m.focusHostField((m.focusedHostField + 1) % 2)
		return m, nil
	case key.Matches(msg, common.DefaultKeys.Up):
		m.focusHostField(wrapIndex(m.focusedHostField-1, 2))
		return m, nil
	case key.Matches(msg, common.DefaultKeys.Down):
		m.focusHostField(wrapIndex(m.focusedHostField+1, 2))
		return m, nil
	case key.Matches(msg, m.keyMap.AddEntry):
		domain := strings.TrimSpace(m.hostDomainInput.Value())
		if domain == "" {
			m.hostsError = "Enter a domain before adding a hosts entry."
			return m, nil
		}
		ip := strings.TrimSpace(m.hostIPInput.Value())
		if ip == "" {
			ip = networkDefaultIP
		}
		if err := corenetwork.ValidateHostsEntry(ip, domain); err != nil {
			m.hostsError = err.Error()
			return m, nil
		}
		m.hostsError = ""
		return m.requestRun(networkRunOptions{
			actions: m.selectedOrCurrentActionIDs(),
			hosts: corenetwork.HostsOptions{
				Mode:   corenetwork.HostsAdd,
				Domain: domain,
				IP:     ip,
			},
		})
	}

	var cmd tea.Cmd
	if m.focusedHostField == 0 {
		m.hostDomainInput, cmd = m.hostDomainInput.Update(msg)
	} else {
		m.hostIPInput, cmd = m.hostIPInput.Update(msg)
	}
	cmds = append(cmds, cmd)
	return m, batch(cmds...)
}

func (m *NetworkModel) focusHostField(index int) {
	m.focusedHostField = index
	if index == 0 {
		m.hostDomainInput.Focus()
		m.hostIPInput.Blur()
		return
	}
	m.hostDomainInput.Blur()
	m.hostIPInput.Focus()
}

func (m NetworkModel) startRun(options networkRunOptions) (tea.Model, tea.Cmd) {
	m.state = networkStateRunning
	m.results = nil
	m.err = nil
	m.canceled = false
	m.notice = ""

	actions := options.actions
	if len(actions) == 0 {
		actions = []networkActionID{m.currentAction().id}
	}
	if actionIDsContain(actions, networkActionTogglePersistent) {
		m.persistentMode = !m.persistentMode
	}
	if actionIDsContain(actions, networkActionClearPersistent) || actionIDsContain(actions, networkActionResetDefaults) {
		m.persistentMode = false
	}
	options.persistentMode = m.persistentMode
	options.actions = actions
	m.pendingRun = networkRunOptions{}
	m.checkedActions = make(map[networkActionID]bool)
	m.running = actions
	m.runStarted = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), networkRunTimeout)
	m.cancelNetwork = cancel
	return m, batch(m.spinner.Tick, runNetwork(ctx, m.manager, options))
}

func (m NetworkModel) cancelRunningNetwork(quitAfter bool) (tea.Model, tea.Cmd) {
	if m.cancelNetwork != nil {
		m.cancelNetwork()
		m.cancelNetwork = nil
	}
	m.notice = cancelingNotice("network operation", quitAfter)
	return m, nil
}

func (m NetworkModel) Running() bool {
	return m.state == networkStateRunning
}

func (m NetworkModel) OwnsKeys() bool {
	return m.Running() || m.state == networkStateEditingHostsAdd || m.state == networkStateConfirmingWrite
}

func (m NetworkModel) renderActions(rows int) string {
	var b strings.Builder
	start, end := m.visibleActionRange(rows)
	if start > 0 {
		fmt.Fprintf(&b, "%s\n", common.Muted.Render(fmt.Sprintf("↑ %d more", start)))
	}
	for index := start; index < end; index++ {
		action := networkActions[index]
		cursor := "  "
		label := action.title
		if m.action == index {
			cursor = common.Accent.Render("▸") + " "
			label = lipgloss.NewStyle().Bold(true).Render(label)
		}
		marker := "[ ]"
		if m.checkedActions[action.id] {
			marker = common.Success.Render("[x]")
		}
		fmt.Fprintf(&b, "%s%s %s\n", cursor, marker, label)
	}
	if end < len(networkActions) {
		b.WriteString(common.Muted.Render(fmt.Sprintf("↓ %d more", len(networkActions)-end)))
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m NetworkModel) visibleActionRange(rows int) (int, int) {
	rows = common.MinInt(common.MaxInt(1, rows-2), len(networkActions))
	start := common.MaxInt(0, m.action-rows/2)
	end := start + rows
	if end > len(networkActions) {
		end = len(networkActions)
		start = common.MaxInt(0, end-rows)
	}
	return start, end
}

func (m NetworkModel) currentAction() networkActionItem {
	index := m.action
	if index < 0 || index >= len(networkActions) {
		return networkActions[0]
	}
	return networkActions[index]
}

func (m NetworkModel) selectedActionIDs() []networkActionID {
	if len(m.checkedActions) == 0 {
		return nil
	}

	actions := make([]networkActionID, 0, len(m.checkedActions))
	for _, action := range networkActions {
		if m.checkedActions[action.id] {
			actions = append(actions, action.id)
		}
	}
	return actions
}

func (m NetworkModel) selectedOrCurrentActionIDs() []networkActionID {
	actions := m.selectedActionIDs()
	if len(actions) > 0 {
		return actions
	}
	return []networkActionID{m.currentAction().id}
}

func (m *NetworkLogViewerModel) SetSize(width, height int) {
	if width == m.viewport.Width && height == m.viewport.Height {
		return
	}
	m.viewport.Width = width
	m.viewport.Height = height
	m.refreshContent()
}

func (m *NetworkLogViewerModel) SetReport(report corenetwork.Report) {
	m.report = &report
	m.refreshContent()
	m.viewport.GotoTop()
}

func (m *NetworkLogViewerModel) SetMouseFocused(focused bool) {
	m.mouseFocused = focused
}

func (m *NetworkLogViewerModel) refreshContent() {
	if m.report == nil {
		return
	}
	m.viewport.SetContent(renderNetworkActivity(*m.report, m.viewport.Width))
}

func (m NetworkLogViewerModel) Update(msg tea.Msg) (NetworkLogViewerModel, tea.Cmd) {
	if m.report == nil {
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m NetworkLogViewerModel) IsKeyScrollInput(msg tea.KeyMsg) bool {
	if m.report == nil {
		return false
	}
	return key.Matches(msg, m.viewport.KeyMap.Up, m.viewport.KeyMap.PageUp, m.viewport.KeyMap.Down, m.viewport.KeyMap.PageDown)
}

func (m NetworkLogViewerModel) IsFocusedMouseScrollInput(msg tea.MouseMsg) bool {
	if m.report == nil || !m.mouseFocused || msg.Action != tea.MouseActionPress {
		return false
	}
	return msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown
}

func runNetwork(ctx context.Context, manager corenetwork.NetworkManager, options networkRunOptions) tea.Cmd {
	return func() tea.Msg {
		results, err := runNetworkActions(ctx, manager, options)

		return networkFinishedMsg{
			results:  results,
			err:      err,
			canceled: runWasCanceled(ctx, err),
		}
	}
}

func runNetworkActions(ctx context.Context, manager corenetwork.NetworkManager, options networkRunOptions) ([]networkActionResult, error) {
	actions := options.actions
	if len(actions) == 0 {
		actions = []networkActionID{networkActionViewConfig}
	}

	results := make([]networkActionResult, 0, len(actions))
	var runErrors []error
	for _, action := range actions {
		report, err := runNetworkAction(ctx, manager, action, options)
		results = append(results, networkActionResult{action: action, report: report, err: err})
		if err != nil {
			runErrors = append(runErrors, err)
		}
		if ctx.Err() != nil {
			break
		}
	}
	return results, errors.Join(runErrors...)
}

func combinedNetworkReport(results []networkActionResult) corenetwork.Report {
	var combined corenetwork.Report
	for index, result := range results {
		if len(results) > 1 {
			combined.Entries = append(combined.Entries, corenetwork.Entry{Level: corenetwork.LevelInfo, Message: fmt.Sprintf("%d/%d %s", index+1, len(results), actionTitle(result.action))})
		}
		combined.Entries = append(combined.Entries, result.report.Entries...)
	}
	return combined
}

func runNetworkAction(ctx context.Context, manager corenetwork.NetworkManager, action networkActionID, options networkRunOptions) (corenetwork.Report, error) {
	withPersistence := func(opts corenetwork.ConfigOptions) corenetwork.ConfigOptions {
		opts.Persistent = options.persistentMode
		return opts
	}

	switch action {
	case networkActionDiagnostics:
		return manager.Diagnostics(ctx)
	case networkActionApplyConfig:
		return manager.ApplyConfig(ctx, withPersistence(corenetwork.DefaultConfigOptions()))
	case networkActionSetCloudflareDNS:
		return manager.SetDNS(ctx, withPersistence(corenetwork.CloudflareDNSOptions()))
	case networkActionSetGoogleDNS:
		return manager.SetDNS(ctx, withPersistence(corenetwork.GoogleDNSOptions()))
	case networkActionSetOpenDNS:
		return manager.SetDNS(ctx, withPersistence(corenetwork.OpenDNSOptions()))
	case networkActionSetQuad9DNS:
		return manager.SetDNS(ctx, withPersistence(corenetwork.Quad9DNSOptions()))
	case networkActionFlushDNS:
		return manager.FlushDNSCache(ctx)
	case networkActionEnableDoH:
		return manager.EnableDoH(ctx)
	case networkActionDisableDoH:
		return manager.DisableDoH(ctx)
	case networkActionOptimize:
		return manager.OptimizeNetworkSettings(ctx)
	case networkActionResetOptimizations:
		return manager.ResetNetworkOptimizations(ctx)
	case networkActionResetDNS:
		return manager.ResetDNS(ctx)
	case networkActionResetDefaults:
		return manager.ResetToDefaults(ctx)
	case networkActionHostsView:
		return manager.EditHosts(ctx, corenetwork.HostsOptions{Mode: corenetwork.HostsView})
	case networkActionHostsAdd:
		return manager.EditHosts(ctx, options.hosts)
	case networkActionHostsRemoveCustom:
		return manager.EditHosts(ctx, corenetwork.HostsOptions{Mode: corenetwork.HostsRemoveCustom})
	case networkActionHostsBackup:
		return manager.EditHosts(ctx, corenetwork.HostsOptions{Mode: corenetwork.HostsBackup})
	case networkActionHostsRestore:
		return manager.EditHosts(ctx, corenetwork.HostsOptions{Mode: corenetwork.HostsRestore})
	case networkActionBrowserChrome:
		return manager.ClearBrowserCache(ctx, corenetwork.BrowserChrome)
	case networkActionBrowserFirefox:
		return manager.ClearBrowserCache(ctx, corenetwork.BrowserFirefox)
	case networkActionBrowserEdge:
		return manager.ClearBrowserCache(ctx, corenetwork.BrowserEdge)
	case networkActionBrowserBrave:
		return manager.ClearBrowserCache(ctx, corenetwork.BrowserBrave)
	case networkActionBrowserOpera:
		return manager.ClearBrowserCache(ctx, corenetwork.BrowserOpera)
	case networkActionBrowserAll:
		return manager.ClearBrowserCache(ctx, corenetwork.BrowserAll)
	case networkActionPersistentStatus:
		return manager.PersistentStatus(ctx)
	case networkActionTogglePersistent:
		return manager.SetPersistentMode(ctx, options.persistentMode, corenetwork.DefaultConfigOptions())
	case networkActionApplyPersistent:
		return manager.ApplyPersistentSettings(ctx)
	case networkActionClearPersistent:
		return manager.ClearPersistentSettings(ctx)
	case networkActionViewConfig:
		fallthrough
	default:
		return manager.CurrentConfig(ctx)
	}
}

func actionIDsContain(actions []networkActionID, target networkActionID) bool {
	for _, action := range actions {
		if action == target {
			return true
		}
	}
	return false
}

func actionTitle(id networkActionID) string {
	for _, action := range networkActions {
		if action.id == id {
			return action.title
		}
	}
	return "Network action"
}

func renderNetworkActivity(report corenetwork.Report, width int) string {
	if len(report.Entries) == 0 {
		return common.Muted.Render("No activity")
	}

	var b strings.Builder
	for _, entry := range report.Entries {
		for _, line := range common.WrapLine(fmt.Sprintf("[%s] ", entry.Level), entry.Message, width) {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
