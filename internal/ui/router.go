package ui

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"utils/internal/ui/common"
	"utils/internal/ui/views"
)

type AppFeature interface {
	Title() string
	Model() tea.Model
}

type helpKeysFeature interface {
	HelpKeys() []key.Binding
}

type tab struct {
	feature AppFeature
	model   tea.Model
}

type notice struct {
	tone common.Tone
	text string
	id   int
}

type noticeExpiredMsg struct {
	id int
}

type Router struct {
	tabs         []tab
	active       int
	info         appInfo
	width        int
	height       int
	helpOpen     bool
	notice       notice
	quitAfterRun bool
}

var _ tea.Model = Router{}

var shellKeys = struct {
	Next key.Binding
	Prev key.Binding
	Help key.Binding
	Quit key.Binding
	Back key.Binding
}{
	Next: key.NewBinding(key.WithKeys("tab")),
	Prev: key.NewBinding(key.WithKeys("shift+tab")),
	Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit: key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	Back: key.NewBinding(key.WithKeys("esc", "?", "q"), key.WithHelp("esc", "close")),
}

func NewRouter(features ...AppFeature) Router {
	return NewRouterWithVersion("dev", features...)
}

func NewRouterWithVersion(version string, features ...AppFeature) Router {
	if len(features) == 0 {
		features = DefaultFeatures()
	}
	tabs := make([]tab, 0, len(features))
	for _, feature := range features {
		if feature == nil {
			continue
		}
		tabs = append(tabs, tab{feature: feature, model: feature.Model()})
	}
	return Router{tabs: tabs, info: currentAppInfo(version)}
}

func (m Router) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.tabs))
	for _, tab := range m.tabs {
		cmds = append(cmds, tab.model.Init())
	}
	return tea.Batch(cmds...)
}

func (m Router) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m.broadcast(m.bodySize())
	case common.NoticeMsg:
		return m.showNotice(msg.Tone, msg.Text)
	case noticeExpiredMsg:
		if msg.id == m.notice.id {
			m.notice = notice{id: m.notice.id}
		}
		return m, nil
	case tea.MouseMsg:
		return m.updateMouse(msg)
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m.broadcast(msg)
}

func (m Router) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.tabs) == 0 {
		if common.IsForceQuit(msg) || key.Matches(msg, shellKeys.Quit) {
			return m, tea.Quit
		}
		return m, nil
	}
	active := m.activeModel()
	if common.IsForceQuit(msg) {
		if m.quitAfterRun || !isRunning(active) {
			return m, tea.Quit
		}
		m.quitAfterRun = true
		return m.updateActive(msg)
	}
	m.notice.text = ""
	if m.helpOpen {
		if key.Matches(msg, shellKeys.Back) {
			m.helpOpen = false
		}
		return m, nil
	}
	if target, ok := m.switchTarget(msg); ok && isRunning(active) {
		if target == m.active {
			return m, nil
		}
		return m.showNotice(common.ToneWarning, "A run is in progress · esc cancels it before switching")
	}
	if isRunning(active) || ownsKeys(active) {
		return m.updateActive(msg)
	}
	if target, ok := m.switchTarget(msg); ok {
		return m.activate(target)
	}
	switch {
	case key.Matches(msg, shellKeys.Help):
		m.helpOpen = true
		return m, nil
	case key.Matches(msg, shellKeys.Quit):
		return m, tea.Quit
	}
	return m.updateActive(msg)
}

func (m Router) switchTarget(msg tea.KeyMsg) (int, bool) {
	count := len(m.tabs)
	switch {
	case key.Matches(msg, shellKeys.Next):
		return (m.active + 1) % count, true
	case key.Matches(msg, shellKeys.Prev):
		return (m.active + count - 1) % count, true
	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1:
		if index, err := strconv.Atoi(string(msg.Runes)); err == nil && index >= 1 && index <= count {
			return index - 1, true
		}
	}
	return 0, false
}

