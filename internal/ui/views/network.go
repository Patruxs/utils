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
	networkActionHostsAdd
	networkActionHostsRemoveCustom
	networkActionHostsBackup
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
	networkStateRunning
	networkStateFinished
)

type networkKeyMap struct {
	Move       key.Binding
	Select     key.Binding
	Toggle     key.Binding
	NextField  key.Binding
	CancelRun  key.Binding
	BackToMenu key.Binding
}

type networkContextualKeyMap struct {
	networkKeyMap
	state networkViewState
}

type NetworkModel struct {
	spinner          spinner.Model
	help             help.Model
	keyMap           networkKeyMap
	logViewer        NetworkLogViewerModel
	manager          corenetwork.NetworkManager
	report           *corenetwork.Report
	err              error
	cancelNetwork    context.CancelFunc
	notice           string
	layout           common.Layout
	state            networkViewState
	action           int
	checkedActions   map[networkActionID]bool
	persistentMode   bool
	hostDomainInput  textinput.Model
	hostIPInput      textinput.Model
	focusedHostField int
}

type NetworkLogViewerModel struct {
	viewport     viewport.Model
	report       *corenetwork.Report
	mouseFocused bool
}

type networkFinishedMsg struct {
	report corenetwork.Report
	err    error
}

type networkActionItem struct {
	id      networkActionID
	title   string
	details []string
}

type networkRunOptions struct {
	persistentMode bool
	hosts          corenetwork.HostsOptions
	actions        []networkActionID
}

var _ tea.Model = NetworkModel{}

const (
	networkTitle        = "Network & Diagnostics Manager"
	networkSubtitle     = "Cross-platform network inspection, diagnostics, cache clearing, and per-command elevated configuration."
	networkBaselineNote = "The TUI stays in standard-user mode. DNS, DoH, MTU, reset, and hosts writes request Administrator/root only for that command, then warn and retry standard-user fallback if elevation is denied."
	networkRunTimeout   = 2 * time.Minute

	defaultNetworkLogViewportHeight = 12
	networkActionsReservedHeight    = 11
	networkActionsMinHeight         = 6
	networkLogReservedHeight        = 18
	networkLogMinHeight             = 5
	networkLogMaxHeight             = 20
)

