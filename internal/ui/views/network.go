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
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	corenetwork "utils/internal/core/network"
	"utils/internal/ui/common"
)

type networkViewState int

const (
	networkStateSelectingOptions networkViewState = iota
	networkStateEditingHostsAdd
	networkStateRunning
	networkStateFinished
)

const (
	networkRunTimeout    = 2 * time.Minute
	networkStatusTimeout = 15 * time.Second
	networkStatusTick    = time.Second
	networkLineBuffer    = 512
	networkOutputKeep    = 400
	networkDefaultIP     = "127.0.0.1"
	networkDefaultDomain = "example.local"
	networkCanceledRun   = "Canceled · nothing was changed"
	networkQuitNotice    = "UTILS quits when the run stops · ctrl+c again quits now"
)

type networkKeyMap struct {
	Move        key.Binding
	Choose      key.Binding
	Run         key.Binding
	RunBatch    key.Binding
	Batch       key.Binding
	Refresh     key.Binding
	Info        key.Binding
	ClearBatch  key.Binding
	NextField   key.Binding
	PrevField   key.Binding
	AddEntry    key.Binding
	LeaveForm   key.Binding
	Scroll      key.Binding
	Back        key.Binding
	CloseResult key.Binding
}

func newNetworkKeyMap() networkKeyMap {
	return networkKeyMap{
		Move:        key.NewBinding(key.WithKeys("up", "down", "k", "j"), key.WithHelp("↑↓", "move")),
		Choose:      key.NewBinding(key.WithKeys("left", "right", "h", "l"), key.WithHelp("←→", "choose")),
		Run:         key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "run")),
		RunBatch:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "run batch")),
		Batch:       key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "add to batch")),
		Refresh:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh status")),
		Info:        key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "info")),
		ClearBatch:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear batch")),
		NextField:   key.NewBinding(key.WithKeys("tab", "down"), key.WithHelp("tab", "next")),
		PrevField:   key.NewBinding(key.WithKeys("shift+tab", "up")),
		AddEntry:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "add")),
		LeaveForm:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Scroll:      key.NewBinding(key.WithKeys("up", "down", "pgup", "pgdown"), key.WithHelp("↑↓", "scroll")),
		Back:        key.NewBinding(key.WithKeys("enter", "esc"), key.WithHelp("enter", "back")),
		CloseResult: key.NewBinding(key.WithKeys("esc", "i")),
	}
}

type NetworkModel struct {
	manager   corenetwork.NetworkManager
	keyMap    networkKeyMap
	spinner   spinner.Model
	logViewer common.LogViewer
	width     int
	height    int
	state     networkViewState

	focus      networkActionID
	choices    [2]networkActionID
	listOffset int
	batch      []networkActionID
	infoOpen   bool

	status         corenetwork.Status
	statusLoaded   bool
	statusLoading  bool
	statusAt       time.Time
	statusSeq      int
	statusTicking  bool
	announceStatus bool
	persistentMode bool

	hostDomainInput  textinput.Model
	hostIPInput      textinput.Model
	focusedHostField int

	confirm    common.Confirm
	pendingRun networkRunOptions

	running       []networkActionID
	runSeq        int
	runStarted    time.Time
	runElapsed    time.Duration
	runIndex      int
	cancelNetwork context.CancelFunc
	canceling     bool
	lines         chan tea.Msg
	output        []string

	results  []networkActionResult
	err      error
	canceled bool
}

type networkActionResult struct {
	action networkActionID
	report corenetwork.Report
	err    error
}

type networkRunOptions struct {
	actions []networkActionID
	params  map[networkActionID]corenetwork.Params
}

type networkStatusMsg struct {
	seq    int
	status corenetwork.Status
	err    error
}

type networkStatusTickMsg struct{}

type networkLineMsg struct {
	seq  int
	line string
}

type networkStepMsg struct {
	seq   int
	index int
}

type networkFinishedMsg struct {
	seq      int
	results  []networkActionResult
	err      error
	canceled bool
}

var _ tea.Model = NetworkModel{}

func NewNetworkModel() NetworkModel {
	return NewNetworkModelWithManager(corenetwork.NewNetworkManager(nil))
}