func (m Router) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.helpOpen || len(m.tabs) == 0 {
		return m, nil
	}
	if msg.Y == 0 && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		target := common.TabAt(m.tabTitles(), msg.X-common.MarginX)
		if target < 0 || target == m.active {
			return m, nil
		}
		if isRunning(m.activeModel()) {
			return m.showNotice(common.ToneWarning, "A run is in progress · esc cancels it before switching")
		}
		return m.activate(target)
	}
	msg.X -= common.MarginX
	msg.Y -= common.HeaderHeight(m.height)
	return m.updateActive(msg)
}

func (m Router) activate(target int) (tea.Model, tea.Cmd) {
	if target == m.active {
		return m, nil
	}
	m.active = target
	return m.updateActive(common.ActivatedMsg{})
}

func (m Router) showNotice(tone common.Tone, text string) (tea.Model, tea.Cmd) {
	id := m.notice.id + 1
	m.notice = notice{tone: tone, text: text, id: id}
	return m, tea.Tick(noticeDuration, func(time.Time) tea.Msg { return noticeExpiredMsg{id: id} })
}

func (m Router) activeModel() tea.Model {
	if len(m.tabs) == 0 {
		return nil
	}
	return m.tabs[m.active].model
}

func (m Router) withModel(index int, model tea.Model) Router {
	tabs := append([]tab(nil), m.tabs...)
	tabs[index].model = model
	m.tabs = tabs
	return m
}

func (m Router) updateActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(m.tabs) == 0 {
		return m, nil
	}
	next, cmd := m.tabs[m.active].model.Update(msg)
	m = m.withModel(m.active, next)
	return m, m.afterUpdate(cmd)
}

func (m Router) broadcast(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0, len(m.tabs))
	for index, tab := range m.tabs {
		next, cmd := tab.model.Update(msg)
		m = m.withModel(index, next)
		cmds = append(cmds, cmd)
	}
	return m, m.afterUpdate(tea.Batch(cmds...))
}

func (m Router) afterUpdate(cmd tea.Cmd) tea.Cmd {
	if m.quitAfterRun && !isRunning(m.activeModel()) {
		return tea.Batch(cmd, tea.Quit)
	}
	return cmd
}

func isRunning(model tea.Model) bool {
	running, ok := model.(common.RunningView)
	return ok && running.Running()
}

func ownsKeys(model tea.Model) bool {
	owner, ok := model.(common.KeyOwner)
	return ok && owner.OwnsKeys()
}

func (m Router) View() string {
	width, height := m.terminalSize()
	if view, small := common.TooSmall(width, height); small {
		return view
	}

	bodyWidth, bodyHeight := m.bodyDimensions()
	body := ""
	if model := m.activeModel(); model != nil {
		body = fitBody(model.View(), bodyWidth, bodyHeight)
		if modal, ok := model.(common.ModalView); ok {
			if overlay := modal.Modal(); overlay != "" {
				body = common.Overlay(body, overlay, bodyWidth, bodyHeight)
			}
		}
	}
	if m.helpOpen {
		body = common.Overlay(body, m.helpView(bodyWidth, bodyHeight), bodyWidth, bodyHeight)
	}

	rows := []string{
		common.Header(bodyWidth, m.tabTitles(), m.active, m.info.headerChoices()...),
		fitBody(body, bodyWidth, bodyHeight),
		m.footer(bodyWidth),
	}
	return lipgloss.NewStyle().Padding(0, common.MarginX).Render(strings.Join(rows, "\n"))
}

func (m Router) footer(width int) string {
	model := m.activeModel()
	status := ""
	if view, ok := model.(common.FooterView); ok {
		status = view.FooterStatus()
	}
	room := common.FooterRoom(width, status)
	if m.notice.text != "" {
		return common.Footer(width, common.Notice(room, m.notice.tone, m.notice.text), status)
	}
	return common.Footer(width, common.Hints(m.fitHints(room)), status)
}

