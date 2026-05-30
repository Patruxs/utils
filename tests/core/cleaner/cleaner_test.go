package cleaner_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "token"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := cleaner.Run(context.Background(), cleaner.Options{Execute: true})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected target to be deleted, stat err=%v", err)
	}
	if report.Deleted == 0 {
		t.Fatalf("expected at least one delete entry, got %#v", report)
	}
	if report.LogPath != "" {
		t.Fatalf("expected no default log path, got %q", report.LogPath)
	}
	assertNoDefaultCleanupLogs(t, home)
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

func TestRunHonorsCanceledContext(t *testing.T) {
	home := fakeHome(t)
	target := filepath.Join(home, ".npmrc")
	if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := cleaner.Run(ctx, cleaner.Options{Execute: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected canceled cleanup to leave target: %v", err)
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
		assertCommandCalled(t, commands.killCommands, "taskkill", "/F", "/IM", "Codex.exe")
		return
	}

	assertCommandCalled(t, commands.killCommands, "pkill", "-x", "claude")
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

type fakeProcessCommandRunner struct {
	killCommands [][]string
}

func newFakeProcessCommandRunner() *fakeProcessCommandRunner {
	return &fakeProcessCommandRunner{}
}

func (r *fakeProcessCommandRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	if name == "tasklist.exe" {
		return []byte("\"Codex.exe\",\"1234\",\"Console\",\"1\",\"10,000 K\"\n"), nil
	}
	return nil, nil
}

func (r *fakeProcessCommandRunner) Run(_ context.Context, name string, args ...string) error {
	command := append([]string{name}, args...)
	switch name {
	case "pgrep":
		if len(args) == 2 && args[0] == "-x" && args[1] == "claude" {
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
