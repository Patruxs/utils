package router_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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

	if view := router.View(); !strings.Contains(view, "System & Credential Cleaner") || !strings.Contains(view, "up/down: choose") {
		t.Fatalf("expected cleaner view after enter, got:\n%s", view)
	}

	model, cmd = router.Update(key("q"))
	if cmd != nil {
		t.Fatal("returning to menu should not return a command")
	}
	router = model.(ui.Router)

	if view := router.View(); !strings.Contains(view, "version") || !strings.Contains(view, "dev") || !strings.Contains(view, "enter: open") {
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
		"UTILS Developer Hub",
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
}

func (f testFeature) Title() string {
	return f.title
}

func (f testFeature) Description() string {
	return f.description
}

func (f testFeature) Model() tea.Model {
	return testModel{view: f.title + " View"}
}

type testModel struct {
	view string
}

func (m testModel) Init() tea.Cmd {
	return nil
}

func (m testModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m testModel) View() string {
	return m.view
}
