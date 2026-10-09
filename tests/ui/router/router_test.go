package router_test

import (
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"utils/internal/ui"
	"utils/internal/ui/common"
)

func TestRouterSwitchesToolsAndKeepsEachViewState(t *testing.T) {
	var router tea.Model = ui.NewRouter(stubFeature("Alpha"), stubFeature("Beta"))
	router, _ = router.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	router, _ = router.Update(key("x"))
	for _, step := range []struct {
		key  tea.KeyMsg
		want string
	}{
		{tea.KeyMsg{Type: tea.KeyTab}, "Beta keys: activated"},
		{key("y"), "Beta keys: activated y"},
		{key("1"), "Alpha keys: x activated"},
		{key("2"), "Beta keys: activated y activated"},
	} {
		router, _ = router.Update(step.key)
		if view := router.View(); !strings.Contains(view, step.want) {
			t.Fatalf("after %q expected %q:\n%s", step.key, step.want, view)
		}
	}
}

func TestRouterQuitKeysReturnCommand(t *testing.T) {
	router := ui.NewRouter()

	if _, cmd := router.Update(key("q")); cmd == nil {
		t.Fatal("q should return a quit command")
	}
	if _, cmd := router.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil {
		t.Fatal("ctrl+c should return a quit command")
	}
}

func TestRouterHelpOverlayOpensAndCloses(t *testing.T) {
	var router tea.Model = ui.NewRouterWithVersion("v1.2.3", stubFeature("Alpha"))
	router, _ = router.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if view := router.View(); !strings.Contains(view, "v1.2.3") || strings.Contains(view, "\u2588\u2588") {
		t.Fatalf("expected the version in the header and no banner before ?:\n%s", view)
	}

	router, _ = router.Update(key("?"))
	if view := router.View(); !strings.Contains(view, "\u2588\u2588") || !strings.Contains(view, "switch tool") {
		t.Fatalf("expected ? to open the help overlay with the banner and keys:\n%s", view)
	}
	router, _ = router.Update(key("x"))
	router, _ = router.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if view := router.View(); strings.Contains(view, "\u2588\u2588") || !strings.Contains(view, "Alpha keys:") || strings.Contains(view, "Alpha keys: x") {
		t.Fatalf("expected esc to close the overlay without handing keys to the view:\n%s", view)
	}
}

func TestRouterRefusesToolSwitchWhileRunning(t *testing.T) {
	busy := stubFeature("Busy")
	busy.model.running = true
	var router tea.Model = ui.NewRouter(busy, stubFeature("Other"))
	router, _ = router.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	for _, msg := range []tea.KeyMsg{{Type: tea.KeyTab}, key("2"), key("q"), key("?")} {
		router, _ = router.Update(msg)
	}
	view := router.View()
	if !strings.Contains(view, "Busy keys: q ?") || strings.Contains(view, "Other keys") {
		t.Fatalf("expected the running view to stay active and receive q and ?:\n%s", view)
	}
	router, _ = router.Update(key("2"))
	if view := router.View(); !strings.Contains(view, "A run is in progress") {
		t.Fatalf("expected a notice that switching is refused:\n%s", view)
	}
}

func TestRouterFrameFillsTerminalWithViewInsideIt(t *testing.T) {
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 24}, {Width: 120, Height: 40}, {Width: 50, Height: 10}, {Width: 40, Height: 8}} {
		tall := stubFeature("Tall")
		tall.model.rows = 200
		var router tea.Model = ui.NewRouter(tall, stubFeature("Other"))
		router, _ = router.Update(size)
		for _, screen := range []string{"view", "help"} {
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
			router, _ = router.Update(key("?"))
		}
	}
}

func TestRouterFrameKeepsItsHeightWithAnOverlongNotice(t *testing.T) {
	var router tea.Model = ui.NewRouter(stubFeature("Alpha"))
	router, _ = router.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	router, _ = router.Update(common.NoticeMsg{Tone: common.ToneDanger, Text: "Could not save the log: write cleanup log: open\n/home/pat/" + strings.Repeat("deep/", 20) + "offboarding-cleanup.log: read-only file system"})

	rows := strings.Split(router.View(), "\n")
	if len(rows) != 24 || !strings.Contains(rows[0], "UTILS") {
		t.Fatalf("expected 24 rows with the header on top, got %d:\n%s", len(rows), strings.Join(rows, "\n"))
	}
	for _, row := range rows {
		if lipgloss.Width(row) > 80 {
			t.Fatalf("row wider than the terminal: %q", row)
		}
	}
	if footer := rows[len(rows)-1]; !strings.Contains(footer, "Could not") || !strings.Contains(footer, "read-only file system") {
		t.Fatalf("expected the notice to keep its head and its tail on one line: %q", footer)
	}
}

func TestRouterHandsMouseToViewInBodyCoordinates(t *testing.T) {
	var router tea.Model = ui.NewRouter(stubFeature("Mouse"))
	router, _ = router.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	rows := strings.Split(router.View(), "\n")
	firstBodyRow := 0
	for index, row := range rows {
		if strings.Contains(row, "Mouse keys:") {
			firstBodyRow = index
		}
	}
	router, _ = router.Update(tea.MouseMsg{X: 1, Y: firstBodyRow, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})

	if view := router.View(); !strings.Contains(view, "mouse 0,0") {
		t.Fatalf("expected a click on the first body row to reach the view at 0,0:\n%s", view)
	}
}

func key(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

type testFeature struct {
	title string
	model stubModel
}

func stubFeature(title string) testFeature {
	return testFeature{title: title, model: stubModel{name: title}}
}

func (f testFeature) Title() string    { return f.title }
func (f testFeature) Model() tea.Model { return f.model }

type stubModel struct {
	name    string
	events  []string
	rows    int
	running bool
}

func (m stubModel) Init() tea.Cmd { return nil }

func (m stubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.events = append(m.events, msg.String())
	case common.ActivatedMsg:
		m.events = append(m.events, "activated")
	case tea.MouseMsg:
		m.events = append(m.events, "mouse "+strconv.Itoa(msg.X)+","+strconv.Itoa(msg.Y))
	}
	return m, nil
}

func (m stubModel) View() string {
	return m.name + " keys: " + strings.Join(m.events, " ") + strings.Repeat("\n"+strings.Repeat("x", 300), m.rows)
}

func (m stubModel) Running() bool { return m.running }
