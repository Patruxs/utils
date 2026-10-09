package cleaner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type GroupID string

const (
	GroupCredentials       GroupID = "credentials"
	GroupSSHKeys           GroupID = "ssh_keys"
	GroupShellHistory      GroupID = "shell_history"
	GroupBrowserProfiles   GroupID = "browser_profiles"
	GroupFullToolReset     GroupID = "full_tool_reset"
	GroupCredentialManager GroupID = "credential_manager"
	GroupForceStop         GroupID = "force_stop"
)

const RefusedOutsideProfile = "link leaves profile"

var planGroupOrder = []GroupID{
	GroupCredentials,
	GroupSSHKeys,
	GroupShellHistory,
	GroupBrowserProfiles,
	GroupFullToolReset,
	GroupCredentialManager,
	GroupForceStop,
}

type Plan struct {
	Home   string
	Groups []PlanGroup
}

type PlanGroup struct {
	ID        GroupID
	Enabled   bool
	Available bool
	Entries   []PlanEntry
	Processes []string
}

type PlanEntry struct {
	Target     string
	Path       string
	LinkTarget string
	Refused    string
	IsDir      bool
}

type Progress struct {
	Done  int
	Total int
	Entry Entry
}

func (p Plan) Group(id GroupID) PlanGroup {
	for _, group := range p.Groups {
		if group.ID == id {
			return group
		}
	}
	return PlanGroup{ID: id}
}

func (g PlanGroup) Found() int {
	if g.ID == GroupForceStop {
		return len(g.Processes)
	}
	found := 0
	for _, entry := range g.Entries {
		if entry.Refused == "" {
			found++
		}
	}
	return found
}

func (o Options) Enabled(id GroupID) bool {
	switch id {
	case GroupCredentials:
		return true
	case GroupSSHKeys:
		return o.CleanSSHKeys
	case GroupShellHistory:
		return o.CleanShellHistory
	case GroupBrowserProfiles:
		return o.IncludeBrowserProfiles
	case GroupFullToolReset:
		return o.FullToolReset
	case GroupCredentialManager:
		return o.CleanCredentialManager
	case GroupForceStop:
		return o.ForceStopProcesses
	}
	return false
}

func GroupAvailable(id GroupID) bool {
	return id != GroupCredentialManager || runtime.GOOS == osWindows
}

func PlanCleanup(ctx context.Context, opts Options) (Plan, error) {
	return NewCleaner(nil, nil).Plan(ctx, opts)
}

func (c Cleaner) Plan(ctx context.Context, opts Options) (Plan, error) {
	c = c.withDefaults()

	home, realHome, err := c.resolveHome()
	if err != nil {
		return Plan{}, err
	}

	scan := c.scan(ctx, home, realHome, planGroupOrder)
	plan := Plan{Home: home}
	var scanErrors []error
	for _, id := range planGroupOrder {
		group := scan.group(id)
		if group.err != nil {
			scanErrors = append(scanErrors, group.err)
		}
		plan.Groups = append(plan.Groups, PlanGroup{
			ID:        id,
			Enabled:   opts.Enabled(id),
			Available: GroupAvailable(id),
			Entries:   group.planEntries(),
			Processes: group.processes,
		})
	}

	if err := ctx.Err(); err != nil {
		return plan, err
	}
	return plan, errors.Join(scanErrors...)
}

func (c Cleaner) resolveHome() (string, string, error) {
	home, err := c.fs.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("detect user home: %w", err)
	}

	home, err = filepath.Abs(home)
	if err != nil {
		return "", "", fmt.Errorf("resolve user home: %w", err)
	}

	realHome, err := c.fs.EvalSymlinks(home)
	if err != nil {
		return "", "", fmt.Errorf("resolve user home: %w", err)
	}

	return home, realHome, nil
}

type inspectionState int

const (
	inspectionNotPresent inspectionState = iota
	inspectionPresent
	inspectionRefused
	inspectionFailed
)

type inspection struct {
	state      inspectionState
	target     targetPath
	resolved   string
	linkTarget string
	isDir      bool
	err        error
}

func (i inspection) attempted() bool {
	return i.state == inspectionPresent || i.state == inspectionFailed
}

