package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"utils/internal/ui/common"
	"utils/internal/ui/views"
)

type AppFeature interface {
	Title() string
	Description() string
	Model() tea.Model
}

type menuItem struct {
	feature AppFeature
}

func (i menuItem) Title() string {
	return i.feature.Title()
}

func (i menuItem) Description() string {
	return i.feature.Description()
}

func (i menuItem) FilterValue() string {
	return strings.Join([]string{i.Title(), i.Description()}, " ")
}

type runningModel interface {
	Running() bool
}

type keyOwningModel interface {
	OwnsKeys() bool
}

type breadcrumbModel interface {
	Breadcrumb() []string
}

type footerKeysModel interface {
	FooterKeys() help.KeyMap
}

type footerStatusModel interface {
	FooterStatus() string
}

type Router struct {
	menu          list.Model
	activeFeature AppFeature
	activeModel   tea.Model
	version       string
	width         int
	height        int
	err           error
	quitAfterRun  bool
}

var _ tea.Model = Router{}

func NewRouter(features ...AppFeature) Router {
	return NewRouterWithVersion("dev", features...)
}

func NewRouterWithVersion(version string, features ...AppFeature) Router {
	if len(features) == 0 {
		features = DefaultFeatures()
	}

	items := make([]list.Item, 0, len(features))
	for _, feature := range features {
		if feature == nil {
			continue
		}
		items = append(items, menuItem{feature: feature})
	}

	menu := list.New(items, menuDelegate{}, 0, 0)
	menu.SetShowTitle(false)
	menu.SetShowStatusBar(false)
	menu.SetShowHelp(false)
	menu.SetShowFilter(false)
	menu.Styles.TitleBar = lipgloss.NewStyle().PaddingBottom(1)
	menu.DisableQuitKeybindings()

	return Router{
		menu:    menu,
		version: normalizeVersion(version),
	}
}

func (m Router) Init() tea.Cmd {
	return nil
}

func (m Router) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	router := next.(Router)
	router.menu.SetShowFilter(router.menu.SettingFilter())
	return router, cmd
}

func (m Router) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.menu.SetSize(m.bodyWidth(), common.MaxInt(1, m.bodyHeight()-m.menuLogoHeight()))

		if m.activeModel != nil {
			next, cmd := m.activeModel.Update(m.activeWindowSize())
			m.activeModel = next
			return m, cmd
		}

		return m, nil
	case tea.MouseMsg:
		if m.activeModel != nil {
			msg.X -= common.MarginX
			msg.Y -= m.headerHeight()
			return m.updateActive(msg)
		}
	case tea.KeyMsg:
		if m.activeModel == nil && m.menu.SettingFilter() && !common.IsForceQuit(msg) {
			var cmd tea.Cmd
			m.menu, cmd = m.menu.Update(msg)
			return m, cmd
		}
		if m.activeModel != nil && common.IsForceQuit(msg) {
			if m.quitAfterRun || !isRunning(m.activeModel) {
				return m, tea.Quit
			}
			m.quitAfterRun = true
			return m.updateActive(msg)
		}
		if m.activeModel != nil && ownsKeys(m.activeModel) {
			return m.updateActive(msg)
		}

		switch {
		case key.Matches(msg, common.DefaultKeys.Quit):
			if common.IsForceQuit(msg) || m.activeFeature == nil {
				return m, tea.Quit
			}
			return m.returnToMenu(), nil
		case msg.Type == tea.KeyEsc:
			if m.activeFeature != nil {
				return m.returnToMenu(), nil
			}
		case key.Matches(msg, common.DefaultKeys.Enter):
			if m.activeFeature == nil {
				return m.activateSelected()
			}
		}

		if common.IsForceQuit(msg) {
			return m, tea.Quit
		}
	}

	if m.activeFeature != nil && m.activeModel != nil {
		return m.updateActive(msg)
	}

	var cmd tea.Cmd
	m.menu, cmd = m.menu.Update(msg)
	return m, cmd
}

func (m Router) updateActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.activeModel.Update(msg)
	m.activeModel = next
	if m.quitAfterRun && !isRunning(next) {
		return m, tea.Batch(cmd, tea.Quit)
	}
	return m, cmd
}

func isRunning(model tea.Model) bool {
	running, ok := model.(runningModel)
	return ok && running.Running()
}

func ownsKeys(model tea.Model) bool {
	owner, ok := model.(keyOwningModel)
	return ok && owner.OwnsKeys()
}

func (m Router) View() string {
	width, height := m.terminalSize()
	if view, small := common.TooSmall(width, height); small {
		return view
	}

	var crumbs []string
	body, keys, status := m.menuBody(), help.KeyMap(menuKeys(m.menu)), m.menuStatus()
	if m.activeFeature != nil && m.activeModel != nil {
		crumbs, body, keys, status = m.activeCrumbs(), m.activeModel.View(), activeFooterKeys(m.activeModel), activeFooterStatus(m.activeModel)
	}

	rows := []string{
		common.Header(m.bodyWidth(), height, crumbs, m.version),
		fitBody(body, m.bodyWidth(), m.bodyHeight()),
		common.Footer(m.bodyWidth(), keys, status),
	}
	return lipgloss.NewStyle().Padding(0, common.MarginX).Render(strings.Join(rows, "\n"))
}

func (m Router) terminalSize() (int, int) {
	if m.width <= 0 || m.height <= 0 {
		return common.DefaultContentWidth, common.DefaultContentHeight
	}
	return m.width, m.height
}

