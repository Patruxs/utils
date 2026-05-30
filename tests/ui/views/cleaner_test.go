package views_test

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"utils/internal/ui/views"
)

func TestCleanerViewRendersDefaultControls(t *testing.T) {
	model := views.NewCleanerModel()

	view := model.View()
	for _, want := range []string{
		"System & Credential Cleaner",
		"Always included: dev credentials/configs",
		"[ ] Include browser profiles",
		"Adds full Chrome, Edge, Brave",
		"[ ] Clean Windows Credential Manager allowlist",
		"up/down: choose",
		"space: toggle",
		"enter: cleanup",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}

	for _, unwanted := range []string{
		"Actions",
		"Run dry-run cleanup",
		"Run execute cleanup",
		"b browser profiles",
		"c credential manager",
	} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("view should not render option shortcut %q:\n%s", unwanted, view)
		}
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

func TestCleanerViewRequiresSelectedOptionBeforeCleanupPrompt(t *testing.T) {
	model := views.NewCleanerModel()

	next, cmd := model.Update(specialKey(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("enter with no selected options should not return a command")
	}
	model = next.(views.CleanerModel)

	view := model.View()
	if strings.Contains(view, "Choose cleanup mode") || strings.Contains(view, "Running ") {
		t.Fatalf("enter with no selected options should not open cleanup flow:\n%s", view)
	}
	if !strings.Contains(view, "Select at least one cleanup option") {
		t.Fatalf("expected no-selection warning:\n%s", view)
	}

	next, cmd = model.Update(specialKey(tea.KeySpace))
	if cmd != nil {
		t.Fatal("selecting an option after warning should not return a command")
	}
	model = next.(views.CleanerModel)

	if view := model.View(); strings.Contains(view, "Select at least one cleanup option") {
		t.Fatalf("selecting an option should clear no-selection warning:\n%s", view)
	}
}

func TestCleanerViewKeepsFocusedOptionsOnSeparateLines(t *testing.T) {
	model := views.NewCleanerModel()

	view := stripANSI(model.View())
	if strings.Contains(view, "Include browser profiles  [ ] Clean Windows Credential Manager") {
		t.Fatalf("expected focused options to render on separate lines:\n%s", view)
	}

	if !strings.Contains(view, "> [ ] Include browser profiles\n      - Adds full Chrome") {
		t.Fatalf("expected focused browser option to render its details on following lines:\n%s", view)
	}

	if !strings.Contains(view, "\n    [ ] Clean Windows Credential Manager") {
		t.Fatalf("expected credential option to remain on its own line:\n%s", view)
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
		"Stops running Chrome, Edge, Firefox, VS Code, Visual Studio, Claude,",
		"and Codex before cleanup so locked auth/profile files can be handled",
		"This happens in dry-run too",
		"Dry-run still only logs file and",
		"Credential Manager deletions",
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
	if !strings.Contains(view, "This happens in dry-run too") {
		t.Fatalf("checked force-stop option should keep showing its dry-run detail:\n%s", view)
	}
}

func TestCleanerViewWrapsStaticTextOnResize(t *testing.T) {
	model := views.NewCleanerModel()

	next, cmd := model.Update(tea.WindowSizeMsg{Width: 44, Height: 18})
	if cmd != nil {
		t.Fatal("resize should not return a command")
	}
	model = next.(views.CleanerModel)

	view := stripANSI(model.View())
	if strings.Contains(view, "Dry-run-first cleanup for local developer credentials") {
		t.Fatalf("expected subtitle to wrap after resize:\n%s", view)
	}
	if !strings.Contains(view, "Dry-run-first cleanup for local") || !strings.Contains(view, "credentials, shell history") {
		t.Fatalf("expected wrapped subtitle content to remain visible:\n%s", view)
	}
}

func TestCleanerViewWrapsOptionDetailsOnResize(t *testing.T) {
	const width = 52
	model := views.NewCleanerModel()

	next, cmd := model.Update(tea.WindowSizeMsg{Width: width, Height: 20})
	if cmd != nil {
		t.Fatal("resize should not return a command")
	}
	model = next.(views.CleanerModel)
	next, _ = model.Update(specialKey(tea.KeyDown))
	model = next.(views.CleanerModel)
	next, _ = model.Update(specialKey(tea.KeyDown))
	model = next.(views.CleanerModel)

	view := stripANSI(model.View())
	optionsBlock := view
	if _, after, ok := strings.Cut(view, "Options\n"); ok {
		optionsBlock = after
	}
	if beforeSafety, _, ok := strings.Cut(optionsBlock, "Targets stay under"); ok {
		optionsBlock = beforeSafety
	}
	for _, line := range strings.Split(strings.TrimSpace(optionsBlock), "\n") {
		if len([]rune(line)) > width {
			t.Fatalf("expected option detail line <= %d columns, got %d:\n%s", width, len([]rune(line)), view)
		}
	}
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

func TestCleanerViewCanCancelRunningCleanup(t *testing.T) {
	model := views.NewCleanerModel()

	next, _ := model.Update(specialKey(tea.KeySpace))
	model = next.(views.CleanerModel)
	next, _ = model.Update(specialKey(tea.KeyEnter))
	model = next.(views.CleanerModel)
	next, cmd := model.Update(specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("dry-run choice should return a command")
	}
	model = next.(views.CleanerModel)

	next, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatal("canceling running cleanup should not return a command")
	}
	model = next.(views.CleanerModel)

	if view := model.View(); !strings.Contains(view, "Canceling cleanup") {
		t.Fatalf("expected cancel notice while cleanup is still finishing:\n%s", view)
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