var networkActions = []networkActionItem{
	{networkActionViewConfig, "View Current Network Config", []string{"Reads adapter, DNS, IP, MTU, DoH, hosts, and ping information without requesting elevation."}},
	{networkActionDiagnostics, "Run Network Diagnostics", []string{"Checks connectivity, DNS resolution, and ping quality for google.com, cloudflare.com, and github.com."}},
	{networkActionApplyConfig, "Apply Network Config (DNS, DoH, MTU)", []string{"Applies Cloudflare DNS, Windows DoH templates where supported, and MTU 1500."}},
	{networkActionSetCloudflareDNS, "Set Cloudflare DNS (1.1.1.1)", []string{"Fast and secure preset from tool.ps1: 1.1.1.1 and 1.0.0.1."}},
	{networkActionSetGoogleDNS, "Set Google DNS (8.8.8.8)", []string{"Reliable preset from tool.ps1: 8.8.8.8 and 8.8.4.4."}},
	{networkActionSetOpenDNS, "Set OpenDNS (208.67.222.222)", []string{"Family-safe preset from tool.ps1: 208.67.222.222 and 208.67.220.220."}},
	{networkActionSetQuad9DNS, "Set Quad9 DNS (9.9.9.9)", []string{"Malware-protection preset from tool.ps1: 9.9.9.9 and 149.112.112.112."}},
	{networkActionFlushDNS, "Flush DNS Cache", []string{"Flushes OS DNS caches using platform-specific commands."}},
	{networkActionEnableDoH, "Enable DNS over HTTPS (DoH)", []string{"Registers Windows DoH templates for Cloudflare, Google, and Quad9; warns on platforms without a generic OS DoH CLI."}},
	{networkActionDisableDoH, "Disable DNS over HTTPS (DoH)", []string{"Removes Windows DoH server entries; warns on platforms without generic OS DoH state."}},
	{networkActionOptimize, "Optimize Network Settings", []string{"Applies Windows TCP optimizations from tool.ps1 and best-effort MTU/TCP equivalents on macOS/Linux."}},
	{networkActionResetOptimizations, "Reset Network Optimizations", []string{"Runs Windows TCP/Winsock reset or best-effort platform reset commands."}},
	{networkActionResetDNS, "Reset DNS to Automatic", []string{"Resets DNS to DHCP/automatic/default resolver behavior and flushes caches where supported."}},
	{networkActionResetDefaults, "Reset Network Settings to Defaults", []string{"Resets DNS, disables DoH where supported, and clears persistent DNS settings."}},
	{networkActionHostsView, "Hosts: View File", []string{"Reads the hosts file without elevation."}},
	{networkActionHostsAdd, "Hosts: Add Entry", []string{"Prompts for domain and IP, then appends IP<TAB>domain with per-command elevation."}},
	{networkActionHostsRemoveCustom, "Hosts: Remove Custom Entries", []string{"Preserves comments, localhost entries, and blank lines, matching tool.ps1."}},
	{networkActionHostsBackup, "Hosts: Backup File", []string{"Copies hosts to hosts.backup."}},
	{networkActionHostsRestore, "Hosts: Restore Backup", []string{"Restores hosts.backup over the active hosts file."}},
	{networkActionBrowserChrome, "Clear Chrome/Chromium Cache", []string{"Clears common Chrome and Chromium cache/code-cache paths for the current user."}},
	{networkActionBrowserFirefox, "Clear Firefox Cache", []string{"Clears Firefox profile cache2 folders for the current user."}},
	{networkActionBrowserEdge, "Clear Edge Cache", []string{"Clears common Microsoft Edge cache/code-cache paths for the current user."}},
	{networkActionBrowserBrave, "Clear Brave Cache", []string{"Clears common Brave cache/code-cache paths for the current user."}},
	{networkActionBrowserOpera, "Clear Opera Cache", []string{"Clears common Opera cache/code-cache paths for the current user."}},
	{networkActionBrowserAll, "Clear All Browser Caches", []string{"Runs all browser cache cleaners from the original script scope."}},
	{networkActionPersistentStatus, "Persistent DNS: View Status", []string{"Shows saved persistent DNS mode and preset values."}},
	{networkActionTogglePersistent, "Persistent DNS: Toggle Mode", []string{"Turns persistent DNS mode on or off for future DNS preset actions."}},
	{networkActionApplyPersistent, "Persistent DNS: Apply Saved Settings", []string{"Loads saved DNS preset values and applies them with per-command elevation."}},
	{networkActionClearPersistent, "Persistent DNS: Clear Settings", []string{"Removes saved persistent DNS settings."}},
}

func newNetworkKeyMap() networkKeyMap {
	return networkKeyMap{
		Move: key.NewBinding(
			key.WithKeys("up", "down", "k", "j"),
			key.WithHelp("up/down:", "choose"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter:", "run checked"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space:", "toggle"),
		),
		NextField: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab:", "next field"),
		),
		CancelRun: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c:", "cancel"),
		),
		BackToMenu: key.NewBinding(
			key.WithKeys("q", "esc"),
			key.WithHelp("q/esc", "main menu"),
		),
	}
}

func (k networkKeyMap) contextual(state networkViewState) networkContextualKeyMap {
	return networkContextualKeyMap{
		networkKeyMap: k,
		state:         state,
	}
}

func (k networkContextualKeyMap) ShortHelp() []key.Binding {
	switch k.state {
	case networkStateRunning:
		return []key.Binding{k.CancelRun}
	case networkStateEditingHostsAdd:
		return []key.Binding{k.NextField, k.Select, k.BackToMenu}
	default:
		return []key.Binding{k.Move, k.Toggle, k.Select, k.BackToMenu}
	}
}

func (k networkContextualKeyMap) FullHelp() [][]key.Binding {
	switch k.state {
	case networkStateRunning:
		return [][]key.Binding{{k.CancelRun}}
	case networkStateEditingHostsAdd:
		return [][]key.Binding{{k.NextField, k.Select, k.BackToMenu}}
	default:
		return [][]key.Binding{{k.Move, k.Toggle, k.Select, k.BackToMenu}}
	}
}

