package views

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"utils/internal/core/cleaner"
)

func TestCleanerResultShowsRunErrorsThatHaveNoLogEntry(t *testing.T) {
	logFailure := errors.New("write cleanup log: permission denied")

	model := finishedCleanerModel(t, cleaner.Report{Entries: []cleaner.Entry{{Level: cleaner.LevelInfo, Message: "Local cleanup finished."}}}, logFailure)

	if view := plainView(model); !strings.Contains(view, "FAILED") || !strings.Contains(view, "permission denied") {
		t.Fatalf("expected an error without a log entry to be listed under FAILED:\n%s", view)
	}
}

func TestCleanerResultKeepsLongPathsWhole(t *testing.T) {
	const width = 52
	longPath := `C:\Users\Infra_IT_Intership_P\AppData\Local\Microsoft\Edge\User Data\Default\Cookies`
	model := NewCleanerModel()
	next, _ := model.Update(tea.WindowSizeMsg{Width: width, Height: 30})
	next, _ = next.Update(cleanerFinishedMsg{report: cleaner.Report{
		Entries: []cleaner.Entry{{Level: cleaner.LevelDelete, Message: "Deleted", Target: "browser cache", Path: longPath}},
	}})

	view := plainView(next.(CleanerModel))
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > width {
			t.Fatalf("expected lines <= %d columns, got %d:\n%s", width, lipgloss.Width(line), view)
		}
	}
	joined := removeWhitespace(strings.NewReplacer("│", "", "╭", "", "╮", "", "╰", "", "╯", "").Replace(view))
	if !strings.Contains(joined, removeWhitespace(longPath)) {
		t.Fatalf("expected the full path %q to stay visible:\n%s", longPath, view)
	}
}

func TestCleanerResultOpensAtTopAndScrollsWithArrowKeys(t *testing.T) {
	model := finishedCleanerModelWithDeletions(t, 40)

	before := plainView(model)
	if !strings.Contains(before, "file-00") || strings.Contains(before, "file-39") {
		t.Fatalf("expected the result to open at the top:\n%s", before)
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Fatal("scrolling the result should not return a command")
	}
	model = next.(CleanerModel)
	if after := plainView(model); after == before || model.cursor != 0 {
		t.Fatalf("expected down to scroll the result instead of moving the scope cursor:\n%s", after)
	}
}

func TestCleanerEnterAfterRunReturnsToScopeAndRescans(t *testing.T) {
	model := finishedCleanerModelWithDeletions(t, 20)

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(CleanerModel)
	if model.state != cleanerScope || cmd == nil || !model.scanning {
		t.Fatalf("expected enter after a run to return to the scope and rescan, state=%v", model.state)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	model = next.(CleanerModel)
	if !model.options.CleanSSHKeys {
		t.Fatal("expected down then space after a run to toggle the next scope row")
	}
}

func finishedCleanerModelWithDeletions(t *testing.T, count int) CleanerModel {
	t.Helper()
	entries := make([]cleaner.Entry, count)
	for i := range entries {
		entries[i] = cleaner.Entry{Level: cleaner.LevelDelete, Message: "Deleted", Path: fmt.Sprintf("/elsewhere/file-%02d", i)}
	}
	return finishedCleanerModel(t, cleaner.Report{Entries: entries, Deleted: count}, nil)
}

func finishedCleanerModel(t *testing.T, report cleaner.Report, err error) CleanerModel {
	t.Helper()

	model := NewCleanerModelWith(func(context.Context, cleaner.Options) (cleaner.Plan, error) {
		return cleaner.Plan{}, nil
	}, nil, nil)
	next, cmd := model.Update(tea.WindowSizeMsg{Width: 88, Height: 21})
	if cmd != nil {
		t.Fatal("resize should not return a command")
	}
	next, cmd = next.Update(cleanerFinishedMsg{report: report, err: err})
	if cmd != nil {
		t.Fatal("finished cleanup message should not return a command")
	}
	return next.(CleanerModel)
}

func plainView(model CleanerModel) string {
	return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(model.View(), "")
}

func removeWhitespace(value string) string {
	return strings.Join(strings.Fields(value), "")
}
