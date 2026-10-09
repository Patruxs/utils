package common

import (
	"fmt"
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
	return strings.TrimSpace(stripANSIForCheckboxTest(strings.Split(viewer.View(), "\n")[0]))
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

func TestLogViewerToggleFoldedKeepsTheVisibleRows(t *testing.T) {
	viewer := NewLogViewer()
	viewer.SetSize(40, 5)
	viewer.SetSections([]LogSection{
		{ID: "skipped", Title: "SKIPPED", Lines: numberedLogLines("kept", 2), Folded: numberedLogLines("folded", 20), FoldedSummary: "20 not present"},
		{ID: "notes", Title: "NOTES", Lines: numberedLogLines("note", 10)},
	})
	for range 5 {
		viewer, _ = viewer.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if got := topLogRow(viewer); got != "note2" {
		t.Fatalf("expected note2 at the top before expanding, got %q", got)
	}

	viewer.ToggleFolded("skipped")
	if got := topLogRow(viewer); got != "note2" {
		t.Fatalf("expanding a section above should keep note2 at the top, got %q", got)
	}
	if !strings.HasSuffix(viewer.Position(), "of 34") {
		t.Fatalf("expected folded lines to be added to the content, got %q", viewer.Position())
	}

	viewer.ToggleFolded("skipped")
	if got := topLogRow(viewer); got != "note2" {
		t.Fatalf("folding a section above should keep note2 at the top, got %q", got)
	}

	viewer.ToggleFolded("skipped")
	viewer.SetSections(viewer.sections)
	for range 8 {
		viewer, _ = viewer.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if got := topLogRow(viewer); got != "folded6" {
		t.Fatalf("expected to be inside the folded lines, got %q", got)
	}
	viewer.ToggleFolded("skipped")
	if got := topLogRow(viewer); !strings.HasPrefix(got, "SKIPPED") {
		t.Fatalf("folding the lines on screen should show their section header, got %q", got)
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

	view := stripANSIForCheckboxTest(viewer.View())
	for _, row := range strings.Split(view, "\n") {
		if lipgloss.Width(row) > width {
			t.Fatalf("expected rows <= %d columns, got %d: %q", width, lipgloss.Width(row), row)
		}
	}
	if !strings.Contains(removeWhitespace(view), removeWhitespace(path)) {
		t.Fatalf("expected the whole path to stay visible:\n%s", view)
	}
}
