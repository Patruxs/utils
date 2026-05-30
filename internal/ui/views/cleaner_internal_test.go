package views

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"utils/internal/core/cleaner"
)

func TestRenderActivityWrapsLongPathsToViewportWidth(t *testing.T) {
	const width = 52
	longPath := `C:\Users\Infra_IT_Intership_P\AppData\Local\Microsoft\Edge\User Data\Default\Cookies`
	report := cleaner.Report{
		Entries: []cleaner.Entry{
			{
				Level:   cleaner.LevelDryRun,
				Message: "Would delete browser cache: " + longPath,
			},
		},
	}

	view := stripANSIForCleanerTest(renderActivity(report, width))
	for _, line := range strings.Split(view, "\n") {
		if len([]rune(line)) > width {
			t.Fatalf("expected wrapped activity line <= %d columns, got %d:\n%s", width, len([]rune(line)), view)
		}
	}

	if !strings.Contains(removeWhitespace(view), removeWhitespace(longPath)) {
		t.Fatalf("expected wrapped activity to preserve full path %q:\n%s", longPath, view)
	}
}

func TestLogViewerWrapsLogPathToViewportWidth(t *testing.T) {
	const width = 44
	longPath := `C:\Users\Infra_IT_Intership_P\offboarding-cleanup-20260527-104557.log`
	report := cleaner.Report{LogPath: longPath}

	viewer := NewLogViewerModel()
	viewer.SetSize(width, 10)
	viewer.SetReport(report)

	view := stripANSIForCleanerTest(viewer.View())
	logBlock := view
	if beforeStats, _, ok := strings.Cut(view, "  dry-run:"); ok {
		logBlock = beforeStats
	}

	for _, line := range strings.Split(strings.TrimSpace(logBlock), "\n") {
		if len([]rune(line)) > width {
			t.Fatalf("expected wrapped log path line <= %d columns, got %d:\n%s", width, len([]rune(line)), view)
		}
	}

	if !strings.Contains(removeWhitespace(logBlock), removeWhitespace(longPath)) {
		t.Fatalf("expected wrapped log block to preserve full path %q:\n%s", longPath, view)
	}
}

func TestCleanerFinishedViewScrollsRecentActivityWithArrowKeys(t *testing.T) {
	model := finishedCleanerModelWithActivities(t, 20)

	before := stripANSIForCleanerTest(model.View())
	if !strings.Contains(before, "activity 19") {
		t.Fatalf("expected finished activity to start at the bottom:\n%s", before)
	}
	if strings.Contains(before, "scroll activity") {
		t.Fatalf("finished view should not render scroll activity help text:\n%s", before)
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyUp})
	if cmd != nil {
		t.Fatal("scrolling recent activity should not return a command")
	}
	model = next.(CleanerModel)

	after := stripANSIForCleanerTest(model.View())
	if !strings.Contains(after, "activity 14") || strings.Contains(after, "activity 19") {
		t.Fatalf("expected up arrow to scroll recent activity instead of changing options:\n%s", after)
	}
}

func TestCleanerFinishedViewRequiresClickBeforeMouseWheelScroll(t *testing.T) {
	model := finishedCleanerModelWithActivities(t, 20)

	next, cmd := model.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp, X: 1, Y: 20})
	if cmd != nil {
		t.Fatal("unfocused mouse wheel should not return a command")
	}
	model = next.(CleanerModel)
	if view := stripANSIForCleanerTest(model.View()); !strings.Contains(view, "activity 19") || strings.Contains(view, "activity 12") {
		t.Fatalf("mouse wheel should not scroll before clicking recent activity:\n%s", view)
	}

	view := model.View()
	recentActivityY := strings.Count(view[:strings.Index(view, "Recent activity\n")], "\n")
	next, cmd = model.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 1, Y: recentActivityY})
	if cmd != nil {
		t.Fatal("clicking recent activity should not return a command")
	}
	model = next.(CleanerModel)

	next, cmd = model.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp, X: 1, Y: recentActivityY + 1})
	if cmd != nil {
		t.Fatal("focused mouse wheel should not return a command")
	}
	model = next.(CleanerModel)
	if view := stripANSIForCleanerTest(model.View()); !strings.Contains(view, "activity 12") || strings.Contains(view, "activity 19") {
		t.Fatalf("mouse wheel should scroll after clicking recent activity:\n%s", view)
	}

	next, cmd = model.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 1, Y: 0})
	if cmd != nil {
		t.Fatal("clicking outside recent activity should not return a command")
	}
	model = next.(CleanerModel)

	next, cmd = model.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown, X: 1, Y: recentActivityY + 1})
	if cmd != nil {
		t.Fatal("unfocused mouse wheel should not return a command after clicking outside")
	}
	model = next.(CleanerModel)
	if view := stripANSIForCleanerTest(model.View()); !strings.Contains(view, "activity 12") || strings.Contains(view, "activity 19") {
		t.Fatalf("mouse wheel should stop scrolling after clicking outside recent activity:\n%s", view)
	}
}

func finishedCleanerModelWithActivities(t *testing.T, count int) CleanerModel {
	t.Helper()

	model := NewCleanerModel()
	next, cmd := model.Update(tea.WindowSizeMsg{Width: 88, Height: 23})
	if cmd != nil {
		t.Fatal("resize should not return a command")
	}
	model = next.(CleanerModel)

	entries := make([]cleaner.Entry, count)
	for i := range entries {
		entries[i] = cleaner.Entry{
			Level:   cleaner.LevelInfo,
			Message: fmt.Sprintf("activity %02d", i),
		}
	}

	next, cmd = model.Update(cleanerFinishedMsg{report: cleaner.Report{Entries: entries}})
	if cmd != nil {
		t.Fatal("finished cleanup message should not return a command")
	}

	return next.(CleanerModel)
}

func stripANSIForCleanerTest(value string) string {
	return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(value, "")
}

func removeWhitespace(value string) string {
	return strings.Join(strings.Fields(value), "")
}
