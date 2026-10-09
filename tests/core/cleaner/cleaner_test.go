package cleaner_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"utils/internal/core/cleaner"
)

func TestRunDryRunKeepsExistingTargetsWithoutWritingDefaultLog(t *testing.T) {
	home := fakeHome(t)
	target := filepath.Join(home, ".aws", "credentials")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := cleaner.Run(context.Background(), cleaner.Options{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected dry-run to keep existing target: %v", err)
	}
	if report.DryRuns == 0 {
		t.Fatalf("expected at least one dry-run entry, got %#v", report)
	}
	if report.Deleted != 0 {
		t.Fatalf("dry-run should not delete files, deleted=%d", report.Deleted)
	}
	if report.LogPath != "" {
		t.Fatalf("expected no default log path, got %q", report.LogPath)
	}
	assertNoDefaultCleanupLogs(t, home)
}

func TestRunExecuteDeletesExistingTargetsUnderFakeHome(t *testing.T) {
	home := fakeHome(t)
	target := filepath.Join(home, ".kube", "cache")
	codexAuth := filepath.Join(home, ".codex", "auth.json")
	codexSessions := filepath.Join(home, ".codex", "sessions", "history.jsonl")
	shellHistory := filepath.Join(home, ".bash_history")
	browserCache := filepath.Join(home, ".cache", "chromium", "data")
	if runtime.GOOS == "windows" {
		browserCache = filepath.Join(home, "AppData", "Local", "Google", "Chrome", "User Data", "Default", "Cache", "data")
	}
	for _, file := range []string{filepath.Join(target, "token"), codexAuth, codexSessions, shellHistory, browserCache} {
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("secret"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	report, err := cleaner.Run(context.Background(), cleaner.Options{Execute: true})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected target to be deleted, stat err=%v", err)
	}
	if _, err := os.Stat(codexAuth); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected baseline to delete the AI tool credential file, stat err=%v", err)
	}
	for _, kept := range []string{codexSessions, shellHistory, browserCache} {
		if _, err := os.Stat(kept); err != nil {
			t.Fatalf("expected baseline to keep non-credential file %q, stat err=%v", kept, err)
		}
	}

	if _, err := cleaner.Run(context.Background(), cleaner.Options{Execute: true, FullToolReset: true, CleanShellHistory: true, IncludeBrowserProfiles: true}); err != nil {
		t.Fatalf("Run with opt-in options returned error: %v", err)
	}
	for _, removed := range []string{filepath.Dir(codexAuth), shellHistory, browserCache} {
		if _, err := os.Stat(removed); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected opt-in options to delete %q, stat err=%v", removed, err)
		}
	}
	if report.Deleted == 0 {
		t.Fatalf("expected at least one delete entry, got %#v", report)
	}
	if report.LogPath != "" {
		t.Fatalf("expected no default log path, got %q", report.LogPath)
	}
	assertNoDefaultCleanupLogs(t, home)
}

func TestRunExecuteRefusesTargetBehindLinkThatLeavesHome(t *testing.T) {
	home := fakeHome(t)
	outside := t.TempDir()
	outsideToken := filepath.Join(outside, "gh", "hosts.yml")
	if err := os.MkdirAll(filepath.Dir(outsideToken), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsideToken, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(home, ".config")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	report, _ := cleaner.Run(context.Background(), cleaner.Options{Execute: true})

	if _, err := os.Stat(outsideToken); err != nil {
		t.Fatalf("expected file outside home to survive, stat err=%v", err)
	}
	if !hasEntry(report, cleaner.LevelSkip, outside) {
		t.Fatalf("expected a SKIP entry naming %q, got %#v", outside, report.Entries)
	}
}

func TestRunExecuteRemovesSymlinkAndReportsTargetKept(t *testing.T) {
	realHome := fakeHome(t)
	linkedHome := filepath.Join(t.TempDir(), "home")
	if err := os.Symlink(realHome, linkedHome); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	t.Setenv("HOME", linkedHome)
	t.Setenv("USERPROFILE", linkedHome)

	dotfile := filepath.Join(realHome, "dotfiles", "npmrc")
	if err := os.MkdirAll(filepath.Dir(dotfile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dotfile, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(realHome, ".npmrc")
	if err := os.Symlink(dotfile, link); err != nil {
		t.Fatal(err)
	}

	report, err := cleaner.Run(context.Background(), cleaner.Options{Execute: true})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if _, err := os.Lstat(link); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected symlink to be removed, lstat err=%v", err)
	}
	if _, err := os.Stat(dotfile); err != nil {
		t.Fatalf("expected symlink target to be kept, stat err=%v", err)
	}
	if !hasEntry(report, cleaner.LevelDelete, "symlink") || !hasEntry(report, cleaner.LevelDelete, dotfile) {
		t.Fatalf("expected a delete entry naming the symlink and its kept target, got %#v", report.Entries)
	}
}

func TestRunExecuteReportsPartialDeletion(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX directory permissions enforced for the current user")
	}
	home := fakeHome(t)
	cache := filepath.Join(home, ".aws", "sso", "cache")
	token := filepath.Join(cache, "token.json")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(token, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(cache), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(filepath.Dir(cache), 0o700) })

	report, err := cleaner.Run(context.Background(), cleaner.Options{Execute: true})
	if err == nil {
		t.Fatal("expected Run to return the removal error")
	}

	if _, err := os.Stat(token); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected the file inside to be removed, stat err=%v", err)
	}
	if !hasEntry(report, cleaner.LevelError, "Could not fully delete") {
		t.Fatalf("expected the failure to be reported as partial, got %#v", report.Entries)
	}
}