func NewNetworkModel() NetworkModel {
	domainInput := textinput.New()
	domainInput.Placeholder = "example.local"
	domainInput.Prompt = "domain: "
	domainInput.CharLimit = 253
	domainInput.Width = 40
	domainInput.SetValue("example.local")
	domainInput.Focus()

	ipInput := textinput.New()
	ipInput.Placeholder = "127.0.0.1"
	ipInput.Prompt = "ip:     "
	ipInput.CharLimit = 64
	ipInput.Width = 40
	ipInput.SetValue("127.0.0.1")
	ipInput.Blur()

	return NetworkModel{
		spinner: spinner.New(
			spinner.WithSpinner(spinner.Dot),
			spinner.WithStyle(common.Accent),
		),
		help:            common.NewHelpModel(),
		keyMap:          newNetworkKeyMap(),
		logViewer:       NewNetworkLogViewerModel(),
		manager:         corenetwork.NewNetworkManager(nil),
		layout:          common.NewLayout(0, 0),
		checkedActions:  make(map[networkActionID]bool),
		hostDomainInput: domainInput,
		hostIPInput:     ipInput,
	}
}

func NewNetworkLogViewerModel() NetworkLogViewerModel {
	logViewport := viewport.New(0, defaultNetworkLogViewportHeight)
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
		m.layout = common.NewLayout(msg.Width, msg.Height)
		m.layoutComponents()
	case networkFinishedMsg:
		if m.cancelNetwork != nil {
			m.cancelNetwork()
			m.cancelNetwork = nil
		}
		m.state = networkStateFinished
		m.report = &msg.report
		m.err = msg.err
		m.notice = ""
		m.logViewer.SetMouseFocused(false)
		m.logViewer.SetReport(msg.report)
		m.layoutComponents()
		return m, nil
	case tea.KeyMsg:
		switch m.state {
		case networkStateRunning:
			if key.Matches(msg, m.keyMap.CancelRun) {
				return m.cancelRunningNetwork()
			}
			return m, batch(cmds...)
		case networkStateEditingHostsAdd:
			return m.updateHostsAddForm(msg, cmds...)
		case networkStateFinished:
			if m.logViewer.IsKeyScrollInput(msg) {
				var cmd tea.Cmd
				m.logViewer, cmd = m.logViewer.Update(msg)
				cmds = append(cmds, cmd)
				return m, batch(cmds...)
			}
			fallthrough
		case networkStateSelectingOptions:
			switch {
			case key.Matches(msg, common.DefaultKeys.Up):
				m.moveActions(-1)
				return m, nil
			case key.Matches(msg, common.DefaultKeys.Down):
				m.moveActions(1)
				return m, nil
			case key.Matches(msg, common.DefaultKeys.Space):
				m.toggleCurrentAction()
				return m, nil
			case key.Matches(msg, common.DefaultKeys.Enter):
				return m.startSelectedAction()
			}
		}
	case tea.MouseMsg:
		if m.state == networkStateFinished && m.report != nil {
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

func (m NetworkModel) View() string {
	var b strings.Builder
	layout := m.layout

	b.WriteString(layout.RenderWrapped(networkTitle, common.Title.Render))
	b.WriteString("\n")
	b.WriteString(layout.RenderWrapped(networkSubtitle, common.Muted.Render))
	b.WriteString("\n\n")

	switch m.state {
	case networkStateRunning:
		b.WriteString(layout.RenderWrapped(fmt.Sprintf("%s Running %s...", m.spinner.View(), m.currentAction().title), func(strs ...string) string {
			return strings.Join(strs, "")
		}))
		b.WriteString("\n\n")
	case networkStateEditingHostsAdd:
		b.WriteString(layout.RenderWrapped("Add a hosts entry. Blank IP defaults to 127.0.0.1.", common.Muted.Render))
		b.WriteString("\n\n")
		b.WriteString(m.hostDomainInput.View())
		b.WriteString("\n")
		b.WriteString(m.hostIPInput.View())
		b.WriteString("\n\n")
	default:
		b.WriteString(layout.RenderWrapped(networkBaselineNote, common.Muted.Render))
		b.WriteString("\n")
		persistence := "off"
		if m.persistentMode {
			persistence = "on"
		}
		b.WriteString(layout.RenderWrapped("Persistent DNS mode for preset actions: "+persistence, common.Muted.Render))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("Actions %d/%d  checked: %d\n", m.action+1, len(networkActions), len(m.selectedActionIDs())))
		b.WriteString(m.renderActions())
		b.WriteString("\n")
	}

	if m.notice != "" {
		b.WriteString(layout.RenderWrapped(m.notice, common.Warning.Render))
		b.WriteString("\n\n")
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

func (m *NetworkModel) layoutComponents() {
	m.help.Width = m.layout.Width
	width := common.MinInt(50, common.MaxInt(20, m.layout.Width-10))
	m.hostDomainInput.Width = width
	m.hostIPInput.Width = width
	m.logViewer.SetSize(m.layout.Width, m.layout.ViewportHeight(networkLogReservedHeight, networkLogMinHeight, networkLogMaxHeight))
}

func (m NetworkModel) mouseInLogViewer(msg tea.MouseMsg) bool {
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

func (m *NetworkModel) moveActions(delta int) {
	if m.state != networkStateFinished {
		m.state = networkStateSelectingOptions
	}

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
	actions := m.selectedActionIDs()
	if len(actions) == 0 {
		actions = []networkActionID{m.currentAction().id}
	}

	if actionIDsContain(actions, networkActionHostsAdd) {
		m.state = networkStateEditingHostsAdd
		m.notice = ""
		m.report = nil
		m.err = nil
		m.focusHostField(0)
		return m, nil
	}

	return m.startRun(networkRunOptions{actions: actions})
}

func (m NetworkModel) updateHostsAddForm(msg tea.KeyMsg, cmds ...tea.Cmd) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keyMap.NextField):
		m.focusHostField((m.focusedHostField + 1) % 2)
		return m, nil
	case key.Matches(msg, common.DefaultKeys.Up):
		m.focusHostField(wrapIndex(m.focusedHostField-1, 2))
		return m, nil
	case key.Matches(msg, common.DefaultKeys.Down):
		m.focusHostField(wrapIndex(m.focusedHostField+1, 2))
		return m, nil
	case key.Matches(msg, common.DefaultKeys.Enter):
		domain := strings.TrimSpace(m.hostDomainInput.Value())
		if domain == "" {
			m.notice = "Enter a domain before adding a hosts entry."
			return m, nil
		}
		ip := strings.TrimSpace(m.hostIPInput.Value())
		if ip == "" {
			ip = "127.0.0.1"
		}
		return m.startRun(networkRunOptions{
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
	m.report = nil
	m.err = nil
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

	ctx, cancel := context.WithTimeout(context.Background(), networkRunTimeout)
	m.cancelNetwork = cancel
	return m, batch(m.spinner.Tick, runNetwork(ctx, m.manager, options))
}

func (m NetworkModel) cancelRunningNetwork() (tea.Model, tea.Cmd) {
	if m.cancelNetwork != nil {
		m.cancelNetwork()
		m.cancelNetwork = nil
	}
	m.notice = "Canceling network operation..."
	return m, nil
}

func (m NetworkModel) renderActions() string {
	var b strings.Builder
	width := m.layout.Width
	if width <= 0 {
		width = common.DefaultContentWidth
	}

	start, end := m.visibleActionRange()
	if start > 0 {
		b.WriteString(common.Muted.Render("  ... earlier actions"))
		b.WriteString("\n")
	}
	for index := start; index < end; index++ {
		action := networkActions[index]
		selected := m.action == index
		checked := m.checkedActions != nil && m.checkedActions[action.id]
		b.WriteString(networkActionRow(selected, checked, action.title))
		if selected {
			for _, detail := range action.details {
				for _, line := range common.WrapLine("      - ", detail, width) {
					b.WriteString(common.Muted.Render(line))
					b.WriteString("\n")
				}
			}
		}
	}
	if end < len(networkActions) {
		b.WriteString(common.Muted.Render("  ... more actions"))
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func (m NetworkModel) visibleActionRange() (int, int) {
	rows := m.layout.ListHeight(len(networkActions), networkActionsReservedHeight, networkActionsMinHeight)
	rows = common.MaxInt(networkActionsMinHeight, rows-2)
	rows = common.MinInt(rows, len(networkActions))

	start := m.action - rows/2
	if start < 0 {
		start = 0
	}
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

func (m NetworkModel) renderHelp() string {
	return m.help.View(m.keyMap.contextual(m.state))
}

func (m *NetworkLogViewerModel) SetSize(width, height int) {
	if width <= 0 {
		width = common.DefaultContentWidth
	}
	if height <= 0 {
		height = defaultNetworkLogViewportHeight
	}

	m.viewport.Width = width
	m.viewport.Height = height
	m.refreshContent()
}

func (m *NetworkLogViewerModel) SetReport(report corenetwork.Report) {
	m.report = &report
	m.refreshContent()
	m.viewport.GotoBottom()
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

	switch {
	case key.Matches(msg, m.viewport.KeyMap.Up, m.viewport.KeyMap.PageUp, m.viewport.KeyMap.HalfPageUp):
		return true
	case key.Matches(msg, m.viewport.KeyMap.Down, m.viewport.KeyMap.PageDown, m.viewport.KeyMap.HalfPageDown):
		return true
	}

	return false
}

func (m NetworkLogViewerModel) IsFocusedMouseScrollInput(msg tea.MouseMsg) bool {
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

func (m NetworkLogViewerModel) View() string {
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
	for _, line := range common.WrapLine("  operation: ", report.Operation, width) {
		b.WriteString(line)
		b.WriteString("\n")
	}
	stats := fmt.Sprintf("warnings: %d  errors: %d", report.Warnings, report.Errors)
	for _, line := range common.WrapLine("  ", stats, width) {
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString("Recent activity\n")
	b.WriteString(m.viewport.View())

	return b.String()
}

func runNetwork(ctx context.Context, manager corenetwork.NetworkManager, options networkRunOptions) tea.Cmd {
	return func() tea.Msg {
		report, err := runNetworkActions(ctx, manager, options)

		return networkFinishedMsg{
			report: report,
			err:    err,
		}
	}
}

func runNetworkActions(ctx context.Context, manager corenetwork.NetworkManager, options networkRunOptions) (corenetwork.Report, error) {
	actions := options.actions
	if len(actions) == 0 {
		actions = []networkActionID{networkActionViewConfig}
	}
	if len(actions) == 1 {
		return runNetworkAction(ctx, manager, actions[0], options)
	}

	combined := corenetwork.Report{Operation: fmt.Sprintf("Batch Network Operations (%d selected)", len(actions))}
	var runErrors []error
	for index, action := range actions {
		actionReport, err := runNetworkAction(ctx, manager, action, options)
		combined.Entries = append(combined.Entries, corenetwork.Entry{
			Time:    time.Now(),
			Level:   corenetwork.LevelInfo,
			Message: fmt.Sprintf("Starting %d/%d: %s", index+1, len(actions), actionTitle(action)),
		})
		combined.Entries = append(combined.Entries, actionReport.Entries...)
		combined.Warnings += actionReport.Warnings
		combined.Errors += actionReport.Errors
		if err != nil {
			combined.Errors++
			runErrors = append(runErrors, err)
			combined.Entries = append(combined.Entries, corenetwork.Entry{
				Time:    time.Now(),
				Level:   corenetwork.LevelError,
				Message: fmt.Sprintf("%s failed: %v", actionTitle(action), err),
			})
		}
		if ctx.Err() != nil {
			break
		}
	}
	return combined, errors.Join(runErrors...)
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
		return common.Muted.Render("  No activity")
	}

	if width <= 0 {
		width = common.DefaultContentWidth
	}

	var b strings.Builder
	for _, entry := range report.Entries {
		prefix := fmt.Sprintf("  [%s] ", entry.Level)
		for _, line := range common.WrapLine(prefix, entry.Message, width) {
			b.WriteString(styleNetworkActivityLine(entry.Level, line))
			b.WriteString("\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func styleNetworkActivityLine(level corenetwork.Level, line string) string {
	switch level {
	case corenetwork.LevelWarn:
		return common.Warning.Render(line)
	case corenetwork.LevelError:
		return common.Error.Render(line)
	case corenetwork.LevelSuccess:
		return common.Success.Render(line)
	default:
		return common.Muted.Render(line)
	}
}

func networkActionRow(selected bool, checked bool, label string) string {
	cursor := " "
	renderedLabel := label
	if selected {
		cursor = common.Selected.Render(">")
		renderedLabel = common.Selected.Render(label)
	}

	marker := "[ ]"
	if checked {
		marker = common.Success.Render("[x]")
	}

	return fmt.Sprintf("  %s %s %s\n", cursor, marker, renderedLabel)
}
