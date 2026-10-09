package router_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"utils/internal/ui"
)

func TestRouterOpensCleanerAndReturnsToMenu(t *testing.T) {
	router := ui.NewRouter()

	model, cmd := router.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if cmd != nil {
		t.Fatal("window resize should not return a command while on menu")
	}
	router = model.(ui.Router)

	model, cmd = router.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("cleaner init should not return a command")
	}
	router = model.(ui.Router)

	if view := router.View(); !strings.Contains(view, "System & Credential Cleaner") || strings.Contains(view, "Network & Diagnostics Manager") {
		t.Fatalf("expected cleaner view after enter, got:\n%s", view)
	}

	model, cmd = router.Update(key("q"))
	if cmd != nil {
		t.Fatal("returning to menu should not return a command")
	}
	router = model.(ui.Router)

	if view := router.View(); !strings.Contains(view, "System & Credential Cleaner") || !strings.Contains(view, "Network & Diagnostics Manager") {
		t.Fatalf("expected menu view after q, got:\n%s", view)
	}
}

func TestRouterMenuRendersFeatureList(t *testing.T) {
	router := ui.NewRouter()
	model, _ := router.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	router = model.(ui.Router)

	view := router.View()
	for _, want := range []string{
		"System & Credential Cleaner",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("menu view missing %q:\n%s", want, view)
		}
	}

	for _, removed := range []string{
		"Infrastructure & Cloud Manager",
		"Application Deployment Helper",
		"Centralized developer and system utilities.",
	} {
		if strings.Contains(view, removed) {
			t.Fatalf("menu view should not render removed feature %q:\n%s", removed, view)
		}
	}
}

func TestRouterMenuRendersLogoAndVersion(t *testing.T) {
	router := ui.NewRouterWithVersion("v1.2.3")
	model, _ := router.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	router = model.(ui.Router)

	view := router.View()
	for _, want := range []string{
		"\u2588\u2588",
		"version",
		"v1.2.3",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("menu view missing %q:\n%s", want, view)
		}
	}
}

func TestRouterQuitKeysReturnCommand(t *testing.T) {
	router := ui.NewRouter()

	_, cmd := router.Update(key("q"))
	if cmd == nil {
		t.Fatal("q on the menu should return a quit command")
	}

	_, cmd = router.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should return a quit command")
	}
}

func TestRouterAcceptsInjectedFeatureRegistry(t *testing.T) {
	router := ui.NewRouter(testFeature{
		title:       "Injected Tool",
		description: "Feature supplied by a registry.",
	})

	model, _ := router.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	router = model.(ui.Router)

	if view := router.View(); !strings.Contains(view, "Injected Tool") {
		t.Fatalf("menu view missing injected feature:\n%s", view)
	}

	model, cmd := router.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("test feature init should not return a command")
	}
	router = model.(ui.Router)

	if view := router.View(); !strings.Contains(view, "Injected Tool View") {
		t.Fatalf("expected injected feature model after enter, got:\n%s", view)
	}
}

func key(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

type testFeature struct {
	title       string
	description string
	rows        int
}

func (f testFeature) Title() string {
	return f.title
}

func (f testFeature) Description() string {
	return f.description
}

func (f testFeature) Model() tea.Model {
	return testModel{view: f.title + " View" + strings.Repeat("\n"+strings.Repeat("x", 300), f.rows)}
}

type testModel struct {
	view  string
	mouse *tea.MouseMsg
}

func (m testModel) Init() tea.Cmd {
	return nil
}

func (m testModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if mouse, ok := msg.(tea.MouseMsg); ok {
		m.mouse = &mouse
	}
	return m, nil
}

func (m testModel) View() string {
	if m.mouse != nil {
		return fmt.Sprintf("mouse at %d,%d", m.mouse.X, m.mouse.Y)
	}
	return m.view
}

func TestRouterFrameFillsTerminalWithViewInsideIt(t *testing.T) {
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 24}, {Width: 120, Height: 40}, {Width: 40, Height: 10}} {
		tall := testFeature{title: "Tall", description: "Tall", rows: 200}
		var router tea.Model = ui.NewRouter(tall)
		router, _ = router.Update(size)
		for _, screen := range []string{"menu", "view"} {
			view := router.View()
			rows := strings.Split(view, "\n")
			if len(rows) != size.Height {
				t.Fatalf("%s at %dx%d: expected %d rows, got %d:\n%s", screen, size.Width, size.Height, size.Height, len(rows), view)
			}
			for _, row := range rows {
				if lipgloss.Width(row) > size.Width {
					t.Fatalf("%s at %dx%d: row wider than the terminal: %q", screen, size.Width, size.Height, row)
				}
			}
			router, _ = router.Update(tea.KeyMsg{Type: tea.KeyEnter})
		}
	}
}

func TestRouterHandsMouseToViewInBodyCoordinates(t *testing.T) {
	var router tea.Model = ui.NewRouter(testFeature{title: "Mouse", description: "Mouse"})
	router, _ = router.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	router, _ = router.Update(tea.KeyMsg{Type: tea.KeyEnter})

	rows := strings.Split(router.View(), "\n")
	firstBodyRow := 0
	for index, row := range rows {
		if strings.Contains(row, "Mouse View") {
			firstBodyRow = index
		}
	}
	router, _ = router.Update(tea.MouseMsg{X: 1, Y: firstBodyRow, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})

	if view := router.View(); !strings.Contains(view, "mouse at 0,0") {
		t.Fatalf("expected a click on the first body row to reach the view at 0,0:\n%s", view)
	}
}
