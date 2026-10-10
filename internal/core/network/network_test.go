package network

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"
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

func TestCanceledOperationIsReportedAsCanceledNotFailed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := &fakeCommandRunner{output: func(string, []string) ([]byte, error) {
		cancel()
		return []byte("=== Network Diagnostics ===\n"), errors.New("signal: killed")
	}}

	report, err := NewNetworkManager(runner).Diagnostics(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if report.Errors != 0 {
		t.Fatalf("expected a canceled run not to count as an error: %+v", report.Entries)
	}
	for _, entry := range report.Entries {
		if entry.Level == LevelError {
			t.Fatalf("expected no ERROR entry after cancel, got %q", entry.Message)
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

func TestPreviewShowsTheCommandsTheActionRuns(t *testing.T) {
	saved := "PersistentMode=True\nDNSPrimary=9.9.9.9\nDNSSecondary=149.112.112.112\nDNSName=Quad9\n"
	discoveredSaved := "persistent=" + strings.ReplaceAll(strings.TrimSpace(saved), "\n", "\npersistent=")
	readAtRunTime := strings.NewReplacer(placeholderSavedPrimary, "9.9.9.9", placeholderSavedBackup, "149.112.112.112")
	statuses := map[string]Status{
		"nothing discovered": {},
		"NetworkManager":     parseStatus(runtime.GOOS, "tools= nmcli resolvectl\nconnection=1a2b3c4d:802-11-wireless\nlink=2: wlan0: <UP> mtu 1500\nadapterindex=12 Wi-Fi\nservice=Wi-Fi\nport=Wi-Fi\ndoh=True\ndohserver=1.1.1.1\n"+discoveredSaved),
		"systemd-resolved":   parseStatus(runtime.GOOS, "tools= resolvectl\nlink=2: eth0: <UP> mtu 1500"),
		"resolv.conf":        parseStatus(runtime.GOOS, "tools= nscd"),
	}
	dns := CloudflareDNSOptions()
	dns.Persistent = true
	params := Params{DNS: dns, Hosts: HostsOptions{IP: "10.1.2.3", Domain: "dev.local"}, Browser: BrowserAll, Persistent: true}
	disable := params
	disable.Persistent = false

	for name, status := range statuses {
		for action := ActionViewConfig; action <= ActionClearPersistent; action++ {
			for _, params := range []Params{params, disable} {
				runner := &fakeCommandRunner{output: func(string, []string) ([]byte, error) { return []byte(saved), nil }}
				_, _ = NewNetworkManager(runner).Run(context.Background(), action, params)
				executed := executedScripts(t, runner.calls)

				steps := previewSteps(runtime.GOOS, action, params, status)
				if action.Supported() && action != ActionFlushDNS && len(steps) == 0 {
					t.Fatalf("%s: expected a preview for action %d", name, action)
				}
				for _, step := range steps {
					if !strings.Contains(executed, readAtRunTime.Replace(step.template)) {
						t.Fatalf("%s: action %d previews %q, which is not in what it ran:\n%s", name, action, step.template, executed)
					}
				}
			}
		}
	}
}

func TestPreviewUsesDiscoveredConnectionsAndAPlaceholderOtherwise(t *testing.T) {
	params := Params{DNS: CloudflareDNSOptions()}
	known := parseStatus(osLinux, "tools= nmcli\nconnection=1a2b3c4d:802-11-wireless\nconnection=99999999:loopback\n")

	lines := strings.Join(renderedPreview(osLinux, ActionSetDNS, params, known), "\n")
	if !strings.Contains(lines, `nmcli connection modify 1a2b3c4d ipv4.dns "1.1.1.1 1.0.0.1"`) || strings.Contains(lines, "99999999") {
		t.Fatalf("expected the real non-loopback connection and DNS values, got:\n%s", lines)
	}

	unknown := strings.Join(renderedPreview(osLinux, ActionSetDNS, params, Status{}), "\n")
	if !strings.Contains(unknown, placeholderConnection) {
		t.Fatalf("expected a placeholder when connections are unknown, got:\n%s", unknown)
	}
}

func TestReadStatusRunsOnlyReadOnlyCommands(t *testing.T) {
	runner := &fakeCommandRunner{output: func(string, []string) ([]byte, error) {
		return []byte("route=default via 10.0.0.1 dev eth0\n"), nil
	}}

	if _, err := NewNetworkManager(runner).ReadStatus(context.Background()); err != nil {
		t.Fatal(err)
	}

	reads := map[string]bool{}
	var writes []string
	for _, writing := range []bool{false, true} {
		for action := ActionViewConfig; action <= ActionClearPersistent; action++ {
			if action.Writes() != writing {
				continue
			}
			for _, step := range previewSteps(runtime.GOOS, action, Params{Persistent: true, Browser: BrowserAll}, Status{}) {
				if !writing {
					reads[step.template] = true
				} else if !reads[step.template] {
					writes = append(writes, step.template)
				}
			}
		}
	}
	nullRedirects := strings.NewReplacer("2>/dev/null", "", ">/dev/null", "", "2>&1", "")
	for _, call := range runner.calls {
		if strings.HasPrefix(call, commandSudo+" ") {
			if call != "sudo -n true" {
				t.Fatalf("expected only the non-prompting sudo probe, got %q", call)
			}
			continue
		}
		if strings.Contains(nullRedirects.Replace(call), ">") {
			t.Fatalf("expected no output redirection into a file, got:\n%s", call)
		}
		for _, write := range writes {
			if strings.Contains(call, write) {
				t.Fatalf("expected no write command in the status read, found %q", write)
			}
		}
	}
}

func TestStatusLeavesUnreadableFieldsUnknown(t *testing.T) {
	runner := &fakeCommandRunner{output: func(name string, _ []string) ([]byte, error) {
		if name == commandSudo {
			return nil, errors.New("exit status 1")
		}
		return []byte("route=default via 10.0.0.1 dev eth0\ngarbage line\n"), errors.New("exit status 127")
	}}

	status, err := NewNetworkManager(runner).ReadStatus(context.Background())

	if err != nil {
		t.Fatalf("expected a partial read not to fail the whole status, got %v", err)
	}
	if runtime.GOOS == osLinux && (status.Adapter != "eth0" || status.Gateway != "10.0.0.1") {
		t.Fatalf("expected the readable fields to be kept, got %+v", status)
	}
	if status.Address != "" || status.MTU != 0 || len(status.DNS) != 0 || status.Persistent != StateUnknown {
		t.Fatalf("expected the unreadable fields to stay unknown, got %+v", status)
	}
}

func renderedPreview(goos string, action Action, params Params, status Status) []string {
	var lines []string
	for _, step := range previewSteps(goos, action, params, status) {
		lines = append(lines, renderPreviewStep(step))
	}
	return lines
}

func executedScripts(t *testing.T, calls []string) string {
	t.Helper()
	var scripts []string
	for _, call := range calls {
		scripts = append(scripts, call)
		_, encoded, ok := strings.Cut(call, "-EncodedCommand ")
		if !ok {
			continue
		}
		encoded, _, _ = strings.Cut(encoded, `"`)
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		units := make([]uint16, len(raw)/2)
		for index := range units {
			units[index] = binary.LittleEndian.Uint16(raw[2*index:])
		}
		scripts = append(scripts, string(utf16.Decode(units)))
	}
	return strings.Join(scripts, "\n")
}