func NewNetworkModelWithManager(manager corenetwork.NetworkManager) NetworkModel {
	domainInput := textinput.New()
	domainInput.Placeholder = networkDefaultDomain
	domainInput.Prompt = ""
	domainInput.CharLimit = 253
	domainInput.SetValue(networkDefaultDomain)

	ipInput := textinput.New()
	ipInput.Placeholder = networkDefaultIP
	ipInput.Prompt = ""
	ipInput.CharLimit = 64
	ipInput.SetValue(networkDefaultIP)

	return NetworkModel{
		manager: manager,
		keyMap:  newNetworkKeyMap(),
		spinner: spinner.New(
			spinner.WithSpinner(trimmedSpinner(spinner.Dot)),
			spinner.WithStyle(common.Accent),
		),
		logViewer:       common.NewLogViewer(),
		focus:           networkActionViewConfig,
		choices:         [2]networkActionID{networkActionBrowserChrome, networkActionPersistentStatus},
		hostDomainInput: domainInput,
		hostIPInput:     ipInput,
	}
}

func (m NetworkModel) Init() tea.Cmd {
	return nil
}

func (m NetworkModel) Running() bool {
	return m.state == networkStateRunning
}

func (m NetworkModel) OwnsKeys() bool {
	return m.Running() || m.state == networkStateEditingHostsAdd || m.confirm.Open()
}

func (m NetworkModel) Modal() string {
	if m.confirm.Open() {
		return m.confirm.View(m.width, m.height)
	}
	if m.infoOpen && m.layout().detailHidden && m.state == networkStateSelectingOptions {
		return m.infoOverlay()
	}
	return ""
}

func (m NetworkModel) FooterKeys() help.KeyMap {
	k := m.keyMap
	switch {
	case m.confirm.Open():
		return m.confirm.Keys()
	case m.state == networkStateRunning:
		return common.KeyList{common.DefaultKeys.CancelRun, common.DefaultKeys.CancelAndQuit}
	case m.state == networkStateEditingHostsAdd:
		return common.KeyList{k.NextField, k.AddEntry, k.LeaveForm}
	case m.state == networkStateFinished:
		return common.KeyList{k.Scroll, k.Back, k.Refresh}
	}
	keys := common.KeyList{k.Run}
	if len(m.batch) > 0 {
		keys = common.KeyList{k.RunBatch}
	}
	if m.focusedRowIsInline() {
		keys = append(keys, k.Choose)
	}
	keys = append(keys, k.Batch, k.Refresh)
	if m.layout().detailHidden {
		keys = append(keys, k.Info)
	}
	if len(m.batch) > 0 {
		keys = append(keys, k.ClearBatch)
	}
	return keys
}

func (m NetworkModel) FooterStatus() string {
	switch m.state {
	case networkStateRunning:
		if m.canceling {
			return common.Warning.Render("Canceling…")
		}
		if len(m.running) > 1 {
			return fmt.Sprintf("%d/%d · %s", m.runIndex+1, len(m.running), formatTenths(time.Since(m.runStarted)))
		}
		return formatTenths(time.Since(m.runStarted))
	case networkStateFinished:
		_, errorCount := networkProblemCounts(m.results)
		if errorCount > 0 {
			return common.Error.Render(fmt.Sprintf("%d %s", errorCount, plural(errorCount, "error", "errors")))
		}
		return ""
	case networkStateSelectingOptions:
		if len(m.batch) > 0 {
			return fmt.Sprintf("%d in batch", len(m.batch))
		}
	}
	return ""
}

