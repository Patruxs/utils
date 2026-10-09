package cleaner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Level string

const (
	LevelInfo   Level = "INFO"
	LevelWarn   Level = "WARN"
	LevelSkip   Level = "SKIP"
	LevelDryRun Level = "DRY-RUN"
	LevelDelete Level = "DELETE"
	LevelError  Level = "ERROR"
)

type Options struct {
	Execute                bool
	CleanSSHKeys           bool
	CleanShellHistory      bool
	FullToolReset          bool
	IncludeBrowserProfiles bool
	CleanCredentialManager bool
	ForceStopProcesses     bool
	LogPath                string
}

type Entry struct {
	Time    time.Time
	Level   Level
	Message string
}

type Report struct {
	Entries  []Entry
	LogPath  string
	Deleted  int
	DryRuns  int
	Skipped  int
	Warnings int
	Errors   int
}

type FileSystem interface {
	UserHomeDir() (string, error)
	Getenv(key string) string
	MkdirAll(path string, perm os.FileMode) error
	Lstat(name string) (os.FileInfo, error)
	ReadDir(name string) ([]os.DirEntry, error)
	OpenRoot(dir string) (*os.Root, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	EvalSymlinks(path string) (string, error)
	Glob(pattern string) ([]string, error)
}

type CommandRunner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	Run(ctx context.Context, name string, args ...string) error
}

type Cleaner struct {
	fs       FileSystem
	commands CommandRunner
}

func NewCleaner(fs FileSystem, commands CommandRunner) Cleaner {
	return Cleaner{fs: fs, commands: commands}.withDefaults()
}

func Run(ctx context.Context, opts Options) (Report, error) {
	return NewCleaner(nil, nil).Run(ctx, opts)
}

func (c Cleaner) Run(ctx context.Context, opts Options) (Report, error) {
	c = c.withDefaults()

	home, err := c.fs.UserHomeDir()
	if err != nil {
		return Report{}, fmt.Errorf("detect user home: %w", err)
	}

	home, err = filepath.Abs(home)
	if err != nil {
		return Report{}, fmt.Errorf("resolve user home: %w", err)
	}

	realHome, err := c.fs.EvalSymlinks(home)
	if err != nil {
		return Report{}, fmt.Errorf("resolve user home: %w", err)
	}

	logPath, err := c.resolveLogPath(opts.LogPath, home)
	if err != nil {
		return Report{}, err
	}

	report := Report{LogPath: logPath}
	if report.LogPath == "" {
		report.add(LevelInfo, "Starting local offboarding cleanup.")
	} else {
		report.add(LevelInfo, "Starting local offboarding cleanup. Log: %s", logPath)
	}
	if opts.Execute {
		report.add(LevelInfo, "Mode: execute. Local files can be deleted.")
	} else {
		report.add(LevelInfo, "Mode: dry-run. No files will be deleted.")
	}

	var runErrors []error
	clean := func(targets []targetPath) {
		if err := c.cleanTargets(ctx, &report, realHome, targets, opts.Execute); err != nil {
			runErrors = append(runErrors, err)
		}
	}

	stopRun := func(err error) (Report, error) {
		report.add(LevelWarn, "Cleanup stopped before finishing: %v. Remaining targets were not checked.", err)
		return c.finishRun(report, append(runErrors, err))
	}

	stages := []func(){
		func() {
			if err := c.handleTargetProcesses(ctx, &report, opts.ForceStopProcesses); err != nil {
				report.add(LevelWarn, "Could not handle running target processes: %v", err)
				if opts.ForceStopProcesses {
					runErrors = append(runErrors, err)
				}
			}
		},
		func() {
			clean(developerCredentialTargets(home, c.fs))
		},
		func() {
			if !opts.FullToolReset {
				report.add(LevelInfo, "Full tool reset is off. Tool folders, tool configs, IDE data, and AI tool data were kept.")
				return
			}
			report.add(LevelWarn, "Full tool reset is enabled. This removes whole tool folders, including installed runtimes, VMs, IDE data, and AI tool data.")
			clean(fullToolResetTargets(home, c.fs))
		},
		func() {
			if !opts.CleanSSHKeys {
				report.add(LevelInfo, "SSH keys were not removed. Enable SSH key cleanup to remove local keys.")
				return
			}
			report.add(LevelWarn, "SSH key cleanup is enabled. This removes local private and public keys.")
			clean(sshTargets(home, c.fs))
		},
		func() {
			if !opts.CleanShellHistory {
				report.add(LevelInfo, "Shell and tool histories were kept. Enable history cleanup to remove them.")
				return
			}
			report.add(LevelWarn, "History cleanup is enabled. This removes shell, REPL, database, and debugger histories.")
			clean(historyTargets(home, c.fs))
		},
		func() {
			if !opts.IncludeBrowserProfiles {
				report.add(LevelInfo, "Browser caches and profiles were not removed. Enable browser profile cleanup to remove caches, cookies, sessions, passwords, extensions, local storage, history, and bookmarks.")
				return
			}
			report.add(LevelWarn, "Browser profile cleanup is enabled. This removes browser caches, local sign-ins, and profile data.")
			clean(browserCacheTargets(home, c.fs))
			clean(browserProfileTargets(home, c.fs))
		},
		func() {
			if !opts.CleanCredentialManager {
				report.add(LevelInfo, "Windows Credential Manager was not changed. Enable Credential Manager cleanup to review or delete matching dev credentials.")
				return
			}
			if runtime.GOOS != osWindows {
				report.add(LevelSkip, "Windows Credential Manager cleanup is only available on Windows.")
				return
			}
			report.add(LevelInfo, "Windows Credential Manager cleanup is enabled with a conservative allowlist.")
			if err := c.cleanCredentialManager(ctx, &report, opts.Execute); err != nil {
				runErrors = append(runErrors, err)
			}
		},
	}

	for _, stage := range stages {
		if err := ctx.Err(); err != nil {
			return stopRun(err)
		}
		stage()
	}

	if err := ctx.Err(); err != nil {
		return stopRun(err)
	}

	report.add(LevelInfo, "Local cleanup finished.")
	report.add(LevelInfo, "Reminder: revoke remote sessions, PATs, SSH keys, API keys, and SSO sessions from their admin portals.")
	return c.finishRun(report, runErrors)
}

