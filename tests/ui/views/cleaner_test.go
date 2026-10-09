package views_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"utils/internal/core/cleaner"
	"utils/internal/ui/common"
	"utils/internal/ui/views"
)

func TestCleanerPreviewUpdatesOnToggleAndNeverDeletes(t *testing.T) {
	home := fakeHome(t)
	credentials := filepath.Join(home, ".aws", "credentials")
	history := filepath.Join(home, ".bash_history")
	writeFile(t, credentials)
	writeFile(t, history)
	core := cleaner.NewCleaner(nil, &fakeCommands{})
	h := newHarness(t, views.NewCleanerModelWith(core.Plan, failingRun(t), failingSave(t)))

	h.do(h.model.Init())
	if view := h.view(); !strings.Contains(view, filepath.Join("~", ".aws", "credentials")) || strings.Contains(view, ".bash_history") {
		t.Fatalf("expected the preview to list the baseline credential only:\n%s", view)
	}

	h.press(specialKey(tea.KeyDown), specialKey(tea.KeyDown))
	for round := range 4 {
		h.press(specialKey(tea.KeySpace))
		shown := strings.Contains(h.view(), filepath.Join("~", ".bash_history"))
		if want := round%2 == 0; shown != want {
			t.Fatalf("after toggle %d expected history in the preview=%v:\n%s", round+1, want, h.view())
		}
	}

	for _, path := range []string{credentials, history} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected the preview to keep %q: %v", path, err)
		}
	}
}

func TestCleanerShowsLoadingAndIgnoresAStaleScan(t *testing.T) {
	planner := func(_ context.Context, opts cleaner.Options) (cleaner.Plan, error) {
		name := "/home/dev/.old-scan"
		if opts.CleanSSHKeys {
			name = "/home/dev/.new-scan"
		}
		return cleaner.Plan{Groups: []cleaner.PlanGroup{{ID: cleaner.GroupCredentials, Entries: []cleaner.PlanEntry{{Path: name}}}}}, nil
	}
	h := newHarness(t, views.NewCleanerModelWith(planner, failingRun(t), failingSave(t)))
	if view := h.view(); !strings.Contains(view, "Scanning") {
		t.Fatalf("expected a loading state before the first scan arrives:\n%s", view)
	}
	stale := h.model.Init()

	h.send(specialKey(tea.KeyDown))
	h.do(h.send(specialKey(tea.KeySpace)))
	h.do(stale)

	if view := h.view(); !strings.Contains(view, ".new-scan") || strings.Contains(view, ".old-scan") {
		t.Fatalf("expected the newer scan to stay on screen:\n%s", view)
	}
}

func TestCleanerForceStopNeverRunsFromThePreview(t *testing.T) {
	fakeHome(t)
	commands := &fakeCommands{}
	core := cleaner.NewCleaner(nil, commands)
	h := newHarness(t, views.NewCleanerModelWith(core.Plan, failingRun(t), failingSave(t)))
	h.do(h.model.Init())

	h.press(specialKey(tea.KeyUp), specialKey(tea.KeySpace))

	running := "code"
	if runtime.GOOS == "windows" {
		running = "Code.exe"
	}
	if view := h.view(); !strings.Contains(view, "WILL FORCE-STOP · 1") || !strings.Contains(view, running) {
		t.Fatalf("expected the preview to list the app it would stop:\n%s", view)
	}
	if len(commands.kills) != 0 {
		t.Fatalf("expected the preview never to stop a process, got %#v", commands.kills)
	}
}

func TestCleanerDryRunLogKeyWritesLogWithoutDeleting(t *testing.T) {
	home := fakeHome(t)
	credentials := filepath.Join(home, ".aws", "credentials")
	writeFile(t, credentials)
	commands := &fakeCommands{}
	core := cleaner.NewCleaner(nil, commands)
	var used cleaner.Options
	run := func(ctx context.Context, opts cleaner.Options) (cleaner.Report, error) {
		used = opts
		return core.Run(ctx, opts)
	}
	h := newHarness(t, views.NewCleanerModelWith(core.Plan, run, failingSave(t)))
	h.do(h.model.Init())
	h.press(specialKey(tea.KeyUp), specialKey(tea.KeySpace))

	h.press(key("d"))

	if used.Execute || used.ForceStopProcesses || len(commands.kills) != 0 {
		t.Fatalf("expected d to run a dry-run without force-stop, got %+v and kills %#v", used, commands.kills)
	}
	if _, err := os.Stat(credentials); err != nil {
		t.Fatalf("expected d to keep the files it lists: %v", err)
	}
	if len(h.notices) != 1 || !strings.Contains(h.notices[0].Text, used.LogPath) || used.LogPath == "" {
		t.Fatalf("expected one notice naming the log path %q, got %#v", used.LogPath, h.notices)
	}
	if _, err := os.Stat(used.LogPath); err != nil || filepath.Dir(used.LogPath) != home {
		t.Fatalf("expected the dry-run log in the profile at %q: %v", used.LogPath, err)
	}
}

