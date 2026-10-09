package common

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func groupedCheckboxItems(count int) []CheckboxItem {
	items := make([]CheckboxItem, 0, count)
	for i := 1; i <= count; i++ {
		items = append(items, CheckboxItem{
			ID:    fmt.Sprintf("item%d", i),
			Label: fmt.Sprintf("Action number %d with a label long enough to need cutting", i),
			Tag:   "● writes",
			Group: fmt.Sprintf("group %d", (i-1)/5),
		})
	}
	return items
}

func TestCheckboxListKeepsCursorInWindowAndCountsHiddenItems(t *testing.T) {
	const height = 8
	items := groupedCheckboxItems(29)
	model := NewCheckboxList(items, 70, height)
	model.SetHideDetails(true)
	model.SetFocused(true)

	for step := 0; step < 2*len(items); step++ {
		if step < len(items) {
			model = model.MoveDown()
		} else {
			model = model.MoveUp()
		}
		view := stripANSIForCheckboxTest(model.View())
		rows := strings.Split(view, "\n")
		if len(rows) > height {
			t.Fatalf("step %d: expected at most %d rows, got %d:\n%s", step, height, len(rows), view)
		}
		if !strings.Contains(view, "▸ [ ] Action number "+strings.TrimPrefix(model.Selected().ID, "item")+" ") {
			t.Fatalf("step %d: cursor on %s is not visible:\n%s", step, model.Selected().ID, view)
		}

		shown := strings.Count(view, "Action number")
		above, below := 0, 0
		if strings.HasPrefix(rows[0], "↑ ") {
			fmt.Sscanf(rows[0], "↑ %d more", &above)
		}
		if last := rows[len(rows)-1]; strings.HasPrefix(last, "↓ ") {
			fmt.Sscanf(last, "↓ %d more", &below)
		}
		if above+shown+below != len(items) {
			t.Fatalf("step %d: %d above + %d shown + %d below != %d items:\n%s", step, above, shown, below, len(items), view)
		}
	}
}

func TestCheckboxListRowsFitWidth(t *testing.T) {
	for _, width := range []int{40, 80} {
		model := NewCheckboxList(groupedCheckboxItems(6), width, 20)
		model.SetHideDetails(true)
		model.SetFocused(true)
		model, _ = model.SetChecked("item2", true)

		for _, row := range strings.Split(stripANSIForCheckboxTest(model.View()), "\n") {
			if lipgloss.Width(row) > width {
				t.Fatalf("expected rows <= %d columns, got %d: %q", width, lipgloss.Width(row), row)
			}
		}
	}
}

func TestCheckboxListRendersFocusedAndCheckedDetails(t *testing.T) {
	const width = 40
	model := NewCheckboxList([]CheckboxItem{
		{
			ID:    "one",
			Label: "One",
			Details: []string{
				"Explains exactly what this option will do before the user runs it.",
			},
		},
		{ID: "two", Label: "Two"},
	}, width, 4)
	model.SetFocused(true)

	view := stripANSIForCheckboxTest(model.View())
	if !strings.Contains(view, "- Explains exactly what this") || !strings.Contains(view, "option will do before the user") {
		t.Fatalf("focused option should render details:\n%s", view)
	}
	assertCheckboxLinesFit(t, view, width)

	model, _ = model.SetChecked("one", true)
	model.SetFocused(false)

	view = stripANSIForCheckboxTest(model.View())
	if !strings.Contains(view, "- Explains exactly what this") || !strings.Contains(view, "option will do before the user") {
		t.Fatalf("checked option should keep rendering details when focus leaves:\n%s", view)
	}
	assertCheckboxLinesFit(t, view, width)
}

func assertCheckboxLinesFit(t *testing.T, view string, width int) {
	t.Helper()

	for _, line := range strings.Split(view, "\n") {
		if len([]rune(line)) > width {
			t.Fatalf("expected checkbox line <= %d columns, got %d:\n%s", width, len([]rune(line)), view)
		}
	}
}

func stripANSIForCheckboxTest(value string) string {
	return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(value, "")
}