func (c Cleaner) finishRun(report Report, runErrors []error) (Report, error) {
	if report.LogPath != "" {
		if err := c.writeLog(report.LogPath, report.Entries); err != nil {
			runErrors = append(runErrors, fmt.Errorf("write cleanup log: %w", err))
		}
	}

	return report, errors.Join(runErrors...)
}

func (c Cleaner) withDefaults() Cleaner {
	if c.fs == nil {
		c.fs = osFileSystem{}
	}
	if c.commands == nil {
		c.commands = execCommandRunner{}
	}
	return c
}

type osFileSystem struct{}

func (osFileSystem) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

func (osFileSystem) Getenv(key string) string {
	return os.Getenv(key)
}

func (osFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (osFileSystem) Lstat(name string) (os.FileInfo, error) {
	return os.Lstat(name)
}

func (osFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}

func (osFileSystem) OpenRoot(dir string) (*os.Root, error) {
	return os.OpenRoot(dir)
}

func (osFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (osFileSystem) EvalSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

func (osFileSystem) Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

type execCommandRunner struct{}

func (execCommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return newCommand(ctx, name, args...).Output()
}

func (execCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	return newCommand(ctx, name, args...).Run()
}

func newCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = commandWaitDelay
	return cmd
}

func (r *Report) add(level Level, format string, args ...any) {
	switch level {
	case LevelDelete:
		r.Deleted++
	case LevelDryRun:
		r.DryRuns++
	case LevelSkip:
		r.Skipped++
	case LevelWarn:
		r.Warnings++
	case LevelError:
		r.Errors++
	}

	r.Entries = append(r.Entries, Entry{
		Time:    time.Now(),
		Level:   level,
		Message: fmt.Sprintf(format, args...),
	})
}

func (r *Report) merge(other Report) {
	r.Entries = append(r.Entries, other.Entries...)
	r.Deleted += other.Deleted
	r.DryRuns += other.DryRuns
	r.Skipped += other.Skipped
	r.Warnings += other.Warnings
	r.Errors += other.Errors
}

func (c Cleaner) resolveLogPath(path string, home string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve cleanup log path: %w", err)
	}

	if !isUnderUserHome(home, abs) {
		return "", fmt.Errorf("refusing to write cleanup log outside current user profile: %s", abs)
	}

	if err := c.fs.MkdirAll(filepath.Dir(abs), userPrivateDirPerm); err != nil {
		return "", fmt.Errorf("prepare cleanup log directory: %w", err)
	}

	return abs, nil
}

