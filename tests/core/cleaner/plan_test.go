package cleaner_test

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"utils/internal/core/cleaner"
)

func TestPlanNeverChangesTheProfileWhateverTheOptions(t *testing.T) {
	home := fakeHome(t)
	outside := t.TempDir()
	privateKey := "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXk=\n-----END OPENSSH PRIVATE KEY-----\n"
	chrome, edge := chromiumProfileRoots(home)
	writeFiles(t, map[string]string{
		filepath.Join(home, ".aws", "credentials"):                 "secret",
		filepath.Join(home, ".oci", "sessions", "x"):               "secret",
		filepath.Join(home, ".ssh", "id_ed25519"):                  privateKey,
		filepath.Join(home, ".ssh", "known_hosts"):                 "host",
		filepath.Join(home, ".bash_history"):                       "export TOKEN=x",
		filepath.Join(chrome, "Default", "c"):                      "cookie",
		filepath.Join(home, ".codex", "sessions", "history.jsonl"): "history",
		filepath.Join(outside, "gh", "hosts.yml"):                  "secret",
		filepath.Join(edge, "Profile 1", "Login"):                  "password",
	})
	if err := os.MkdirAll(filepath.Join(home, ".config"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "gh"), filepath.Join(home, ".config", "gh")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	before := snapshotTree(t, home, outside)
	commands := newFakeProcessCommandRunner()
	planner := cleaner.NewCleaner(writeGuardFileSystem{t: t}, commands)

	var plan cleaner.Plan
	for round := range 3 {
		for mask := range 1 << 7 {
			opts := cleaner.Options{
				CleanSSHKeys:           mask&1 != 0,
				CleanShellHistory:      mask&2 != 0,
				FullToolReset:          mask&4 != 0,
				IncludeBrowserProfiles: mask&8 != 0,
				CleanCredentialManager: mask&16 != 0,
				ForceStopProcesses:     mask&32 != 0,
				Execute:                mask&64 != 0,
				LogPath:                filepath.Join(home, "logs", fmt.Sprintf("plan-%d-%d.log", round, mask)),
			}
			var err error
			plan, err = planner.Plan(context.Background(), opts)
			if err != nil {
				t.Fatalf("Plan(%+v) returned error: %v", opts, err)
			}
			if got := plan.Group(cleaner.GroupSSHKeys).Enabled; got != opts.CleanSSHKeys {
				t.Fatalf("expected the SSH group enabled=%v, got %v", opts.CleanSSHKeys, got)
			}
		}
	}

	if after := snapshotTree(t, home, outside); !slices.Equal(before, after) {
		t.Fatalf("expected Plan to leave every file as it was\nbefore: %v\nafter:  %v", before, after)
	}
	if len(commands.killCommands) != 0 {
		t.Fatalf("expected Plan never to stop a process, got %#v", commands.killCommands)
	}

	for id, want := range map[cleaner.GroupID]int{
		cleaner.GroupSSHKeys:         2,
		cleaner.GroupShellHistory:    1,
		cleaner.GroupBrowserProfiles: 2,
	} {
		if got := plan.Group(id).Found(); got != want {
			t.Fatalf("expected %s to report %d found while off, got %d: %#v", id, want, got, plan.Group(id).Entries)
		}
	}
	credentials := plan.Group(cleaner.GroupCredentials)
	oci, ok := planEntry(credentials, filepath.Join(home, ".oci"))
	if !ok || !oci.IsDir || oci.Refused != "" {
		t.Fatalf("expected ~/.oci listed as a folder, got %#v", credentials.Entries)
	}
	refused, ok := planEntry(credentials, filepath.Join(home, ".config", "gh", "hosts.yml"))
	if !ok || refused.Refused != cleaner.RefusedOutsideProfile || !strings.HasPrefix(refused.LinkTarget, evalSymlinks(t, outside)) {
		t.Fatalf("expected the link that leaves the profile listed as refused with where it leads, got %#v", credentials.Entries)
	}
	if _, ok := planEntry(credentials, filepath.Join(home, ".npmrc")); ok {
		t.Fatal("expected targets that are not present to be left out of the plan")
	}
}

func TestPlanListsRunningAppsWithoutStoppingThem(t *testing.T) {
	fakeHome(t)
	commands := newFakeProcessCommandRunner()

	plan, err := cleaner.NewCleaner(nil, commands).Plan(context.Background(), cleaner.Options{ForceStopProcesses: true})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}

	group := plan.Group(cleaner.GroupForceStop)
	want := "code"
	if runtime.GOOS == "windows" {
		want = "Code.exe"
	}
	if !group.Enabled || !slices.Equal(group.Processes, []string{want}) {
		t.Fatalf("expected the force-stop group to list %q as running, got %#v", want, group)
	}
	if len(commands.killCommands) != 0 {
		t.Fatalf("expected Plan never to run the kill path, got %#v", commands.killCommands)
	}
}