func (m Router) fitHints(room int) []key.Binding {
	if m.helpOpen {
		return []key.Binding{shellKeys.Back}
	}
	model := m.activeModel()
	var viewHints []key.Binding
	if view, ok := model.(common.FooterView); ok && view.FooterKeys() != nil {
		viewHints = enabled(view.FooterKeys().ShortHelp())
	}
	if isRunning(model) || ownsKeys(model) {
		return trimHints(viewHints, nil, nil, room)
	}
	var switchHint []key.Binding
	if len(m.tabs) > 1 {
		next := m.tabs[(m.active+1)%len(m.tabs)].feature.Title()
		switchHint = []key.Binding{key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", strings.ToLower(next)))}
	}
	return trimHints(viewHints, switchHint, []key.Binding{shellKeys.Help, shellKeys.Quit}, room)
}

func trimHints(viewHints, switchHint, globals []key.Binding, room int) []key.Binding {
	if len(viewHints) > maxFooterHints {
		viewHints = viewHints[:maxFooterHints]
	}
	for {
		all := append(append(append([]key.Binding(nil), viewHints...), switchHint...), globals...)
		if (len(all) <= maxFooterHints && lipgloss.Width(common.Hints(all)) <= room) || len(all) == 0 {
			return all
		}
		switch {
		case len(switchHint) > 0:
			switchHint = nil
		case len(viewHints) > 0 && len(all) > maxFooterHints:
			viewHints = viewHints[:len(viewHints)-1]
		case len(globals) > 1:
			globals = globals[1:]
		case len(viewHints) > 0:
			viewHints = viewHints[:len(viewHints)-1]
		default:
			globals = nil
		}
	}
}

func enabled(bindings []key.Binding) []key.Binding {
	kept := make([]key.Binding, 0, len(bindings))
	for _, binding := range bindings {
		if binding.Enabled() {
			kept = append(kept, binding)
		}
	}
	return kept
}

func (m Router) tabTitles() []string {
	titles := make([]string, 0, len(m.tabs))
	for _, tab := range m.tabs {
		titles = append(titles, tab.feature.Title())
	}
	return titles
}

func (m Router) terminalSize() (int, int) {
	if m.width <= 0 || m.height <= 0 {
		return common.DefaultContentWidth, common.DefaultContentHeight
	}
	return m.width, m.height
}

func (m Router) bodyDimensions() (int, int) {
	width, height := m.terminalSize()
	return common.MaxInt(1, width-2*common.MarginX), common.MaxInt(1, height-common.HeaderHeight(height)-footerHeight)
}

func (m Router) bodySize() tea.WindowSizeMsg {
	width, height := m.bodyDimensions()
	return tea.WindowSizeMsg{Width: width, Height: height}
}

func fitBody(body string, width, height int) string {
	rows := strings.Split(strings.TrimRight(body, "\n"), "\n")
	for index, row := range rows {
		rows[index] = common.Truncate(row, width)
	}
	return common.FitHeight(strings.Join(rows, "\n"), height)
}

type appFeature struct {
	title    string
	helpKeys []key.Binding
	model    func() tea.Model
}

func (f appFeature) Title() string {
	return f.title
}

func (f appFeature) Model() tea.Model {
	return f.model()
}

func (f appFeature) HelpKeys() []key.Binding {
	return f.helpKeys
}

func DefaultFeatures() []AppFeature {
	return []AppFeature{
		appFeature{
			title:    featureCleanerTitle,
			helpKeys: cleanerHelpKeys,
			model: func() tea.Model {
				return views.NewCleanerModel()
			},
		},
		appFeature{
			title:    featureNetworkTitle,
			helpKeys: networkHelpKeys,
			model: func() tea.Model {
				return views.NewNetworkModel()
			},
		},
	}
}