func (m Router) bodyWidth() int {
	width, _ := m.terminalSize()
	return common.MaxInt(1, width-2*common.MarginX)
}

func (m Router) bodyHeight() int {
	_, height := m.terminalSize()
	return common.MaxInt(1, height-m.headerHeight()-footerHeight)
}

func (m Router) headerHeight() int {
	_, height := m.terminalSize()
	return common.HeaderHeight(height)
}

func (m Router) activeCrumbs() []string {
	if model, ok := m.activeModel.(breadcrumbModel); ok {
		if crumbs := model.Breadcrumb(); len(crumbs) > 0 {
			return crumbs
		}
	}
	return []string{m.activeFeature.Title()}
}

func activeFooterKeys(model tea.Model) help.KeyMap {
	if model, ok := model.(footerKeysModel); ok {
		return model.FooterKeys()
	}
	return nil
}

func activeFooterStatus(model tea.Model) string {
	if model, ok := model.(footerStatusModel); ok {
		return model.FooterStatus()
	}
	return ""
}

func (m Router) menuBody() string {
	rows := make([]string, 0, 3)
	if logo := m.menuLogo(); logo != "" {
		rows = append(rows, lipgloss.NewStyle().PaddingLeft(2).Render(common.Accent.Render(logo)), "")
	}
	rows = append(rows, m.menu.View())
	if m.err != nil {
		rows = append(rows, common.Error.Render(m.err.Error()))
	}
	return strings.Join(rows, "\n")
}

func (m Router) menuStatus() string {
	total := len(m.menu.Items())
	shown := len(m.menu.VisibleItems())
	if shown != total {
		return fmt.Sprintf("%d of %d tools", shown, total)
	}
	return fmt.Sprintf("%d tools", total)
}

func (m Router) menuLogo() string {
	logo := strings.TrimRight(appLogo, "\r\n")
	if m.bodyHeight() < logoMinBodyHeight || m.bodyWidth() < lipgloss.Width(logo)+2 {
		return ""
	}
	return logo
}

func (m Router) menuLogoHeight() int {
	logo := m.menuLogo()
	if logo == "" {
		return 0
	}
	return lipgloss.Height(logo) + 1
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "dev"
	}
	return version
}

func (m Router) activateSelected() (tea.Model, tea.Cmd) {
	item, ok := m.menu.SelectedItem().(menuItem)
	if !ok {
		return m, nil
	}

	m.activeFeature = item.feature
	m.activeModel = item.feature.Model()

	if m.width > 0 && m.height > 0 {
		next, _ := m.activeModel.Update(m.activeWindowSize())
		m.activeModel = next
	}

	return m, m.activeModel.Init()
}

func (m Router) activeWindowSize() tea.WindowSizeMsg {
	return tea.WindowSizeMsg{Width: m.bodyWidth(), Height: m.bodyHeight()}
}

func (m Router) returnToMenu() Router {
	m.activeFeature = nil
	m.activeModel = nil
	return m
}

type menuKeyMap struct {
	bindings []key.Binding
}

func (k menuKeyMap) ShortHelp() []key.Binding {
	return k.bindings
}

func (k menuKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.bindings}
}

func menuKeys(menu list.Model) menuKeyMap {
	if menu.SettingFilter() {
		return menuKeyMap{bindings: []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply filter")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}}
	}

	bindings := []key.Binding{
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "move")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	}
	if menu.IsFiltered() {
		bindings = append(bindings, key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear filter")))
	}
	bindings = append(bindings, key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")))
	return menuKeyMap{bindings: bindings}
}

type menuDelegate struct{}

func (d menuDelegate) Height() int {
	return 2
}

func (d menuDelegate) Spacing() int {
	return 1
}

func (d menuDelegate) Update(tea.Msg, *list.Model) tea.Cmd {
	return nil
}

func (d menuDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	entry, ok := item.(menuItem)
	if !ok {
		return
	}

	textWidth := common.MaxInt(1, m.Width()-2)
	title := common.Truncate(entry.Title(), textWidth)
	description := common.Muted.Render(common.Truncate(entry.Description(), textWidth))
	gutter := "  "
	if index == m.Index() {
		gutter = common.Accent.Render("┃") + " "
		title = lipgloss.NewStyle().Bold(true).Render(title)
	}

	fmt.Fprintf(w, "%s%s\n%s%s", gutter, title, gutter, description)
}

func fitBody(body string, width, height int) string {
	rows := strings.Split(strings.TrimRight(body, "\n"), "\n")
	for index, row := range rows {
		rows[index] = common.Truncate(row, width)
	}
	return common.FitHeight(strings.Join(rows, "\n"), height)
}

type appFeature struct {
	title       string
	description string
	model       func() tea.Model
}

func (f appFeature) Title() string {
	return f.title
}

func (f appFeature) Description() string {
	return f.description
}

func (f appFeature) Model() tea.Model {
	return f.model()
}

func DefaultFeatures() []AppFeature {
	return []AppFeature{
		appFeature{
			title:       featureCleanerTitle,
			description: featureCleanerDescription,
			model: func() tea.Model {
				return views.NewCleanerModel()
			},
		},
		appFeature{
			title:       featureNetworkTitle,
			description: featureNetworkDescription,
			model: func() tea.Model {
				return views.NewNetworkModel()
			},
		},
	}
}