func (c Cleaner) cleanPath(ctx context.Context, report *Report, home string, path string, label string, execute bool) error {
	if ctx.Err() != nil || strings.TrimSpace(path) == "" {
		return nil
	}

	realParent, err := c.fs.EvalSymlinks(filepath.Dir(path))
	if errors.Is(err, os.ErrNotExist) {
		report.add(LevelSkip, "%s not found: %s", label, path)
		return nil
	}
	if err != nil {
		report.add(LevelError, "Could not resolve %s: %s: %v", label, path, err)
		return fmt.Errorf("resolve %s %q: %w", label, path, err)
	}

	realPath := filepath.Join(realParent, filepath.Base(path))
	rel, ok := pathInsideHome(home, realPath)
	if !ok {
		report.add(LevelSkip, "Refusing to touch %s outside current user profile: %s resolves to %s", label, path, realPath)
		return nil
	}

	root, err := c.fs.OpenRoot(home)
	if err != nil {
		report.add(LevelError, "Could not open user profile for %s: %s: %v", label, path, err)
		return fmt.Errorf("open user profile for %s %q: %w", label, path, err)
	}
	defer root.Close()

	info, err := root.Lstat(rel)
	if errors.Is(err, os.ErrNotExist) {
		report.add(LevelSkip, "%s not found: %s", label, path)
		return nil
	}
	if err != nil {
		report.add(LevelError, "Could not inspect %s: %s: %v", label, path, err)
		return fmt.Errorf("inspect %s %q: %w", label, path, err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return c.removeSymlink(report, root, rel, path, label, execute)
	}

	if !execute {
		report.add(LevelDryRun, "Would delete %s: %s", label, path)
		return nil
	}

	if err := root.RemoveAll(rel); err != nil {
		if info.IsDir() {
			report.add(LevelError, "Could not fully delete %s: %s: %d items left inside: %v", label, path, countEntriesInside(root, rel), err)
			return fmt.Errorf("fully delete %s %q: %w", label, path, err)
		}
		report.add(LevelError, "Could not delete %s: %s: %v", label, path, err)
		return fmt.Errorf("delete %s %q: %w", label, path, err)
	}

	report.add(LevelDelete, "Deleted %s: %s", label, path)
	return nil
}

func (c Cleaner) removeSymlink(report *Report, root *os.Root, rel string, path string, label string, execute bool) error {
	target, err := c.fs.EvalSymlinks(path)
	if err != nil {
		target, err = root.Readlink(rel)
	}
	if err != nil {
		target = "unknown"
	}

	if !execute {
		report.add(LevelDryRun, "Would remove symlink for %s: %s, target %s kept", label, path, target)
		return nil
	}

	if err := root.Remove(rel); err != nil {
		report.add(LevelError, "Could not remove symlink for %s: %s: %v", label, path, err)
		return fmt.Errorf("remove symlink %s %q: %w", label, path, err)
	}

	report.add(LevelDelete, "Removed symlink for %s: %s, target %s kept", label, path, target)
	return nil
}

func countEntriesInside(root *os.Root, rel string) int {
	start := filepath.ToSlash(rel)
	count := 0
	fs.WalkDir(root.FS(), start, func(name string, _ fs.DirEntry, err error) error {
		if err == nil && name != start {
			count++
		}
		return nil
	})
	return count
}

func pathInsideHome(home string, path string) (string, bool) {
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return "", false
	}
	return rel, true
}

func (c Cleaner) cleanTargets(ctx context.Context, report *Report, home string, targets []targetPath, execute bool) error {
	results := make([]Report, len(targets))
	runErrors := make([]error, len(targets))

	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Go(func() {
			runErrors[i] = c.cleanPath(ctx, &results[i], home, target.path, target.label, execute)
		})
	}
	wg.Wait()

	for _, result := range results {
		report.merge(result)
	}

	return errors.Join(runErrors...)
}

func isUnderUserHome(home string, path string) bool {
	home = cleanComparablePath(home)
	path = cleanComparablePath(path)
	if home == "" || path == "" || home == path {
		return false
	}

	separator := string(os.PathSeparator)
	if !strings.HasSuffix(home, separator) {
		home += separator
	}

	return strings.HasPrefix(path, home)
}

func cleanComparablePath(path string) string {
	path = filepath.Clean(path)
	path = strings.TrimRight(path, string(os.PathSeparator))
	if runtime.GOOS == osWindows {
		path = strings.ToLower(path)
	}
	return path
}

func (c Cleaner) writeLog(path string, entries []Entry) error {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{})

	for _, entry := range entries {
		record := slog.NewRecord(entry.Time, slogLevel(entry.Level), entry.Message, 0)
		record.AddAttrs(slog.String(logAttrCleanupLevel, string(entry.Level)))
		if err := handler.Handle(context.Background(), record); err != nil {
			return fmt.Errorf("write structured log entry: %w", err)
		}
	}

	return c.fs.WriteFile(path, buf.Bytes(), userPrivateFilePerm)
}