func TestRunRejectsLogPathOutsideUserHome(t *testing.T) {
	home := fakeHome(t)
	outsideLog := filepath.Join(t.TempDir(), "cleanup.log")

	_, err := cleaner.Run(context.Background(), cleaner.Options{LogPath: outsideLog})
	if err == nil {
		t.Fatal("expected log path outside fake home to be rejected")
	}
	if !strings.Contains(err.Error(), "outside current user profile") {
		t.Fatalf("expected outside-home error, got %v", err)
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatalf("fake home should still exist: %v", err)
	}
}

func TestRunHonorsCustomLogPathUnderUserHome(t *testing.T) {
	home := fakeHome(t)
	logPath := filepath.Join(home, "logs", "cleanup.log")

	report, err := cleaner.Run(context.Background(), cleaner.Options{LogPath: logPath})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if report.LogPath != logPath {
		t.Fatalf("expected log path %q, got %q", logPath, report.LogPath)
	}
	assertLogContains(t, logPath, "Local cleanup finished.")
}

func TestRunStopsBetweenStagesWhenCanceled(t *testing.T) {
	home := fakeHome(t)
	target := filepath.Join(home, ".npmrc")
	if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	commands := newFakeProcessCommandRunner()
	commands.onFirstCall = cancel

	report, err := cleaner.NewCleaner(nil, commands).Run(ctx, cleaner.Options{Execute: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if n := strings.Count(err.Error(), context.Canceled.Error()); n != 1 {
		t.Fatalf("expected the cancel error once, got %d times: %v", n, err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected canceled cleanup to leave target: %v", err)
	}

	canceledEntries := 0
	for _, entry := range report.Entries {
		if strings.Contains(entry.Message, context.Canceled.Error()) {
			canceledEntries++
		}
		if entry.Message == "Local cleanup finished." {
			t.Fatalf("canceled cleanup should not report finishing, got %#v", report.Entries)
		}
	}
	if canceledEntries != 1 {
		t.Fatalf("expected one canceled entry, got %d in %#v", canceledEntries, report.Entries)
	}
}

func TestRunWarnsAboutTargetProcessesWithoutForceStop(t *testing.T) {
	home := fakeHome(t)
	commands := newFakeProcessCommandRunner()

	report, err := cleaner.NewCleaner(nil, commands).Run(context.Background(), cleaner.Options{
		LogPath: filepath.Join(home, "cleanup.log"),
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if report.Warnings == 0 {
		t.Fatalf("expected a running target process warning, got %#v", report)
	}
	if len(commands.killCommands) != 0 {
		t.Fatalf("expected no kill commands without force stop, got %#v", commands.killCommands)
	}
}

func TestRunForceStopsTargetProcessesWhenOptedIn(t *testing.T) {
	home := fakeHome(t)
	commands := newFakeProcessCommandRunner()

	report, err := cleaner.NewCleaner(nil, commands).Run(context.Background(), cleaner.Options{
		ForceStopProcesses: true,
		LogPath:            filepath.Join(home, "cleanup.log"),
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if report.Warnings == 0 {
		t.Fatalf("expected a force-stop warning entry, got %#v", report)
	}
	if len(commands.killCommands) == 0 {
		t.Fatal("expected at least one kill command")
	}

	if runtime.GOOS == "windows" {
		assertCommandCalled(t, commands.killCommands, "taskkill", "/F", "/IM", "Code.exe")
		assertCommandNotCalled(t, commands.killCommands, "taskkill", "/F", "/IM", "Codex.exe")
		return
	}

	assertCommandCalled(t, commands.killCommands, "pkill", "-x", "code")
	assertCommandNotCalled(t, commands.killCommands, "pkill", "-x", "claude")
}

func TestRunLogsTargetsInStableOrder(t *testing.T) {
	fakeHome(t)
	run := func() []string {
		report, err := cleaner.NewCleaner(jitterFileSystem{}, newFakeProcessCommandRunner()).Run(context.Background(), cleaner.Options{})
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		messages := make([]string, 0, len(report.Entries))
		for _, entry := range report.Entries {
			messages = append(messages, entry.Message)
		}
		return messages
	}

	first := strings.Join(run(), "\n")
	second := strings.Join(run(), "\n")
	if first != second {
		t.Fatalf("expected identical log order across runs, got:\n%s\n---\n%s", first, second)
	}
}

func fakeHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	t.Setenv("HOMEDRIVE", filepath.VolumeName(home))
	t.Setenv("HOMEPATH", strings.TrimPrefix(home, filepath.VolumeName(home)))
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))

	return home
}

type jitterFileSystem struct{}

func (jitterFileSystem) UserHomeDir() (string, error) { return os.UserHomeDir() }

func (jitterFileSystem) Getenv(key string) string { return os.Getenv(key) }

func (jitterFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (jitterFileSystem) Lstat(name string) (os.FileInfo, error) { return os.Lstat(name) }

func (jitterFileSystem) ReadDir(name string) ([]os.DirEntry, error) { return os.ReadDir(name) }

func (jitterFileSystem) OpenRoot(dir string) (*os.Root, error) { return os.OpenRoot(dir) }

func (jitterFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (jitterFileSystem) EvalSymlinks(path string) (string, error) {
	time.Sleep(rand.N(2 * time.Millisecond))
	return filepath.EvalSymlinks(path)
}

func (jitterFileSystem) Glob(pattern string) ([]string, error) { return filepath.Glob(pattern) }

type fakeProcessCommandRunner struct {
	killCommands [][]string
	onFirstCall  func()
}

func (r *fakeProcessCommandRunner) noteCall() {
	if r.onFirstCall != nil {
		r.onFirstCall()
		r.onFirstCall = nil
	}
}

func newFakeProcessCommandRunner() *fakeProcessCommandRunner {
	return &fakeProcessCommandRunner{}
}

func (r *fakeProcessCommandRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	r.noteCall()
	if name == "tasklist.exe" {
		return []byte("\"Codex.exe\",\"1234\",\"Console\",\"1\",\"10,000 K\"\n\"Code.exe\",\"5678\",\"Console\",\"1\",\"10,000 K\"\n"), nil
	}
	return nil, nil
}

func (r *fakeProcessCommandRunner) Run(_ context.Context, name string, args ...string) error {
	r.noteCall()
	command := append([]string{name}, args...)
	switch name {
	case "pgrep":
		if len(args) == 2 && args[0] == "-x" && (args[1] == "claude" || args[1] == "code") {
			return nil
		}
		return errors.New("process not found")
	case "pkill", "taskkill":
		r.killCommands = append(r.killCommands, command)
		return nil
	default:
		return nil
	}
}

func hasEntry(report cleaner.Report, level cleaner.Level, text string) bool {
	for _, entry := range report.Entries {
		if entry.Level == level && strings.Contains(entry.Message, text) {
			return true
		}
	}
	return false
}

func assertLogContains(t *testing.T, path string, want string) {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log %q: %v", path, err)
	}
	if !strings.Contains(string(body), want) {
		t.Fatalf("expected log to contain %q, got:\n%s", want, string(body))
	}
}

func assertNoDefaultCleanupLogs(t *testing.T, home string) {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(home, "offboarding-cleanup-*"))
	if err != nil {
		t.Fatalf("glob default cleanup logs: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no default cleanup logs, got %#v", matches)
	}
}

func assertCommandCalled(t *testing.T, commands [][]string, want ...string) {
	t.Helper()

	for _, command := range commands {
		if strings.Join(command, "\x00") == strings.Join(want, "\x00") {
			return
		}
	}

	t.Fatalf("expected command %#v, got %#v", want, commands)
}

func assertCommandNotCalled(t *testing.T, commands [][]string, unwanted ...string) {
	t.Helper()

	for _, command := range commands {
		if strings.Join(command, "\x00") == strings.Join(unwanted, "\x00") {
			t.Fatalf("expected command %#v not to be called, got %#v", unwanted, commands)
		}
	}
}
