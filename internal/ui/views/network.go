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
	Move       key.Binding
	Select     key.Binding
	AddEntry   key.Binding
	Choose     key.Binding
	Toggle     key.Binding
	NextField  key.Binding
	LeaveForm  key.Binding
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
	canceled         bool
	cancelNetwork    context.CancelFunc
	notice           string
	layout           common.Layout
	compactLevel     compactionLevel
	actionRows       int
	state            networkViewState
	action           int
	checkedActions   map[networkActionID]bool
	persistentMode   bool
	hostDomainInput  textinput.Model
	hostIPInput      textinput.Model
	focusedHostField int
	pendingRun       networkRunOptions
	confirmRun       bool
}

type NetworkLogViewerModel struct {
	viewport     viewport.Model
	report       *corenetwork.Report
	mouseFocused bool
}

type networkFinishedMsg struct {
	report   corenetwork.Report
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
	networkTitle        = "Network & Diagnostics Manager"
	networkSubtitle     = "Cross-platform network inspection, diagnostics, cache clearing, and per-command elevated configuration."
	networkBaselineNote = "Read-only actions run as your user. Write actions show what they change and ask for confirmation first."
	networkRunTimeout   = 2 * time.Minute

	defaultNetworkLogViewportHeight = 12
	networkActionsMinHeight         = 3
	networkLogMinHeight             = 3
	networkLogMaxHeight             = 20
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
		Move: key.NewBinding(
			key.WithKeys("up", "down", "k", "j"),
			key.WithHelp("up/down:", "choose"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter:", "run checked"),
		),
		AddEntry: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter:", "add entry"),
		),
		Choose: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter:", "apply choice"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space:", "toggle"),
		),
		NextField: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab:", "next field"),
		),
		LeaveForm: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc:", "back"),
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
		return []key.Binding{common.DefaultKeys.CancelRun, common.DefaultKeys.CancelAndQuit}
	case networkStateEditingHostsAdd:
		return []key.Binding{k.NextField, k.AddEntry, k.LeaveForm}
	case networkStateConfirmingWrite:
		return []key.Binding{k.Move, k.Choose, common.DefaultKeys.Yes, common.DefaultKeys.No}
	case networkStateFinished:
		return []key.Binding{common.DefaultKeys.ScrollLog, common.DefaultKeys.BackToList, k.BackToMenu}
	default:
		return []key.Binding{k.Move, k.Toggle, k.Select, k.BackToMenu}
	}
}

func (k networkContextualKeyMap) FullHelp() [][]key.Binding {
	switch k.state {
	case networkStateRunning:
		return [][]key.Binding{{common.DefaultKeys.CancelRun, common.DefaultKeys.CancelAndQuit}}
	case networkStateEditingHostsAdd:
		return [][]key.Binding{{k.NextField, k.AddEntry, k.LeaveForm}}
	case networkStateConfirmingWrite:
		return [][]key.Binding{{k.Move, k.Choose, common.DefaultKeys.Yes, common.DefaultKeys.No}}
	case networkStateFinished:
		return [][]key.Binding{{common.DefaultKeys.ScrollLog, common.DefaultKeys.BackToList, k.BackToMenu}}
	default:
		return [][]key.Binding{{k.Move, k.Toggle, k.Select, k.BackToMenu}}
	}
}

func NewNetworkModel() NetworkModel {
	return NewNetworkModelWithManager(corenetwork.NewNetworkManager(nil))
}

func NewNetworkModelWithManager(manager corenetwork.NetworkManager) NetworkModel {
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
		manager:         manager,
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
	next, cmd := m.update(msg)
	fitted := next.(NetworkModel)
	fitted.fitToHeight()
	return fitted, cmd
}

