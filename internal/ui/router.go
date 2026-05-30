package ui

import (
	"strings"

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

type Router struct {
	menu          list.Model
	activeFeature AppFeature
	activeModel   tea.Model
	version       string
	width         int
	height        int
	err           error
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

	menu := list.New(items, list.NewDefaultDelegate(), 0, 0)
	menu.SetShowTitle(false)
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
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		menuSize := common.ContentWindowSize(msg, m.menuReservedHeight())
		m.menu.SetSize(menuSize.Width, common.MaxInt(minMenuHeight, menuSize.Height))

		if m.activeModel != nil {
			next, cmd := m.activeModel.Update(m.activeWindowSize())
			m.activeModel = next
			return m, cmd
		}

		return m, nil
	case tea.KeyMsg:
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
		next, cmd := m.activeModel.Update(msg)
		m.activeModel = next
		return m, cmd
	}

	var cmd tea.Cmd
	m.menu, cmd = m.menu.Update(msg)
	return m, cmd
}

func (m Router) View() string {
	if m.activeFeature != nil && m.activeModel != nil {
		return common.App.Render(lipgloss.JoinVertical(
			lipgloss.Left,
			common.Title.Render(appTitle),
			m.activeModel.View(),
		))
	}

	lines := []string{
		m.menuViewHeader(),
		m.menu.View(),
		common.Help.Render(menuHelpText),
	}

	if m.err != nil {
		lines = append(lines, "", common.Error.Render(m.err.Error()))
	}

	return common.App.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m Router) menuViewHeader() string {
	lines := make([]string, 0, menuBaseHeight+1)
	if logo := m.menuLogo(); logo != "" {
		lines = append(lines, common.Accent.Render(logo))
		lines = append(lines, "") // Small gap between logo and version badge
	}

	lines = append(lines,
		m.versionBadge(),
		"",
	)

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m Router) versionBadge() string {
	leftSide := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D9E0EE")). // Clean, soft warm white
		Background(lipgloss.Color("#302D41")). // Elegant dark slate-purple
		Padding(0, 1).
		Render("version")

	rightSide := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#161A22")). // High-contrast dark charcoal
		Background(lipgloss.Color(common.ColorPrimary)). // Theme's primary blue (#5F87FF)
		Padding(0, 1).
		Render(m.version)

	badge := lipgloss.JoinHorizontal(lipgloss.Top, leftSide, rightSide)

	return lipgloss.PlaceHorizontal(
		lipgloss.Width(strings.TrimRight(appLogo, "\r\n")),
		lipgloss.Center,
		badge,
	)
}

func (m Router) menuLogo() string {
	if common.ContentWidth(m.width) < lipgloss.Width(appLogo) {
		return ""
	}
	return strings.TrimRight(appLogo, "\r\n")
}

func (m Router) menuReservedHeight() int {
	reservedHeight := menuBaseHeight
	if m.menuLogo() != "" {
		reservedHeight += strings.Count(strings.TrimRight(appLogo, "\r\n"), "\n") + 1 + 1 // +1 for the gap
	}
	return reservedHeight
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
	return common.ContentWindowSize(tea.WindowSizeMsg{Width: m.width, Height: m.height}, activeTitleHeight)
}

func (m Router) returnToMenu() Router {
	m.activeFeature = nil
	m.activeModel = nil
	return m
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
