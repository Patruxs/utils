package views

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"utils/internal/core/cleaner"
)

func TestCleanerLogSectionsGroupResultsAndFoldOnlyNotPresentSkips(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "dev")
	credentials := filepath.Join(home, ".aws", "credentials")
	missing := filepath.Join(home, ".npmrc")
	report := cleaner.Report{Entries: []cleaner.Entry{
		{Level: cleaner.LevelInfo, Message: "Mode: dry-run. No files will be deleted."},
		{Level: cleaner.LevelSkip, Message: ".npmrc not found", Target: "developer credential", Path: missing, NotPresent: true},
		{Level: cleaner.LevelDryRun, Message: "Would delete", Target: "developer credential", Path: credentials},
		{Level: cleaner.LevelSkip, Message: "Refusing to touch developer credential outside current user profile", Target: "developer credential", Path: filepath.Join(home, ".config", "gh", "hosts.yml")},
		{Level: cleaner.LevelWarn, Message: "A target process appears to be running."},
		{Level: cleaner.LevelError, Message: "Could not inspect developer credential"},
	}}

	sections := cleanerLogSections(report, nil, false, home)

	var order []string
	for _, section := range sections {
		order = append(order, section.ID)
	}
	if want := []string{logSectionErrors, logSectionWarnings, logSectionRemovals, logSectionSkipped, logSectionNotes}; strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("expected sections %v, got %v", want, order)
	}

	removals := sections[2]
	if len(removals.Lines) != 1 || removals.Lines[0].Text != "developer credential" || removals.Lines[0].Detail != filepath.Join("~", ".aws", "credentials") {
		t.Fatalf("expected the removal to show its label and home-relative path, got %#v", removals.Lines)
	}

	skipped := sections[3]
	if len(skipped.Folded) != 1 || skipped.Folded[0].Detail != filepath.Join("~", ".npmrc") || skipped.Expanded {
		t.Fatalf("expected the not-present skip folded away, got %#v", skipped)
	}
	if len(skipped.Lines) != 1 || !strings.Contains(skipped.Lines[0].Text, "Refusing to touch") {
		t.Fatalf("expected the refused link to stay visible, got %#v", skipped.Lines)
	}

	quiet := cleanerLogSections(cleaner.Report{Entries: report.Entries[:1]}, nil, false, home)
	if len(quiet) != 1 || quiet[0].ID != logSectionNotes {
		t.Fatalf("expected empty groups to be left out, got %#v", quiet)
	}
}

func TestCleanerResultShowsRunErrorsThatHaveNoLogEntry(t *testing.T) {
	logFailure := errors.New("write cleanup log: permission denied")

	model := finishedCleanerModel(t, cleaner.Report{Entries: []cleaner.Entry{{Level: cleaner.LevelInfo, Message: "Local cleanup finished."}}}, logFailure)

	if view := plainView(model); !strings.Contains(view, "ERRORS") || !strings.Contains(view, "permission denied") {
		t.Fatalf("expected an error without a log entry to be listed under ERRORS:\n%s", view)
	}
}

func TestCleanerSkipKeyRevealsNotPresentTargets(t *testing.T) {
	model := finishedCleanerModel(t, cleaner.Report{Entries: []cleaner.Entry{
		{Level: cleaner.LevelSkip, Message: "not found", Target: "developer credential", Path: "/elsewhere/.npmrc", NotPresent: true},
		{Level: cleaner.LevelSkip, Message: "Refusing to touch a link that leaves home"},
	}}, nil)

	before := plainView(model)
	if strings.Contains(before, "/elsewhere/.npmrc") || !strings.Contains(before, "Refusing to touch") {
		t.Fatalf("expected not-present targets folded and other skips visible:\n%s", before)
	}

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	model = next.(CleanerModel)
	if after := plainView(model); !strings.Contains(after, "/elsewhere/.npmrc") {
		t.Fatalf("expected s to show the not-present targets:\n%s", after)
	}
}

