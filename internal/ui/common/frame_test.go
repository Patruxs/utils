package common

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

type testKeyMap []key.Binding

func (k testKeyMap) ShortHelp() []key.Binding  { return k }
func (k testKeyMap) FullHelp() [][]key.Binding { return [][]key.Binding{k} }

func TestFrameComponentsNeverExceedTheirWidth(t *testing.T) {
	keys := testKeyMap{
		key.NewBinding(key.WithKeys("up"), key.WithHelp("↑↓", "move")),
		key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "run selected actions")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back to the main menu")),
	}
	body := "Execute removes local sign-ins, cookies/sessions, saved passwords, extensions 日本語 and bookmarks.\n\nshort"

	for _, width := range []int{4, 9, 20, 39, 60, 78, 118} {
		rendered := map[string]string{
			"panel":   Panel{Title: "Include browser profiles and caches", Meta: "1/29 · 0 checked", Variant: PanelFocused, Width: width, Height: 4}.Render(body),
			"danger":  ConfirmDialog(width, "Change system settings", []string{"• " + body}, []string{"Cancel", "Run these changes"}, 0),
			"footer":  Footer(width, keys, Pill("DRY-RUN", ToneAccent)),
			"header":  Header(width, 24, []string{"Network & Diagnostics Manager", "Hosts: Add Entry"}, "v1.2.3"),
			"running": RunStatus(width, "⣾", "Running: Run Network Diagnostics", body, 0),
			"counts":  RenderCounts(width, []Count{{Label: "would delete", N: 7}, {Label: "skipped", N: 42}, {Label: "errors"}}),
			"notice":  Notice(width, ToneWarning, body),
		}
		for name, view := range rendered {
			for _, line := range strings.Split(view, "\n") {
				if lipgloss.Width(line) > width {
					t.Fatalf("%s at width %d has a %d-column line: %q", name, width, lipgloss.Width(line), line)
				}
			}
		}
		if lines := strings.Split(rendered["panel"], "\n"); len(lines) != 4 || lipgloss.Width(lines[0]) != width {
			t.Fatalf("panel at width %d should fill exactly %dx4, got:\n%s", width, width, rendered["panel"])
		}
	}
}

func TestTooSmallBelowMinimumSize(t *testing.T) {
	for _, size := range []struct {
		width, height int
		tooSmall      bool
	}{
		{39, 10, true},
		{40, 9, true},
		{40, 10, false},
	} {
		message, tooSmall := TooSmall(size.width, size.height)
		if tooSmall != size.tooSmall {
			t.Fatalf("TooSmall(%d, %d) = %v, want %v", size.width, size.height, tooSmall, size.tooSmall)
		}
		for _, line := range strings.Split(message, "\n") {
			if lipgloss.Width(line) > size.width {
				t.Fatalf("too-small message line wider than %d: %q", size.width, line)
			}
		}
	}
}