func (m NetworkModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	model := next.(NetworkModel)
	model.layoutComponents()
	return model, cmd
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
		m.width = msg.Width
		m.height = msg.Height
		m.layoutComponents()
		m.keepFocusVisible()
	case common.ActivatedMsg:
		return m.refreshStatus(false)
	case networkStatusMsg:
		return m.receiveStatus(msg)
	case networkStatusTickMsg:
		return m, tea.Tick(networkStatusTick, func(time.Time) tea.Msg { return networkStatusTickMsg{} })
	case networkLineMsg:
		if msg.seq != m.runSeq || m.state != networkStateRunning {
			return m, nil
		}
		m.output = append(m.output, msg.line)
		if len(m.output) > networkOutputKeep {
			m.output = m.output[len(m.output)-networkOutputKeep:]
		}
		return m, batch(append(cmds, m.listenCmd())...)
	case networkStepMsg:
		if msg.seq != m.runSeq || m.state != networkStateRunning {
			return m, nil
		}
		m.runIndex = msg.index
		return m, batch(append(cmds, m.listenCmd())...)
	case networkFinishedMsg:
		if msg.seq != m.runSeq {
			return m, nil
		}
		return m.finishRun(msg)
	case tea.KeyMsg:
		return m.updateKey(msg, cmds)
	case tea.MouseMsg:
		if m.state == networkStateFinished && (msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown) {
			var cmd tea.Cmd
			m.logViewer, cmd = m.logViewer.Update(msg)
			return m, cmd
		}
	}
	return m, batch(cmds...)
}

func (m NetworkModel) updateKey(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if m.confirm.Open() {
		return m.updateConfirm(msg)
	}
	switch m.state {
	case networkStateRunning:
		if key.Matches(msg, common.DefaultKeys.CancelRun, common.DefaultKeys.CancelAndQuit) {
			return m.cancelRunningNetwork(common.IsForceQuit(msg))
		}
		return m, batch(cmds...)
	case networkStateEditingHostsAdd:
		return m.updateHostsAddForm(msg)
	case networkStateFinished:
		switch {
		case m.logViewer.IsKeyScrollInput(msg):
			var cmd tea.Cmd
			m.logViewer, cmd = m.logViewer.Update(msg)
			return m, cmd
		case key.Matches(msg, m.keyMap.Back):
			m.state = networkStateSelectingOptions
		case key.Matches(msg, m.keyMap.Refresh):
			return m.refreshStatus(true)
		}
		return m, nil
	}
	return m.updateSelecting(msg)
}

func (m NetworkModel) updateSelecting(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := m.keyMap
	switch {
	case m.infoOpen && key.Matches(msg, k.CloseResult):
		m.infoOpen = false
	case key.Matches(msg, common.DefaultKeys.Up):
		m.moveFocus(-1)
	case key.Matches(msg, common.DefaultKeys.Down):
		m.moveFocus(1)
	case key.Matches(msg, k.Choose):
		m.chooseInline(msg.String() == "left" || msg.String() == "h")
	case key.Matches(msg, k.Batch):
		m.toggleBatch(m.focus)
	case key.Matches(msg, k.Run):
		return m.startSelectedAction()
	case key.Matches(msg, k.Refresh):
		return m.refreshStatus(true)
	case key.Matches(msg, k.Info):
		m.infoOpen = !m.infoOpen && m.layout().detailHidden
	case key.Matches(msg, k.ClearBatch):
		m.batch = nil
	}
	return m, nil
}

func (m NetworkModel) refreshStatus(announce bool) (tea.Model, tea.Cmd) {
	m.statusSeq++
	m.statusLoading = true
	m.announceStatus = announce && m.layout().card == networkCardHidden
	seq := m.statusSeq
	manager := m.manager
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), networkStatusTimeout)
		defer cancel()
		status, err := manager.ReadStatus(ctx)
		return networkStatusMsg{seq: seq, status: status, err: err}
	}
}

func (m NetworkModel) receiveStatus(msg networkStatusMsg) (tea.Model, tea.Cmd) {
	if msg.seq != m.statusSeq {
		return m, nil
	}
	m.statusLoading = false
	if msg.err != nil {
		return m, nil
	}
	m.status = msg.status
	m.statusLoaded = true
	m.statusAt = time.Now()
	if m.status.Persistent != corenetwork.StateUnknown {
		m.persistentMode = m.status.Persistent == corenetwork.StateOn
	}

	var cmds []tea.Cmd
	if m.announceStatus {
		m.announceStatus = false
		cmds = append(cmds, common.Notify(common.ToneNormal, ansi.Strip(m.statusSummary())))
	}
	if !m.statusTicking {
		m.statusTicking = true
		cmds = append(cmds, tea.Tick(networkStatusTick, func(time.Time) tea.Msg { return networkStatusTickMsg{} }))
	}
	return m, batch(cmds...)
}