func TestCleanerResultKeepsLongPathsWhole(t *testing.T) {
	const width = 52
	longPath := `C:\Users\Infra_IT_Intership_P\AppData\Local\Microsoft\Edge\User Data\Default\Cookies`
	logPath := `C:\Users\Infra_IT_Intership_P\offboarding-cleanup-20260527-104557.log`
	model := NewCleanerModel()
	next, _ := model.Update(tea.WindowSizeMsg{Width: width, Height: 30})
	next, _ = next.Update(cleanerFinishedMsg{report: cleaner.Report{
		LogPath: logPath,
		Entries: []cleaner.Entry{{Level: cleaner.LevelDryRun, Message: "Would delete", Target: "browser cache", Path: longPath}},
	}})

	view := plainView(next.(CleanerModel))
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > width {
			t.Fatalf("expected lines <= %d columns, got %d:\n%s", width, lipgloss.Width(line), view)
		}
	}
	joined := removeWhitespace(strings.NewReplacer("│", "", "╭", "", "╮", "", "╰", "", "╯", "").Replace(view))
	for _, path := range []string{longPath, logPath} {
		if !strings.Contains(joined, removeWhitespace(path)) {
			t.Fatalf("expected the full path %q to stay visible:\n%s", path, view)
		}
	}
}

func TestCleanerFinishedViewOpensAtTopAndScrollsWithArrowKeys(t *testing.T) {
	model := finishedCleanerModelWithActivities(t, 40)

	before := plainView(model)
	if !strings.Contains(before, "activity 00") || strings.Contains(before, "activity 39") {
		t.Fatalf("expected the finished log to open at the top:\n%s", before)
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Fatal("scrolling the log should not return a command")
	}
	model = next.(CleanerModel)
	if after := plainView(model); after == before || model.optionsList.Selected().ID != optionBrowserProfiles {
		t.Fatalf("expected down to scroll the log instead of moving the options cursor:\n%s", after)
	}
}

func TestCleanerEnterAfterRunReturnsToOptionsSoTheCursorMoves(t *testing.T) {
	model := finishedCleanerModelWithActivities(t, 20)

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(CleanerModel)
	if model.state != StateSelectingOptions || cmd != nil {
		t.Fatalf("expected enter after a finished run to return to options, state=%v", model.state)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	model = next.(CleanerModel)
	if !model.optionsList.Checked(optionCredentialManager) || model.optionsList.Checked(optionBrowserProfiles) {
		t.Fatal("expected down then space after a run to toggle the next option, not the one the cursor was on during the run")
	}
}

func TestCleanerFinishedViewRequiresClickBeforeMouseWheelScroll(t *testing.T) {
	model := finishedCleanerModelWithActivities(t, 40)
	activityY := activityPanelRow(t, model)
	wheel := func(button tea.MouseButton) {
		next, cmd := model.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: button, X: 2, Y: activityY + 2})
		if cmd != nil {
			t.Fatal("mouse wheel should not return a command")
		}
		model = next.(CleanerModel)
	}
	click := func(y int) {
		next, _ := model.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 2, Y: y})
		model = next.(CleanerModel)
	}

	wheel(tea.MouseButtonWheelDown)
	if view := plainView(model); !strings.Contains(view, "activity 00") {
		t.Fatalf("mouse wheel should not scroll before clicking the log:\n%s", view)
	}

	click(activityY + 1)
	wheel(tea.MouseButtonWheelDown)
	scrolled := plainView(model)
	if strings.Contains(scrolled, "activity 00") {
		t.Fatalf("mouse wheel should scroll after clicking the log:\n%s", scrolled)
	}

	click(0)
	wheel(tea.MouseButtonWheelDown)
	if view := plainView(model); view != scrolled {
		t.Fatalf("mouse wheel should stop scrolling after clicking outside the log:\n%s", view)
	}
}

func activityPanelRow(t *testing.T, model CleanerModel) int {
	t.Helper()
	for row, line := range strings.Split(plainView(model), "\n") {
		if strings.Contains(line, "Activity") {
			return row
		}
	}
	t.Fatalf("no Activity panel in view:\n%s", plainView(model))
	return 0
}

func finishedCleanerModelWithActivities(t *testing.T, count int) CleanerModel {
	t.Helper()
	entries := make([]cleaner.Entry, count)
	for i := range entries {
		entries[i] = cleaner.Entry{Level: cleaner.LevelInfo, Message: fmt.Sprintf("activity %02d", i)}
	}
	return finishedCleanerModel(t, cleaner.Report{Entries: entries}, nil)
}

func finishedCleanerModel(t *testing.T, report cleaner.Report, err error) CleanerModel {
	t.Helper()

	model := NewCleanerModel()
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