func (m NetworkModel) update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		m.canceled = msg.canceled
		m.notice = ""
		m.logViewer.SetMouseFocused(false)
		m.logViewer.SetReport(msg.report)
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
				cmds = append(cmds, cmd)
				return m, batch(cmds...)
			}
			if key.Matches(msg, common.DefaultKeys.BackToList) {
				m.state = networkStateSelectingOptions
				return m, nil
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
	if m.compactLevel < compactWithoutSubtitle {
		b.WriteString(layout.RenderWrapped(networkSubtitle, common.Muted.Render))
		b.WriteString("\n")
	}
	b.WriteString("\n")

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
	case networkStateConfirmingWrite:
		b.WriteString(layout.RenderWrapped("These actions change your system:", common.Error.Render))
		b.WriteString("\n")
		for _, change := range m.pendingChanges() {
			b.WriteString(layout.RenderWrapped("- "+change, func(strs ...string) string {
				return strings.Join(strs, "")
			}))
			b.WriteString("\n")
		}
		b.WriteString(layout.RenderWrapped(corenetwork.ElevationNote(), common.Muted.Render))
		b.WriteString("\n\n")
		b.WriteString(confirmationRow(!m.confirmRun, "Cancel"))
		b.WriteString(confirmationRow(m.confirmRun, "Run these changes"))
		b.WriteString("\n")
	default:
		if m.compactLevel < compactWithoutNotes {
			b.WriteString(layout.RenderWrapped(networkBaselineNote+" "+corenetwork.ElevationNote(), common.Muted.Render))
			b.WriteString("\n")
		}
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

	if m.canceled {
		b.WriteString("\n")
		b.WriteString(layout.RenderWrapped(runCanceledText, common.Warning.Render))
		b.WriteString("\n")
	} else if m.err != nil {
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
	m.logViewer.SetSize(m.layout.Width, m.logViewer.viewport.Height)
}

func (m *NetworkModel) fitToHeight() {
	logMinimumExtra := 0
	if m.report != nil {
		logMinimumExtra = networkLogMinHeight - 1
	}

	for level := compactNone; level <= compactWithoutSubtitle; level++ {
		probe := *m
		probe.compactLevel = level
		probe.actionRows = networkActionsMinHeight
		probe.logViewer.viewport.Height = 1
		spare := m.layout.Height - lipgloss.Height(probe.View()) - logMinimumExtra
		if spare < 0 && level < compactWithoutSubtitle {
			continue
		}

		spare = common.MaxInt(0, spare)
		m.compactLevel = level
		logHeight := networkLogMinHeight
		if m.report != nil {
			logHeight = common.MinInt(networkLogMaxHeight, networkLogMinHeight+spare/2)
			spare -= logHeight - networkLogMinHeight
		}
		m.actionRows = common.MinInt(len(networkActions), networkActionsMinHeight+spare)
		if m.report != nil {
			spare -= m.actionRows - networkActionsMinHeight
			m.logViewer.SetSize(m.layout.Width, common.MinInt(networkLogMaxHeight, logHeight+spare))
		}
		return
	}
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

	return m.requestRun(networkRunOptions{actions: actions})
}

func (m NetworkModel) requestRun(options networkRunOptions) (tea.Model, tea.Cmd) {
	for _, action := range options.actions {
		if !isReadOnlyNetworkAction(action) {
			m.state = networkStateConfirmingWrite
			m.pendingRun = options
			m.confirmRun = false
			m.notice = ""
			return m, nil
		}
	}
	return m.startRun(options)
}

func (m NetworkModel) updateWriteConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, common.DefaultKeys.Yes):
		return m.startRun(m.pendingRun)
	case key.Matches(msg, common.DefaultKeys.No, common.DefaultKeys.CancelRun):
		return m.cancelWriteConfirmation()
	case key.Matches(msg, common.DefaultKeys.Up, common.DefaultKeys.Down):
		m.confirmRun = !m.confirmRun
	case key.Matches(msg, common.DefaultKeys.Enter, common.DefaultKeys.Space):
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
	m.notice = "Canceled; nothing was changed."
	return m, nil
}

func (m NetworkModel) pendingChanges() []string {
	changes := make([]string, 0, len(m.pendingRun.actions))
	for _, action := range m.pendingRun.actions {
		if isReadOnlyNetworkAction(action) {
			continue
		}
		changes = append(changes, actionTitle(action)+": "+m.networkActionChange(action))
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

func (m NetworkModel) networkActionChange(action networkActionID) string {
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
		return fmt.Sprintf("appends \"%s\t%s\t%s\" to %s.", m.pendingRun.hosts.IP, m.pendingRun.hosts.Domain, corenetwork.HostsManagedMarker, hostsPath)
	case networkActionHostsRemoveCustom:
		return fmt.Sprintf("removes lines tagged %s from %s; all other lines stay.", corenetwork.HostsManagedMarker, hostsPath)
	case networkActionHostsRestore:
		return fmt.Sprintf("saves %s as %s.before-restore-<time>, then overwrites it with the newest %s.backup-<time>.", hostsPath, hostsPath, hostsPath)
	case networkActionBrowserChrome, networkActionBrowserFirefox, networkActionBrowserEdge, networkActionBrowserBrave, networkActionBrowserOpera, networkActionBrowserAll:
		return "deletes the cache folders listed above for the current user."
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

func (m NetworkModel) updateHostsAddForm(msg tea.KeyMsg, cmds ...tea.Cmd) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keyMap.LeaveForm):
		m.state = networkStateSelectingOptions
		m.notice = ""
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
		if err := corenetwork.ValidateHostsEntry(ip, domain); err != nil {
			m.notice = err.Error()
			return m, nil
		}
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
	m.report = nil
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
		if selected && m.compactLevel < compactWithoutDetails {
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
	rows := common.MinInt(common.MaxInt(networkActionsMinHeight, m.actionRows), len(networkActions))

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

	if width == m.viewport.Width && height == m.viewport.Height {
		return
	}

	atBottom := m.viewport.AtBottom()
	m.viewport.Width = width
	m.viewport.Height = height
	m.refreshContent()
	if atBottom {
		m.viewport.GotoBottom()
	}
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
			report:   report,
			err:      err,
			canceled: runWasCanceled(ctx, err),
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
			runErrors = append(runErrors, err)
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			combined.Errors++
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