func (m *NetworkModel) toggleBatch(id networkActionID) {
	if !networkItem(id).supported() {
		return
	}
	for index, queued := range m.batch {
		if queued == id {
			m.batch = append(m.batch[:index:index], m.batch[index+1:]...)
			return
		}
	}
	m.batch = append(m.batch, id)
}

func (m NetworkModel) inBatch(id networkActionID) bool {
	for _, queued := range m.batch {
		if queued == id {
			return true
		}
	}
	return false
}

func (m NetworkModel) batchInListOrder() []networkActionID {
	var ordered []networkActionID
	for _, item := range networkActions {
		if m.inBatch(item.id) {
			ordered = append(ordered, item.id)
		}
	}
	return ordered
}

func (m NetworkModel) selectedOrCurrentActionIDs() []networkActionID {
	if len(m.batch) > 0 {
		return m.batchInListOrder()
	}
	return []networkActionID{m.focus}
}

func (m NetworkModel) startSelectedAction() (tea.Model, tea.Cmd) {
	actions := m.selectedOrCurrentActionIDs()
	for _, id := range actions {
		if !networkItem(id).supported() {
			return m, nil
		}
	}
	if actionIDsContain(actions, networkActionHostsAdd) {
		m.state = networkStateEditingHostsAdd
		m.focusHostField(0)
		return m, textinput.Blink
	}
	return m.requestRun(actions)
}

func (m NetworkModel) requestRun(actions []networkActionID) (tea.Model, tea.Cmd) {
	options := networkRunOptions{actions: actions, params: map[networkActionID]corenetwork.Params{}}
	for _, id := range actions {
		options.params[id] = m.params(id)
	}
	if !runChangesSystem(actions) {
		return m.startRun(options)
	}
	m.pendingRun = options
	m.confirm = common.NewConfirm("Change system settings", "Run", m.confirmLines(options)...)
	return m, nil
}

func runChangesSystem(actions []networkActionID) bool {
	for _, id := range actions {
		if networkItem(id).writes() {
			return true
		}
	}
	return false
}

func (m NetworkModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var result common.ConfirmResult
	m.confirm, result = m.confirm.Update(msg)
	switch result {
	case common.ConfirmAccepted:
		return m.startRun(m.pendingRun)
	case common.ConfirmCanceled:
		m.pendingRun = networkRunOptions{}
		m.state = networkStateSelectingOptions
		return m, common.Notify(common.ToneNormal, networkCanceledRun)
	}
	return m, nil
}

func (m NetworkModel) updateHostsAddForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := m.keyMap
	switch {
	case key.Matches(msg, k.LeaveForm):
		m.state = networkStateSelectingOptions
		return m, nil
	case key.Matches(msg, k.NextField), key.Matches(msg, k.PrevField):
		m.focusHostField(1 - m.focusedHostField)
		return m, nil
	case key.Matches(msg, k.AddEntry):
		if domainErr, ipErr := m.hostsFieldErrors(); domainErr != "" || ipErr != "" {
			return m, nil
		}
		m.state = networkStateSelectingOptions
		return m.requestRun(m.selectedOrCurrentActionIDs())
	}

	var cmd tea.Cmd
	if m.focusedHostField == 0 {
		m.hostDomainInput, cmd = m.hostDomainInput.Update(msg)
	} else {
		m.hostIPInput, cmd = m.hostIPInput.Update(msg)
	}
	return m, cmd
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

func (m NetworkModel) hostsEntry() corenetwork.HostsOptions {
	ip := strings.TrimSpace(m.hostIPInput.Value())
	if ip == "" {
		ip = networkDefaultIP
	}
	return corenetwork.HostsOptions{Mode: corenetwork.HostsAdd, IP: ip, Domain: strings.TrimSpace(m.hostDomainInput.Value())}
}

func (m NetworkModel) hostsFieldErrors() (string, string) {
	entry := m.hostsEntry()
	domainErr, ipErr := "", ""
	if entry.Domain == "" {
		domainErr = "Enter a domain"
	} else if err := corenetwork.ValidateHostsEntry(networkDefaultIP, entry.Domain); err != nil {
		domainErr = err.Error()
	}
	if err := corenetwork.ValidateHostsEntry(entry.IP, networkDefaultDomain); err != nil {
		ipErr = err.Error()
	}
	return domainErr, ipErr
}