func TestCleanerDeleteGoesThroughAConfirmThatDefaultsToCancel(t *testing.T) {
	ran := 0
	run := func(context.Context, cleaner.Options) (cleaner.Report, error) {
		ran++
		return cleaner.Report{}, nil
	}
	h := newHarness(t, views.NewCleanerModelWith(onePathPlan, run, failingSave(t)))
	h.do(h.model.Init())

	for _, cancel := range []tea.KeyMsg{specialKey(tea.KeyEnter), specialKey(tea.KeyEsc), key("n")} {
		h.press(specialKey(tea.KeyEnter))
		modal := stripANSI(h.model.Modal())
		if !h.model.OwnsKeys() || !strings.Contains(modal, "Delete 1 file") || !strings.Contains(modal, "[ Cancel ]") {
			t.Fatalf("expected enter to open the delete confirm with Cancel focused:\n%s", modal)
		}
		h.press(cancel)
		if h.model.Modal() != "" || h.model.Running() || ran != 0 {
			t.Fatalf("expected %q to cancel without running, ran=%d", cancel.String(), ran)
		}
	}

	h.press(specialKey(tea.KeyEnter), key("y"))
	if ran != 1 {
		t.Fatalf("expected y at the confirm to run the cleanup once, ran=%d", ran)
	}
}

func TestCleanerProgressBarReachesTotal(t *testing.T) {
	release := make(chan struct{})
	run := func(_ context.Context, opts cleaner.Options) (cleaner.Report, error) {
		for i := 1; i <= 3; i++ {
			opts.Progress(cleaner.Progress{Done: i, Total: 3, Entry: cleaner.Entry{Level: cleaner.LevelDelete, Path: "/home/dev/f"}})
		}
		<-release
		return cleaner.Report{Deleted: 3}, nil
	}
	h := newHarness(t, views.NewCleanerModelWith(onePathPlan, run, failingSave(t)))
	h.do(h.model.Init())
	h.press(specialKey(tea.KeyEnter))

	next, cmd := h.model.Update(key("y"))
	h.model = next.(views.CleanerModel)
	for range 3 {
		cmd = h.step(cmd)
	}

	view := h.view()
	close(release)
	if !strings.Contains(view, "3/3") || strings.Contains(view, "░") || !strings.Contains(view, "█") {
		t.Fatalf("expected a full progress bar at 3/3:\n%s", view)
	}
	h.do(cmd)
	if h.model.Running() {
		t.Fatal("expected the run to finish")
	}
}

func TestCleanerResultOrdersFailedFirstAndHidesNotPresent(t *testing.T) {
	report := cleaner.Report{Deleted: 1, Entries: []cleaner.Entry{
		{Level: cleaner.LevelInfo, Message: "Starting local offboarding cleanup."},
		{Level: cleaner.LevelSkip, Path: "/elsewhere/.npmrc", NotPresent: true, Message: "not found"},
		{Level: cleaner.LevelDelete, Path: "/elsewhere/.aws/credentials", Message: "Deleted"},
		{Level: cleaner.LevelSkip, Path: "/elsewhere/.config/gh/hosts.yml", Message: "Refusing to touch"},
		{Level: cleaner.LevelError, Path: "/elsewhere/.oci", Message: "Could not fully delete: /elsewhere/.oci: permission denied"},
	}}
	run := func(context.Context, cleaner.Options) (cleaner.Report, error) {
		return report, errors.New("delete failed")
	}
	h := newHarness(t, views.NewCleanerModelWith(onePathPlan, run, failingSave(t)))
	h.send(tea.WindowSizeMsg{Width: 118, Height: 37})
	h.do(h.model.Init())
	h.press(specialKey(tea.KeyEnter), key("y"))

	view := h.view()
	order := []string{"FAILED · 1", "DELETED · 1", "REFUSED · 1", "KEPT", "ⓘ"}
	last := -1
	for _, want := range order {
		at := strings.Index(view, want)
		if at <= last {
			t.Fatalf("expected %q after the previous section in %v:\n%s", want, order, view)
		}
		last = at
	}
	if strings.Contains(view, ".npmrc") || strings.Contains(view, "NOTES") || strings.Contains(view, "Starting local") {
		t.Fatalf("expected no not-present rows and no notes:\n%s", view)
	}
}

