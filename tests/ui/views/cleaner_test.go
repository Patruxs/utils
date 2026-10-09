package views_test

import (
	"context"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"utils/internal/core/cleaner"
	"utils/internal/ui/views"
)

func TestCleanerViewRendersDefaultControls(t *testing.T) {
	model := views.NewCleanerModel()

	view := stripANSI(model.View())
	for _, want := range []string{
		"[ ] Include browser profiles",
		"Adds full Chrome, Edge, Brave",
		"[ ] Clean Windows Credential Manager allowlist",
		"Always included:",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Choose cleanup mode") || strings.Contains(view, "Run execute cleanup") {
		t.Fatalf("run choices should stay hidden until enter:\n%s", view)
	}
	if model.OwnsKeys() {
		t.Fatal("the options screen should leave esc and q to the router")
	}
}

func TestCleanerViewTogglesOptionsWithArrowSelection(t *testing.T) {
	model := views.NewCleanerModel()

	next, cmd := model.Update(specialKey(tea.KeySpace))
	if cmd != nil {
		t.Fatal("browser profile toggle should not return a command")
	}
	model = next.(views.CleanerModel)

	if view := model.View(); !strings.Contains(view, "[x] Include browser profiles") {
		t.Fatalf("expected browser profile option to be checked:\n%s", view)
	}
	if view := model.View(); strings.Contains(view, "Choose cleanup mode") {
		t.Fatalf("option toggle should not open cleanup mode prompt:\n%s", view)
	}

	next, cmd = model.Update(specialKey(tea.KeyDown))
	if cmd != nil {
		t.Fatal("moving to credential manager option should not return a command")
	}
	model = next.(views.CleanerModel)

	next, cmd = model.Update(specialKey(tea.KeySpace))
	if cmd != nil {
		t.Fatal("credential manager toggle should not return a command")
	}
	model = next.(views.CleanerModel)

	if view := model.View(); !strings.Contains(view, "[x] Clean Windows Credential Manager allowlist") {
		t.Fatalf("expected credential manager option to be checked:\n%s", view)
	}

	next, cmd = model.Update(specialKey(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("opening cleanup mode prompt should not return a command")
	}
	model = next.(views.CleanerModel)

	if view := model.View(); !strings.Contains(view, "Choose cleanup mode for the selected options") {
		t.Fatalf("expected cleanup mode prompt after finishing selection:\n%s", view)
	}
}

func TestCleanerViewOpensCleanupPromptWithBaselineOnly(t *testing.T) {
	model := views.NewCleanerModel()

	next, cmd := model.Update(specialKey(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("opening cleanup mode prompt should not return a command")
	}
	model = next.(views.CleanerModel)

	if view := model.View(); !strings.Contains(view, "Choose cleanup mode") {
		t.Fatalf("expected enter with no options to offer a baseline-only cleanup:\n%s", view)
	}
}

func TestCleanerViewKeepsEachOptionOnItsOwnLine(t *testing.T) {
	model := views.NewCleanerModel()
	labels := []string{"Include SSH keys", "Include browser profiles", "Clean Windows Credential Manager", "Force stop running", "Clean shell and tool history", "Full tool reset"}

	for _, line := range strings.Split(stripANSI(model.View()), "\n") {
		found := 0
		for _, label := range labels {
			if strings.Contains(line, label) {
				found++
			}
		}
		if found > 1 {
			t.Fatalf("expected one option per line, got %q", line)
		}
	}
}

func TestCleanerViewExplainsForceStopOption(t *testing.T) {
	model := views.NewCleanerModel()

	next, _ := model.Update(specialKey(tea.KeyDown))
	model = next.(views.CleanerModel)
	next, _ = model.Update(specialKey(tea.KeyDown))
	model = next.(views.CleanerModel)

	view := stripANSI(model.View())
	for _, want := range []string{
		"[ ] Force stop running target processes",
		"This happens in dry-run too",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("force-stop option missing detail %q:\n%s", want, view)
		}
	}

	next, _ = model.Update(specialKey(tea.KeySpace))
	model = next.(views.CleanerModel)
	next, _ = model.Update(specialKey(tea.KeyUp))
	model = next.(views.CleanerModel)

	view = stripANSI(model.View())
	if !strings.Contains(view, "[x] Force stop running target processes") {
		t.Fatalf("expected force-stop option to remain checked:\n%s", view)
	}

	next, _ = model.Update(specialKey(tea.KeyEnter))
	model = next.(views.CleanerModel)
	if view := stripANSI(model.View()); !strings.Contains(view, "in dry-run too") {
		t.Fatalf("choosing a mode with force stop checked should warn that it also acts in dry-run:\n%s", view)
	}
}

func TestCleanerViewFillsExactlyItsBodyOnEveryScreen(t *testing.T) {
	const width, height = 52, 16
	model := views.NewCleanerModelWithRunner(func(context.Context, cleaner.Options) (cleaner.Report, error) {
		return cleaner.Report{}, nil
	})
	next, _ := model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	model = next.(views.CleanerModel)

	assertFits := func(screen string) {
		t.Helper()
		view := stripANSI(model.View())
		lines := strings.Split(view, "\n")
		if len(lines) != height {
			t.Fatalf("%s: expected %d rows, got %d:\n%s", screen, height, len(lines), view)
		}
		for _, line := range lines {
			if w := lipgloss.Width(line); w > width {
				t.Fatalf("%s: expected lines <= %d columns, got %d:\n%s", screen, width, w, view)
			}
		}
	}
	press := func(msg tea.KeyMsg) tea.Cmd {
		next, cmd := model.Update(msg)
		model = next.(views.CleanerModel)
		return cmd
	}

	assertFits("options")
	press(specialKey(tea.KeySpace))
	press(specialKey(tea.KeyEnter))
	assertFits("mode prompt")
	press(key("e"))
	assertFits("execute confirmation")
	if view := stripANSI(model.View()); !strings.Contains(view, "[ Cancel ]") || !strings.Contains(view, "Run execute cleanup") {
		t.Fatalf("execute confirmation must keep both buttons visible with Cancel focused:\n%s", view)
	}
	press(specialKey(tea.KeyEsc))
	press(specialKey(tea.KeyEnter))
	cmd := press(key("r"))
	assertFits("running")
	for _, msg := range runCommand(cmd) {
		next, _ := model.Update(msg)
		model = next.(views.CleanerModel)
	}
	if model.Running() {
		t.Fatal("expected the run to finish")
	}
	assertFits("result")
}

func TestCleanerViewExecuteConfirmationCanBeCanceledWithArrowSelection(t *testing.T) {
	model := views.NewCleanerModel()

	next, cmd := model.Update(specialKey(tea.KeySpace))
	if cmd != nil {
		t.Fatal("selecting an option should not return a command")
	}
	model = next.(views.CleanerModel)
	next, _ = model.Update(specialKey(tea.KeyEnter))
	model = next.(views.CleanerModel)
	next, _ = model.Update(specialKey(tea.KeyDown))
	model = next.(views.CleanerModel)
	next, cmd = model.Update(specialKey(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("execute mode choice should open confirmation before running")
	}
	model = next.(views.CleanerModel)

	if view := model.View(); !strings.Contains(view, "Execute mode will delete matching local files") {
		t.Fatalf("expected execute confirmation prompt:\n%s", view)
	}

	next, cmd = model.Update(specialKey(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("cancel confirmation should not return a command")
	}
	model = next.(views.CleanerModel)

	if view := model.View(); strings.Contains(view, "Execute mode will delete matching local files") {
		t.Fatalf("expected execute confirmation to be canceled:\n%s", view)
	}
}

func TestCleanerViewStartsDryRunAndExecuteModes(t *testing.T) {
	model := views.NewCleanerModel()

	next, _ := model.Update(specialKey(tea.KeySpace))
	model = next.(views.CleanerModel)
	next, cmd := model.Update(specialKey(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("opening cleanup mode prompt should not return a command")
	}
	model = next.(views.CleanerModel)
	next, cmd = model.Update(specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("dry-run choice should return a command")
	}
	model = next.(views.CleanerModel)

	if view := model.View(); !strings.Contains(view, "Running dry-run cleanup") {
		t.Fatalf("expected dry-run running state:\n%s", view)
	}

	model = views.NewCleanerModel()
	next, _ = model.Update(specialKey(tea.KeySpace))
	model = next.(views.CleanerModel)
	next, _ = model.Update(specialKey(tea.KeyEnter))
	model = next.(views.CleanerModel)
	next, _ = model.Update(specialKey(tea.KeyDown))
	model = next.(views.CleanerModel)
	next, _ = model.Update(specialKey(tea.KeyEnter))
	model = next.(views.CleanerModel)
	next, _ = model.Update(specialKey(tea.KeyDown))
	model = next.(views.CleanerModel)
	next, cmd = model.Update(specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("execute should return a command")
	}
	model = next.(views.CleanerModel)

	if view := model.View(); !strings.Contains(view, "Running execute cleanup") {
		t.Fatalf("expected execute running state:\n%s", view)
	}
}

func TestCleanerViewOptionShortcutKeysAreIgnored(t *testing.T) {
	model := views.NewCleanerModel()

	for _, shortcut := range []string{"b", "c", "r", "e", "x"} {
		next, cmd := model.Update(key(shortcut))
		if cmd != nil {
			t.Fatalf("ignored shortcut %q should not return a command", shortcut)
		}
		model = next.(views.CleanerModel)
	}

	if view := model.View(); strings.Contains(view, "[x] Include browser profiles") {
		t.Fatalf("browser profile shortcut should not toggle option:\n%s", view)
	}
	if view := model.View(); strings.Contains(view, "Choose cleanup mode") || strings.Contains(view, "Running ") {
		t.Fatalf("run shortcuts should be ignored until cleanup prompt is open:\n%s", view)
	}
}

func TestCleanerViewOptionModePromptCanStartDryRun(t *testing.T) {
	model := views.NewCleanerModel()

	next, cmd := model.Update(specialKey(tea.KeySpace))
	if cmd != nil {
		t.Fatal("selecting browser profile option should not return a command")
	}
	model = next.(views.CleanerModel)

	next, cmd = model.Update(specialKey(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("cleanup prompt should not return a command")
	}
	model = next.(views.CleanerModel)

	next, cmd = model.Update(specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("dry-run choice should return a command")
	}
	model = next.(views.CleanerModel)

	if view := model.View(); !strings.Contains(view, "Running dry-run cleanup") {
		t.Fatalf("expected dry-run running state from option mode prompt:\n%s", view)
	}
}

func TestCleanerViewOptionModePromptRoutesExecuteThroughConfirmation(t *testing.T) {
	model := views.NewCleanerModel()

	next, _ := model.Update(specialKey(tea.KeySpace))
	model = next.(views.CleanerModel)
	next, _ = model.Update(specialKey(tea.KeyEnter))
	model = next.(views.CleanerModel)

	next, cmd := model.Update(specialKey(tea.KeyDown))
	if cmd != nil {
		t.Fatal("moving to execute mode choice should not return a command")
	}
	model = next.(views.CleanerModel)

	next, cmd = model.Update(specialKey(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("execute mode choice should open confirmation before running")
	}
	model = next.(views.CleanerModel)

	if view := model.View(); !strings.Contains(view, "Execute mode will delete matching local files") {
		t.Fatalf("expected execute confirmation after choosing execute mode:\n%s", view)
	}

	next, _ = model.Update(specialKey(tea.KeyDown))
	model = next.(views.CleanerModel)
	next, cmd = model.Update(specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("confirmed execute should return a command")
	}
	model = next.(views.CleanerModel)

	if view := model.View(); !strings.Contains(view, "Running execute cleanup") {
		t.Fatalf("expected execute running state after confirmation:\n%s", view)
	}
}

func key(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func specialKey(value tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: value}
}

func stripANSI(value string) string {
	return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(value, "")
}

func runCommand(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		var msgs []tea.Msg
		for _, inner := range msg {
			if inner == nil {
				continue
			}
			if result := inner(); result != nil {
				msgs = append(msgs, result)
			}
		}
		return msgs
	default:
		return []tea.Msg{msg}
	}
}