func (m NetworkModel) startRun(options networkRunOptions) (tea.Model, tea.Cmd) {
	m.state = networkStateRunning
	m.pendingRun = networkRunOptions{}
	m.results = nil
	m.err = nil
	m.canceled = false
	m.canceling = false
	m.output = nil
	m.infoOpen = false
	m.batch = nil
	m.running = options.actions
	m.runIndex = 0
	m.runSeq++
	m.runStarted = time.Now()
	if params, ok := options.params[networkActionTogglePersistent]; ok {
		m.persistentMode = params.Persistent
	}
	if actionIDsContain(options.actions, networkActionClearPersistent) || actionIDsContain(options.actions, networkActionResetDefaults) {
		m.persistentMode = false
	}

	ctx, cancel := context.WithTimeout(context.Background(), networkRunTimeout)
	m.cancelNetwork = cancel
	lines := make(chan tea.Msg, networkLineBuffer)
	m.lines = lines
	return m, batch(m.spinner.Tick, runNetwork(ctx, m.manager, options, m.runSeq, lines), m.listenCmd())
}

func (m NetworkModel) listenCmd() tea.Cmd {
	lines := m.lines
	if lines == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-lines
		if !ok {
			return nil
		}
		return msg
	}
}

func (m NetworkModel) finishRun(msg networkFinishedMsg) (tea.Model, tea.Cmd) {
	if m.cancelNetwork != nil {
		m.cancelNetwork()
		m.cancelNetwork = nil
	}
	m.state = networkStateFinished
	m.lines = nil
	m.runElapsed = time.Since(m.runStarted)
	m.results = msg.results
	m.err = msg.err
	m.canceled = msg.canceled
	m.canceling = false
	m.logViewer.SetSections(networkLogSections(msg.results))
	m.layoutComponents()
	if runChangesSystem(m.running) {
		return m.refreshStatus(false)
	}
	return m, nil
}

func (m NetworkModel) cancelRunningNetwork(quitAfter bool) (tea.Model, tea.Cmd) {
	if m.cancelNetwork != nil {
		m.cancelNetwork()
		m.cancelNetwork = nil
	}
	m.canceling = true
	if quitAfter {
		return m, common.Notify(common.ToneWarning, networkQuitNotice)
	}
	return m, nil
}

func runNetwork(ctx context.Context, manager corenetwork.NetworkManager, options networkRunOptions, seq int, lines chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		defer close(lines)
		send := func(msg tea.Msg) {
			select {
			case lines <- msg:
			default:
			}
		}
		streaming := manager.WithOutputLines(func(line string) { send(networkLineMsg{seq: seq, line: line}) })
		results, err := runNetworkActions(ctx, streaming, options, func(index int) { send(networkStepMsg{seq: seq, index: index}) })
		return networkFinishedMsg{seq: seq, results: results, err: err, canceled: runWasCanceled(ctx, err)}
	}
}

func runNetworkActions(ctx context.Context, manager corenetwork.NetworkManager, options networkRunOptions, onStep func(int)) ([]networkActionResult, error) {
	results := make([]networkActionResult, 0, len(options.actions))
	var runErrors []error
	for index, id := range options.actions {
		onStep(index)
		report, err := manager.Run(ctx, networkItem(id).action, options.params[id])
		results = append(results, networkActionResult{action: id, report: report, err: err})
		if err != nil {
			runErrors = append(runErrors, err)
		}
		if ctx.Err() != nil {
			break
		}
	}
	return results, errors.Join(runErrors...)
}

func actionIDsContain(actions []networkActionID, target networkActionID) bool {
	for _, action := range actions {
		if action == target {
			return true
		}
	}
	return false
}

func formatTenths(elapsed time.Duration) string {
	tenths := int(elapsed.Round(100*time.Millisecond) / (100 * time.Millisecond))
	return fmt.Sprintf("%d:%02d.%d", tenths/600, tenths/10%60, tenths%10)
}

func plural(count int, singular, many string) string {
	if count == 1 {
		return singular
	}
	return many
}