func TestCleanerFillsExactlyItsBodyOnEveryScreen(t *testing.T) {
	for _, size := range [][2]int{{48, 7}, {78, 21}, {98, 27}, {118, 37}, {198, 52}} {
		width, height := size[0], size[1]
		release := make(chan struct{})
		run := func(_ context.Context, opts cleaner.Options) (cleaner.Report, error) {
			opts.Progress(cleaner.Progress{Done: 1, Total: 2, Entry: cleaner.Entry{Level: cleaner.LevelDelete, Path: "/home/dev/.aws/credentials"}})
			<-release
			return cleaner.Report{Entries: []cleaner.Entry{{Level: cleaner.LevelDelete, Path: "/home/dev/.aws/credentials"}}}, nil
		}
		h := newHarness(t, views.NewCleanerModelWith(manyPathPlan, run, failingSave(t)))
		h.send(tea.WindowSizeMsg{Width: width, Height: height})
		h.do(h.model.Init())

		assertFits := func(screen, view string) {
			t.Helper()
			lines := strings.Split(stripANSI(view), "\n")
			if len(lines) != height {
				t.Fatalf("%dx%d %s: expected %d rows, got %d:\n%s", width, height, screen, height, len(lines), stripANSI(view))
			}
			for _, line := range lines {
				if w := lipgloss.Width(line); w > width {
					t.Fatalf("%dx%d %s: expected lines <= %d columns, got %d:\n%s", width, height, screen, width, w, stripANSI(view))
				}
			}
		}
		assertFits("scope", h.model.View())
		h.press(key("i"))
		assertFits("scope with info", h.model.View())
		h.press(specialKey(tea.KeyEnter))
		assertFits("confirm", common.Overlay(h.model.View(), h.model.Modal(), width, height))
		next, cmd := h.model.Update(key("y"))
		h.model = next.(views.CleanerModel)
		cmd = h.step(cmd)
		assertFits("running", h.model.View())
		close(release)
		h.do(cmd)
		assertFits("result", h.model.View())
	}
}

func onePathPlan(context.Context, cleaner.Options) (cleaner.Plan, error) {
	return cleaner.Plan{Groups: []cleaner.PlanGroup{{ID: cleaner.GroupCredentials, Entries: []cleaner.PlanEntry{{Path: "/home/dev/.npmrc"}}}}}, nil
}

func manyPathPlan(context.Context, cleaner.Options) (cleaner.Plan, error) {
	var entries []cleaner.PlanEntry
	for _, name := range strings.Split("abcdefghijklmnopqrstuvwxyz", "") {
		entries = append(entries, cleaner.PlanEntry{Path: "/home/dev/.config/tool-" + name + "/a/very/long/path/that/needs/truncating/credentials.json"})
	}
	entries = append(entries, cleaner.PlanEntry{Path: "/home/dev/.config/gh", Refused: cleaner.RefusedOutsideProfile, LinkTarget: "/mnt/elsewhere/gh"})
	return cleaner.Plan{Groups: []cleaner.PlanGroup{
		{ID: cleaner.GroupCredentials, Entries: entries},
		{ID: cleaner.GroupForceStop, Processes: []string{"code", "chrome"}},
	}}, nil
}

type harness struct {
	t       *testing.T
	model   views.CleanerModel
	notices []common.NoticeMsg
}

func newHarness(t *testing.T, model views.CleanerModel) *harness {
	return &harness{t: t, model: model}
}

func (h *harness) send(msg tea.Msg) tea.Cmd {
	next, cmd := h.model.Update(msg)
	h.model = next.(views.CleanerModel)
	return cmd
}

func (h *harness) press(keys ...tea.KeyMsg) {
	for _, msg := range keys {
		h.do(h.send(msg))
	}
}

func (h *harness) do(cmd tea.Cmd) {
	for range 1000 {
		if cmd == nil {
			return
		}
		cmd = h.step(cmd)
	}
	h.t.Fatal("commands did not settle")
}

func (h *harness) step(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		var rest []tea.Cmd
		for _, inner := range msg {
			if next := h.step(inner); next != nil {
				rest = append(rest, next)
			}
		}
		return tea.Batch(rest...)
	case spinner.TickMsg:
		return nil
	case common.NoticeMsg:
		h.notices = append(h.notices, msg)
		return nil
	case nil:
		return nil
	default:
		return h.send(msg)
	}
}

func (h *harness) view() string {
	return stripANSI(h.model.View())
}

type fakeCommands struct {
	kills [][]string
}

func (c *fakeCommands) Output(_ context.Context, name string, _ ...string) ([]byte, error) {
	if name == "tasklist.exe" {
		return []byte("\"Code.exe\",\"5678\",\"Console\",\"1\",\"10,000 K\"\n"), nil
	}
	return nil, nil
}

func (c *fakeCommands) Run(_ context.Context, name string, args ...string) error {
	switch name {
	case "pgrep":
		if len(args) == 2 && args[1] == "code" {
			return nil
		}
		return errors.New("process not found")
	case "pkill", "taskkill":
		c.kills = append(c.kills, append([]string{name}, args...))
	}
	return nil
}

func failingRun(t *testing.T) views.CleanerRunFunc {
	return func(context.Context, cleaner.Options) (cleaner.Report, error) {
		t.Error("expected no cleanup run")
		return cleaner.Report{}, nil
	}
}

func failingSave(t *testing.T) views.CleanerSaveFunc {
	return func(string, cleaner.Report) (string, error) {
		t.Error("expected no report log save")
		return "", nil
	}
}

func fakeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	return home
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
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
