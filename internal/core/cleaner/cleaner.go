package cleaner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
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
	mu       sync.Mutex
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
	RemoveAll(path string) error
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

	if err := c.handleTargetProcesses(ctx, &report, opts.ForceStopProcesses); err != nil {
		report.add(LevelWarn, "Could not handle running target processes: %v", err)
		if opts.ForceStopProcesses {
			runErrors = append(runErrors, err)
		}
	}

	if err := c.cleanTargets(ctx, &report, home, developerTargets(home, c.fs), opts.Execute); err != nil {
		runErrors = append(runErrors, err)
	}

	if opts.CleanSSHKeys {
		report.add(LevelWarn, "SSH key cleanup is enabled. This removes local private and public keys.")
		if err := c.cleanTargets(ctx, &report, home, sshTargets(home, c.fs), opts.Execute); err != nil {
			runErrors = append(runErrors, err)
		}
	} else {
		report.add(LevelInfo, "SSH keys were not removed. Enable SSH key cleanup to remove local keys.")
	}

	if err := c.cleanTargets(ctx, &report, home, historyTargets(home, c.fs), opts.Execute); err != nil {
		runErrors = append(runErrors, err)
	}

	if err := c.cleanTargets(ctx, &report, home, browserCacheTargets(home, c.fs), opts.Execute); err != nil {
		runErrors = append(runErrors, err)
	}

	if opts.IncludeBrowserProfiles {
		report.add(LevelWarn, "Browser profile cleanup is enabled. This removes local sign-ins and profile data.")
		if err := c.cleanTargets(ctx, &report, home, browserProfileTargets(home, c.fs), opts.Execute); err != nil {
			runErrors = append(runErrors, err)
		}
	} else {
		report.add(LevelInfo, "Browser profiles were not removed. Enable browser profile cleanup to remove local cookies, sessions, passwords, extensions, local storage, history, and bookmarks.")
	}

	if opts.CleanCredentialManager {
		if runtime.GOOS == osWindows {
			report.add(LevelInfo, "Windows Credential Manager cleanup is enabled with a conservative allowlist.")
			if err := c.cleanCredentialManager(ctx, &report, opts.Execute); err != nil {
				runErrors = append(runErrors, err)
			}
		} else {
			report.add(LevelSkip, "Windows Credential Manager cleanup is only available on Windows.")
		}
	} else {
		report.add(LevelInfo, "Windows Credential Manager was not changed. Enable Credential Manager cleanup to review or delete matching dev credentials.")
	}

	report.add(LevelInfo, "Local cleanup finished.")
	report.add(LevelInfo, "Reminder: revoke remote sessions, PATs, SSH keys, API keys, and SSO sessions from their admin portals.")

	if report.LogPath != "" {
		if err := c.writeLog(report.LogPath, report.entriesSnapshot()); err != nil {
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

func (osFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
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
	return exec.CommandContext(ctx, name, args...).Output()
}

func (execCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

func (r *Report) add(level Level, format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()

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

func (r *Report) entriesSnapshot() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries := make([]Entry, len(r.Entries))
	copy(entries, r.Entries)
	return entries
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
	if err := ctx.Err(); err != nil {
		return err
	}

	if strings.TrimSpace(path) == "" {
		return nil
	}

	info, err := c.fs.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		report.add(LevelSkip, "%s not found: %s", label, path)
		return nil
	}
	if err != nil {
		report.add(LevelError, "Could not inspect %s: %s: %v", label, path, err)
		return fmt.Errorf("inspect %s %q: %w", label, path, err)
	}

	resolvedPath, err := filepath.Abs(path)
	if err != nil {
		report.add(LevelError, "Could not resolve %s: %s: %v", label, path, err)
		return fmt.Errorf("resolve %s %q: %w", label, path, err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		resolvedPath, err = c.fs.EvalSymlinks(path)
		if err != nil {
			report.add(LevelError, "Could not resolve symlink for %s: %s: %v", label, path, err)
			return fmt.Errorf("resolve symlink %s %q: %w", label, path, err)
		}
	}

	if !isUnderUserHome(home, resolvedPath) {
		report.add(LevelSkip, "Refusing to touch path outside current user profile: %s", resolvedPath)
		return nil
	}

	if !execute {
		report.add(LevelDryRun, "Would delete %s: %s", label, resolvedPath)
		return nil
	}

	if err := c.fs.RemoveAll(path); err != nil {
		report.add(LevelError, "Could not delete %s: %s: %v", label, resolvedPath, err)
		return fmt.Errorf("delete %s %q: %w", label, resolvedPath, err)
	}

	report.add(LevelDelete, "Deleted %s: %s", label, resolvedPath)
	return nil
}

func (c Cleaner) cleanTargets(ctx context.Context, report *Report, home string, targets []targetPath, execute bool) error {
	group, groupCtx := errgroup.WithContext(ctx)

	var mu sync.Mutex
	var runErrors []error

	for _, target := range targets {
		target := target
		group.Go(func() error {
			if err := c.cleanPath(groupCtx, report, home, target.path, target.label, execute); err != nil {
				mu.Lock()
				runErrors = append(runErrors, err)
				mu.Unlock()
			}
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		runErrors = append(runErrors, err)
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
		"claude":              {},
		"claude.exe":          {},
		"code":                {},
		"code.exe":            {},
		"code - insiders.exe": {},
		"code-insiders":       {},
		"codium":              {},
		"codium.exe":          {},
		"codex":               {},
		"codex.exe":           {},
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
		if err := c.stopProcess(ctx, name); err != nil {
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

	candidates := []string{"chrome", "google-chrome", "chromium", "firefox", "msedge", "claude", "Code", "Code.exe", "code-insiders", "codium", "codex", "devenv"}
	var running []string
	for _, candidate := range candidates {
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
		report.add(LevelError, "Could not list Windows Credential Manager entries: %v", err)
		return fmt.Errorf("list Windows Credential Manager entries: %w", err)
	}

	var runErrors []error
	for _, target := range parseCredentialManagerTargets(string(out)) {
		if !matchesCredentialAllowlist(target) {
			continue
		}

		if !execute {
			report.add(LevelDryRun, "Would delete Windows Credential Manager entry: %s", target)
			continue
		}

		if err := c.commands.Run(ctx, commandCmdkey, commandArgCmdkeyDelete+target); err != nil {
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
