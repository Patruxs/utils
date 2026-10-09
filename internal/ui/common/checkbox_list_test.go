package common

import (
	"regexp"
	"strings"
	"testing"
)

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
