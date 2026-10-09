package common

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func numberedLogLines(prefix string, count int) []LogLine {
	lines := make([]LogLine, 0, count)
	for i := 1; i <= count; i++ {
		lines = append(lines, LogLine{Text: fmt.Sprintf("%s%d", prefix, i)})
	}
	return lines
}

func topLogRow(viewer LogViewer) string {
	return strings.TrimSpace(stripANSI(strings.Split(viewer.View(), "\n")[0]))
}

func TestLogViewerScrollsWithinContentAndResetsOnNewSections(t *testing.T) {
	viewer := NewLogViewer()
	viewer.SetSize(40, 10)
	viewer.SetSections([]LogSection{{ID: "a", Title: "A", Lines: numberedLogLines("a", 30)}})

	if got := viewer.Position(); got != "1–10 of 31" {
		t.Fatalf("expected viewer to open at the top, got %q", got)
	}
	for range 3 {
		viewer, _ = viewer.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if got := viewer.Position(); got != "4–13 of 31" {
		t.Fatalf("expected three rows scrolled, got %q", got)
	}
	for range 10 {
		viewer, _ = viewer.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	}
	if got := viewer.Position(); got != "22–31 of 31" {
		t.Fatalf("expected scrolling to stop at the last row, got %q", got)
	}

	viewer.SetSections([]LogSection{{ID: "b", Title: "B", Lines: numberedLogLines("b", 30)}})
	if got := viewer.Position(); got != "1–10 of 31" {
		t.Fatalf("expected new sections to scroll to the top, got %q", got)
	}
}

func TestLogViewerRowsFitWidthAndKeepLongDetailsWhole(t *testing.T) {
	const width = 44
	path := `C:\Users\Infra_IT_Intership_P\AppData\Local\Microsoft\Edge\User Data\Default\Cookies`
	viewer := NewLogViewer()
	viewer.SetSize(width, 20)
	viewer.SetSections([]LogSection{{ID: "deleted", Title: "WOULD DELETE", Lines: []LogLine{
		{Text: "Edge cookies", Detail: path},
		{Text: "GitHub CLI hosts", Detail: "~/.config/gh/hosts.yml"},
	}}})

	view := stripANSI(viewer.View())
	for _, row := range strings.Split(view, "\n") {
		if lipgloss.Width(row) > width {
			t.Fatalf("expected rows <= %d columns, got %d: %q", width, lipgloss.Width(row), row)
		}
	}
	if !strings.Contains(removeWhitespace(view), removeWhitespace(path)) {
		t.Fatalf("expected the whole path to stay visible:\n%s", view)
	}
}

func TestLogViewerUntitledSectionHasNoHeaderRow(t *testing.T) {
	viewer := NewLogViewer()
	viewer.SetSize(40, 10)
	viewer.SetSections([]LogSection{{ID: "run", Lines: numberedLogLines("line", 2)}})

	if got := topLogRow(viewer); got != "line1" {
		t.Fatalf("expected the first line on the first row, got %q", got)
	}
	if got := viewer.Position(); got != "1–2 of 2" {
		t.Fatalf("expected the two lines only, got %q", got)
	}
}

func stripANSI(value string) string {
	return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(value, "")
}