func TestRunProgressReachesTotal(t *testing.T) {
	home := fakeHome(t)
	files := map[string]string{
		filepath.Join(home, ".aws", "credentials"):   "secret",
		filepath.Join(home, ".npmrc"):                "secret",
		filepath.Join(home, ".oci", "sessions", "x"): "secret",
		filepath.Join(home, ".bash_history"):         "history",
		filepath.Join(home, ".zsh_history"):          "history",
	}
	writeFiles(t, files)

	var steps []cleaner.Progress
	report, err := cleaner.NewCleaner(nil, newFakeProcessCommandRunner()).Run(context.Background(), cleaner.Options{
		Execute:            true,
		CleanShellHistory:  true,
		ForceStopProcesses: true,
		Progress:           func(p cleaner.Progress) { steps = append(steps, p) },
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	const deletions, stopped = 5, 1
	if len(steps) != deletions+stopped {
		t.Fatalf("expected one progress step per deletion and stopped app, got %d: %#v", len(steps), steps)
	}
	for i, step := range steps {
		if step.Done != i+1 || step.Total != deletions+stopped {
			t.Fatalf("expected step %d to be %d/%d, got %d/%d", i, i+1, deletions+stopped, step.Done, step.Total)
		}
	}
	if report.Deleted != deletions {
		t.Fatalf("expected %d deletions, got %d", deletions, report.Deleted)
	}
	for path := range files {
		if _, err := os.Stat(path); err == nil && !strings.Contains(path, ".oci") {
			t.Fatalf("expected %q deleted", path)
		}
	}
}

func writeFiles(t *testing.T, files map[string]string) {
	t.Helper()
	for path, body := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func chromiumProfileRoots(home string) (string, string) {
	switch runtime.GOOS {
	case "windows":
		local := filepath.Join(home, "AppData", "Local")
		return filepath.Join(local, "Google", "Chrome", "User Data"), filepath.Join(local, "Microsoft", "Edge", "User Data")
	case "darwin":
		support := filepath.Join(home, "Library", "Application Support")
		return filepath.Join(support, "Google", "Chrome"), filepath.Join(support, "Microsoft Edge")
	}
	return filepath.Join(home, ".config", "google-chrome"), filepath.Join(home, ".config", "microsoft-edge")
}

func snapshotTree(t *testing.T, roots ...string) []string {
	t.Helper()
	var snapshot []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := os.Lstat(path)
			if err != nil {
				return err
			}
			if info.IsDir() {
				snapshot = append(snapshot, fmt.Sprintf("%s %v", path, info.Mode()))
				return nil
			}
			var body []byte
			if info.Mode().IsRegular() {
				if body, err = os.ReadFile(path); err != nil {
					return err
				}
			}
			link, _ := os.Readlink(path)
			snapshot = append(snapshot, fmt.Sprintf("%s %v %d %d %s %q", path, info.Mode(), info.Size(), info.ModTime().UnixNano(), link, body))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return snapshot
}

func planEntry(group cleaner.PlanGroup, path string) (cleaner.PlanEntry, bool) {
	for _, entry := range group.Entries {
		if entry.Path == path {
			return entry, true
		}
	}
	return cleaner.PlanEntry{}, false
}

type writeGuardFileSystem struct {
	t *testing.T
}

func (writeGuardFileSystem) UserHomeDir() (string, error) { return os.UserHomeDir() }

func (writeGuardFileSystem) Getenv(key string) string { return os.Getenv(key) }

func (writeGuardFileSystem) Lstat(name string) (os.FileInfo, error) { return os.Lstat(name) }

func (writeGuardFileSystem) ReadDir(name string) ([]os.DirEntry, error) { return os.ReadDir(name) }

func (writeGuardFileSystem) OpenRoot(dir string) (*os.Root, error) { return os.OpenRoot(dir) }

func (writeGuardFileSystem) EvalSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

func (writeGuardFileSystem) Glob(pattern string) ([]string, error) { return filepath.Glob(pattern) }

func (f writeGuardFileSystem) MkdirAll(path string, _ os.FileMode) error {
	f.t.Errorf("expected Plan not to create %q", path)
	return os.ErrPermission
}

func (f writeGuardFileSystem) WriteFile(name string, _ []byte, _ os.FileMode) error {
	f.t.Errorf("expected Plan not to write %q", name)
	return os.ErrPermission
}