func (c Cleaner) inspectPath(home string, target targetPath) inspection {
	found := inspection{target: target}

	realParent, err := c.fs.EvalSymlinks(filepath.Dir(target.path))
	if errors.Is(err, os.ErrNotExist) {
		return found
	}
	if err != nil {
		found.state, found.err = inspectionFailed, err
		return found
	}

	found.resolved = filepath.Join(realParent, filepath.Base(target.path))
	rel, ok := pathInsideHome(home, found.resolved)
	if !ok {
		found.state = inspectionRefused
		return found
	}

	root, err := c.fs.OpenRoot(home)
	if err != nil {
		found.state, found.err = inspectionFailed, err
		return found
	}
	defer root.Close()

	info, err := root.Lstat(rel)
	if errors.Is(err, os.ErrNotExist) {
		return found
	}
	if err != nil {
		found.state, found.err = inspectionFailed, err
		return found
	}

	found.state = inspectionPresent
	found.isDir = info.IsDir()
	if info.Mode()&os.ModeSymlink != 0 {
		found.linkTarget = c.symlinkTarget(root, rel, target.path)
	}
	return found
}

func (c Cleaner) symlinkTarget(root *os.Root, rel string, path string) string {
	target, err := c.fs.EvalSymlinks(path)
	if err != nil {
		target, err = root.Readlink(rel)
	}
	if err != nil {
		return "unknown"
	}
	return target
}

type groupScan struct {
	id                 GroupID
	targets            []targetPath
	found              []inspection
	processes          []string
	credentialTargets  []string
	credentialListFail error
	err                error
}

func (g groupScan) planEntries() []PlanEntry {
	var entries []PlanEntry
	for _, found := range g.found {
		entry := PlanEntry{Target: found.target.label, Path: found.target.path, LinkTarget: found.linkTarget, IsDir: found.isDir}
		switch found.state {
		case inspectionNotPresent:
			continue
		case inspectionRefused:
			entry.LinkTarget = found.resolved
			entry.Refused = RefusedOutsideProfile
		case inspectionFailed:
			entry.Refused = found.err.Error()
		}
		entries = append(entries, entry)
	}
	for _, target := range g.credentialTargets {
		entries = append(entries, PlanEntry{Target: targetLabelCredentialManagerEntry, Path: target})
	}
	return entries
}

type cleanupScan struct {
	groups []groupScan
}

func (s cleanupScan) group(id GroupID) groupScan {
	for _, group := range s.groups {
		if group.id == id {
			return group
		}
	}
	return groupScan{id: id}
}

func (s cleanupScan) attempts(opts Options) int {
	total := 0
	for _, group := range s.groups {
		if !opts.Enabled(group.id) {
			continue
		}
		if group.id == GroupForceStop {
			total += len(group.processes)
			continue
		}
		total += len(group.credentialTargets)
		for _, found := range group.found {
			if found.attempted() {
				total++
			}
		}
	}
	return total
}

func (c Cleaner) scan(ctx context.Context, home string, realHome string, ids []GroupID) cleanupScan {
	var scan cleanupScan
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		scan.groups = append(scan.groups, c.scanGroup(ctx, home, realHome, id))
	}
	return scan
}

func (c Cleaner) scanGroup(ctx context.Context, home string, realHome string, id GroupID) groupScan {
	group := groupScan{id: id}
	switch id {
	case GroupForceStop:
		group.processes, group.err = c.runningTargetProcesses(ctx)
		if group.err != nil {
			group.err = fmt.Errorf("list running target processes: %w", group.err)
		}
		return group
	case GroupCredentialManager:
		if !GroupAvailable(id) {
			return group
		}
		group.credentialTargets, group.credentialListFail = c.listCredentialManagerTargets(ctx)
		if group.credentialListFail != nil && ctx.Err() == nil {
			group.err = fmt.Errorf("list Windows Credential Manager entries: %w", group.credentialListFail)
		}
		return group
	}

	group.targets = c.groupTargets(home, id)
	group.found = make([]inspection, len(group.targets))
	var wg sync.WaitGroup
	for i, target := range group.targets {
		wg.Go(func() {
			group.found[i] = c.inspectPath(realHome, target)
		})
	}
	wg.Wait()
	return group
}

func (c Cleaner) groupTargets(home string, id GroupID) []targetPath {
	switch id {
	case GroupCredentials:
		return developerCredentialTargets(home, c.fs)
	case GroupSSHKeys:
		return sshTargets(home, c.fs)
	case GroupShellHistory:
		return historyTargets(home, c.fs)
	case GroupBrowserProfiles:
		return append(browserCacheTargets(home, c.fs), browserProfileTargets(home, c.fs)...)
	case GroupFullToolReset:
		return fullToolResetTargets(home, c.fs)
	}
	return nil
}

type progressTracker struct {
	mu     sync.Mutex
	report func(Progress)
	done   int
	total  int
}

func newProgressTracker(report func(Progress), total int) *progressTracker {
	return &progressTracker{report: report, total: total}
}

func (p *progressTracker) step(entry Entry) {
	if p == nil || p.report == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done++
	p.report(Progress{Done: p.done, Total: p.total, Entry: entry})
}
