package network

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type fakeCommandRunner struct {
	output func(name string, args []string) ([]byte, error)
	calls  []string
}

func (f *fakeCommandRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	return f.output(name, args)
}

func TestFailedElevationEndsWithErrorAndCommandOutput(t *testing.T) {
	runner := &fakeCommandRunner{output: func(string, []string) ([]byte, error) {
		return []byte("sudo: a password is required\n"), errors.New("exit status 1")
	}}

	report, err := NewNetworkManager(runner).SetDNS(context.Background(), CloudflareDNSOptions())

	if err == nil {
		t.Fatal("expected an error when every command fails")
	}
	if !strings.Contains(err.Error(), "a password is required") {
		t.Fatalf("expected the command output in the error, got %v", err)
	}
	if report.Errors == 0 {
		t.Fatalf("expected Report.Errors to count the failure: %+v", report.Entries)
	}
	for _, entry := range report.Entries {
		if entry.Level == LevelSuccess {
			t.Fatalf("expected no SUCCESS entry after a failed write, got %q", entry.Message)
		}
	}
}

func TestSavedDNSWithInjectionIsRejectedBeforeAnyWrite(t *testing.T) {
	saved := "PersistentMode=True\nDNSPrimary=1.1.1.1\"); Start-Process calc; (\"\nDNSSecondary=1.0.0.1\nDNSName=Cloudflare\n"
	runner := &fakeCommandRunner{output: func(string, []string) ([]byte, error) {
		return []byte(saved), nil
	}}

	report, err := NewNetworkManager(runner).ApplyPersistentSettings(context.Background())

	if err == nil {
		t.Fatalf("expected the saved injection value to be rejected: %+v", report.Entries)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected only the settings read to run, got %d commands: %q", len(runner.calls), runner.calls)
	}
}

func TestHostsRemoveDeletesOnlyManagedEntries(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("runs the POSIX hosts scripts with sh")
	}
	hostsPath := filepath.Join(t.TempDir(), "hosts")
	original := "127.0.0.1\tlocalhost\n127.0.1.1\tmybox\n# keep this comment\n\n192.168.1.10\tnas.home\n10.0.0.5\thost.docker.internal"
	if err := os.WriteFile(hostsPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	runShellScript(t, posixAddHostsEntryScript(hostsPath, "10.1.2.3", "dev.local"))
	output := runShellScript(t, posixRemoveManagedHostsScript(hostsPath))

	content, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "dev.local") {
		t.Fatalf("expected the managed entry to be removed:\n%s", content)
	}
	if strings.TrimRight(string(content), "\n") != original {
		t.Fatalf("expected every unmanaged line to stay unchanged:\n%q", content)
	}
	if !strings.Contains(output, "127.0.1.1\tmybox") {
		t.Fatalf("expected the output to list the unmanaged lines left alone:\n%s", output)
	}
}

func TestHostsRestoreUsesNewestBackupAndKeepsCurrentFile(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("runs the POSIX hosts scripts with sh")
	}
	hostsPath := filepath.Join(t.TempDir(), "hosts")
	for _, step := range []struct{ content, stamp string }{
		{"older\n", "20260101-000000.000000000"},
		{"newer\n", "20260102-000000.000000000"},
	} {
		if err := os.WriteFile(hostsPath, []byte(step.content), 0o644); err != nil {
			t.Fatal(err)
		}
		runShellScript(t, posixBackupHostsScript(hostsPath, step.stamp))
	}
	if err := os.WriteFile(hostsPath, []byte("current\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runShellScript(t, posixRestoreHostsScript(hostsPath, "20260103-000000.000000000"))

	assertFileContent(t, hostsPath, "newer\n")
	assertFileContent(t, hostsPath+".before-restore-20260103-000000.000000000", "current\n")
}

func runShellScript(t *testing.T, script string) string {
	t.Helper()
	output, err := exec.Command("sh", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, output)
	}
	return string(output)
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("%s: expected %q, got %q", path, want, content)
	}
}