func slogLevel(level Level) slog.Level {
	switch level {
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (c Cleaner) handleTargetProcesses(ctx context.Context, report *Report, forceStop bool) error {
	names, err := c.runningProcessNames(ctx)
	if err != nil {
		return err
	}

	targetNames := map[string]struct{}{
		"chrome":              {},
		"chrome.exe":          {},
		"google-chrome":       {},
		"chromium":            {},
		"firefox":             {},
		"firefox.exe":         {},
		"msedge":              {},
		"msedge.exe":          {},
		"microsoft edge":      {},
		"code":                {},
		"code.exe":            {},
		"code - insiders.exe": {},
		"code-insiders":       {},
		"codium":              {},
		"codium.exe":          {},
		"devenv":              {},
		"devenv.exe":          {},
	}

	var running []string
	seen := make(map[string]struct{})
	for _, name := range names {
		key := strings.ToLower(name)
		if _, ok := targetNames[key]; !ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		running = append(running, name)
	}

	if len(running) == 0 {
		return nil
	}

	if !forceStop {
		report.add(LevelWarn, "A target process appears to be running. Close browsers, IDEs, and AI apps before cleanup.")
		return nil
	}

	var runErrors []error
	for _, name := range running {
		if ctx.Err() != nil {
			break
		}
		if err := c.stopProcess(ctx, name); err != nil {
			if ctx.Err() != nil {
				break
			}
			report.add(LevelError, "Could not force stop target process %s: %v", name, err)
			runErrors = append(runErrors, fmt.Errorf("force stop target process %q: %w", name, err))
			continue
		}

		report.add(LevelWarn, "Force stopped target process: %s", name)
	}

	return errors.Join(runErrors...)
}

func (c Cleaner) stopProcess(ctx context.Context, name string) error {
	if runtime.GOOS == osWindows {
		return c.commands.Run(ctx, commandTaskkill, commandArgTaskkillForce, commandArgTaskkillImage, name)
	}

	return c.commands.Run(ctx, commandPkill, commandArgExactProcess, name)
}

func (c Cleaner) runningProcessNames(ctx context.Context) ([]string, error) {
	if runtime.GOOS == osWindows {
		out, err := c.commands.Output(ctx, commandTasklist, commandArgTasklistFormat, commandArgTasklistCSV, commandArgTasklistNoHeader)
		if err != nil {
			return nil, err
		}
		return parseTasklistCSV(string(out)), nil
	}

	candidates := []string{"chrome", "google-chrome", "chromium", "firefox", "msedge", "code", "code-insiders", "codium", "devenv"}
	var running []string
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			break
		}
		if err := c.commands.Run(ctx, commandPgrep, commandArgExactProcess, candidate); err == nil {
			running = append(running, candidate)
		}
	}
	return running, nil
}

func parseTasklistCSV(output string) []string {
	var names []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		line = strings.TrimPrefix(line, "\"")
		name, _, _ := strings.Cut(line, "\",")
		name = strings.TrimSpace(strings.Trim(name, "\""))
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func (c Cleaner) cleanCredentialManager(ctx context.Context, report *Report, execute bool) error {
	out, err := c.commands.Output(ctx, commandCmdkey, commandArgCmdkeyList)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		report.add(LevelError, "Could not list Windows Credential Manager entries: %v", err)
		return fmt.Errorf("list Windows Credential Manager entries: %w", err)
	}

	var runErrors []error
	for _, target := range parseCredentialManagerTargets(string(out)) {
		if ctx.Err() != nil {
			break
		}
		if !matchesCredentialAllowlist(target) {
			continue
		}

		if !execute {
			report.add(LevelDryRun, "Would delete Windows Credential Manager entry: %s", target)
			continue
		}

		if err := c.commands.Run(ctx, commandCmdkey, commandArgCmdkeyDelete+target); err != nil {
			if ctx.Err() != nil {
				break
			}
			report.add(LevelError, "Could not delete Windows Credential Manager entry %s: %v", target, err)
			runErrors = append(runErrors, fmt.Errorf("delete Windows Credential Manager entry %q: %w", target, err))
			continue
		}

		report.add(LevelDelete, "Deleted Windows Credential Manager entry: %s", target)
	}

	return errors.Join(runErrors...)
}

func parseCredentialManagerTargets(output string) []string {
	var targets []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if value, ok := strings.CutPrefix(line, cmdkeyTargetPrefix); ok {
			value = strings.TrimSpace(value)
			if value != "" {
				targets = append(targets, value)
			}
		}
	}
	return targets
}

func matchesCredentialAllowlist(target string) bool {
	target = strings.ToLower(target)
	for _, pattern := range credentialManagerAllowlist {
		if strings.Contains(target, pattern) {
			return true
		}
	}
	return false
}
