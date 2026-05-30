package network

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

type Level string

const (
	LevelInfo    Level = "INFO"
	LevelWarn    Level = "WARN"
	LevelSuccess Level = "SUCCESS"
	LevelError   Level = "ERROR"
)

type Entry struct {
	Time    time.Time
	Level   Level
	Message string
}

type Report struct {
	mu        sync.Mutex
	Operation string
	Entries   []Entry
	Warnings  int
	Errors    int
}

type ConfigOptions struct {
	DNSName      string
	DNSPrimary   string
	DNSSecondary string
	EnableDoH    bool
	MTU          int
	Persistent   bool
}

type PersistentSettings struct {
	Enabled      bool
	DNSName      string
	DNSPrimary   string
	DNSSecondary string
}

type HostsMode int

const (
	HostsView HostsMode = iota
	HostsAdd
	HostsRemoveCustom
	HostsBackup
	HostsRestore
)

type HostsOptions struct {
	Mode   HostsMode
	IP     string
	Domain string
}

type BrowserMode int

const (
	BrowserChrome BrowserMode = iota
	BrowserFirefox
	BrowserEdge
	BrowserBrave
	BrowserOpera
	BrowserAll
)

type NetworkManager struct {
	commands CommandRunner
}

func NewNetworkManager(commands CommandRunner) NetworkManager {
	return NetworkManager{commands: commands}.withDefaults()
}

func DefaultConfigOptions() ConfigOptions {
	return CloudflareDNSOptions()
}

func CloudflareDNSOptions() ConfigOptions {
	return ConfigOptions{
		DNSName:      "Cloudflare",
		DNSPrimary:   "1.1.1.1",
		DNSSecondary: "1.0.0.1",
		EnableDoH:    true,
		MTU:          1500,
	}
}

func GoogleDNSOptions() ConfigOptions {
	return ConfigOptions{
		DNSName:      "Google",
		DNSPrimary:   "8.8.8.8",
		DNSSecondary: "8.8.4.4",
		EnableDoH:    true,
		MTU:          1500,
	}
}

func OpenDNSOptions() ConfigOptions {
	return ConfigOptions{
		DNSName:      "OpenDNS",
		DNSPrimary:   "208.67.222.222",
		DNSSecondary: "208.67.220.220",
		MTU:          1500,
	}
}

func Quad9DNSOptions() ConfigOptions {
	return ConfigOptions{
		DNSName:      "Quad9",
		DNSPrimary:   "9.9.9.9",
		DNSSecondary: "149.112.112.112",
		EnableDoH:    true,
		MTU:          1500,
	}
}

func DNSPresets() []ConfigOptions {
	return []ConfigOptions{
		CloudflareDNSOptions(),
		GoogleDNSOptions(),
		OpenDNSOptions(),
		Quad9DNSOptions(),
	}
}

func DefaultHostsOptions() HostsOptions {
	return HostsOptions{Mode: HostsBackup}
}

func CurrentConfig(ctx context.Context) (Report, error) {
	return NewNetworkManager(nil).CurrentConfig(ctx)
}

func Diagnostics(ctx context.Context) (Report, error) {
	return NewNetworkManager(nil).Diagnostics(ctx)
}

func ApplyConfig(ctx context.Context, opts ConfigOptions) (Report, error) {
	return NewNetworkManager(nil).ApplyConfig(ctx, opts)
}

func SetDNS(ctx context.Context, opts ConfigOptions) (Report, error) {
	return NewNetworkManager(nil).SetDNS(ctx, opts)
}

func FlushDNSCache(ctx context.Context) (Report, error) {
	return NewNetworkManager(nil).FlushDNSCache(ctx)
}

func EnableDoH(ctx context.Context) (Report, error) {
	return NewNetworkManager(nil).EnableDoH(ctx)
}

func DisableDoH(ctx context.Context) (Report, error) {
	return NewNetworkManager(nil).DisableDoH(ctx)
}

func OptimizeNetworkSettings(ctx context.Context) (Report, error) {
	return NewNetworkManager(nil).OptimizeNetworkSettings(ctx)
}

func ResetNetworkOptimizations(ctx context.Context) (Report, error) {
	return NewNetworkManager(nil).ResetNetworkOptimizations(ctx)
}

func ResetDNS(ctx context.Context) (Report, error) {
	return NewNetworkManager(nil).ResetDNS(ctx)
}

func ResetToDefaults(ctx context.Context) (Report, error) {
	return NewNetworkManager(nil).ResetToDefaults(ctx)
}

func ClearBrowserCache(ctx context.Context, mode BrowserMode) (Report, error) {
	return NewNetworkManager(nil).ClearBrowserCache(ctx, mode)
}

func EditHosts(ctx context.Context, opts HostsOptions) (Report, error) {
	return NewNetworkManager(nil).EditHosts(ctx, opts)
}

func PersistentStatus(ctx context.Context) (Report, error) {
	return NewNetworkManager(nil).PersistentStatus(ctx)
}

func SetPersistentMode(ctx context.Context, enabled bool, opts ConfigOptions) (Report, error) {
	return NewNetworkManager(nil).SetPersistentMode(ctx, enabled, opts)
}

func ApplyPersistentSettings(ctx context.Context) (Report, error) {
	return NewNetworkManager(nil).ApplyPersistentSettings(ctx)
}

func ClearPersistentSettings(ctx context.Context) (Report, error) {
	return NewNetworkManager(nil).ClearPersistentSettings(ctx)
}

func (m NetworkManager) CurrentConfig(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "View Current Network Config"}
	report.add(LevelInfo, "Inspecting network configuration with standard user permissions.")

	output, err := m.outputPlatformScript(ctx, windowsCurrentConfigScript(), darwinCurrentConfigScript(), linuxCurrentConfigScript())
	report.addOutput(LevelInfo, output)
	if err != nil {
		report.add(LevelError, "Network inspection failed: %v", err)
		return report, err
	}

	report.add(LevelSuccess, "Network inspection completed.")
	return report, nil
}

func (m NetworkManager) Diagnostics(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Run Network Diagnostics"}
	report.add(LevelInfo, "Running connectivity, DNS, and ping diagnostics with standard user permissions.")

	output, err := m.outputPlatformScript(ctx, windowsDiagnosticsScript(), darwinDiagnosticsScript(), linuxDiagnosticsScript())
	report.addOutput(LevelInfo, output)
	if err != nil {
		report.add(LevelError, "Network diagnostics failed: %v", err)
		return report, err
	}

	report.add(LevelSuccess, "Network diagnostics completed.")
	return report, nil
}

func (m NetworkManager) SetDNS(ctx context.Context, opts ConfigOptions) (Report, error) {
	m = m.withDefaults()
	opts = normalizeConfigOptions(opts)
	report := Report{Operation: fmt.Sprintf("Set %s DNS", opts.DNSName)}
	report.add(LevelInfo, "Applying %s DNS (%s, %s).", opts.DNSName, opts.DNSPrimary, opts.DNSSecondary)
	report.add(LevelInfo, "The TUI is running as a standard user. DNS writes request Administrator/root only for the DNS command.")

	if m.applyDNS(ctx, &report, opts) && opts.Persistent {
		m.savePersistentSettings(ctx, &report, opts)
	}

	report.add(LevelSuccess, "DNS configuration flow completed.")
	return report, nil
}

func (m NetworkManager) ApplyConfig(ctx context.Context, opts ConfigOptions) (Report, error) {
	m = m.withDefaults()
	opts = normalizeConfigOptions(opts)
	report := Report{Operation: "Apply Network Config"}
	report.add(LevelInfo, "Applying %s DNS (%s, %s), DoH=%t, MTU=%d.", opts.DNSName, opts.DNSPrimary, opts.DNSSecondary, opts.EnableDoH, opts.MTU)
	report.add(LevelInfo, "The TUI is running as a standard user. Each write command requests Administrator/root only for that command.")

	if m.applyDNS(ctx, &report, opts) && opts.Persistent {
		m.savePersistentSettings(ctx, &report, opts)
	}
	if opts.EnableDoH {
		m.enableDoH(ctx, &report)
	}
	m.applyMTU(ctx, &report, opts.MTU)

	report.add(LevelSuccess, "Network configuration flow completed. Review warnings for any settings that could not be applied.")
	return report, nil
}

func (m NetworkManager) FlushDNSCache(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Flush DNS Cache"}
	m.flushDNS(ctx, &report)
	report.add(LevelSuccess, "DNS cache flush flow completed.")
	return report, nil
}

func (m NetworkManager) EnableDoH(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Enable DNS over HTTPS"}
	m.enableDoH(ctx, &report)
	report.add(LevelSuccess, "DNS over HTTPS enable flow completed.")
	return report, nil
}

func (m NetworkManager) DisableDoH(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Disable DNS over HTTPS"}
	m.disableDoH(ctx, &report)
	report.add(LevelSuccess, "DNS over HTTPS disable flow completed.")
	return report, nil
}

func (m NetworkManager) OptimizeNetworkSettings(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Optimize Network Settings"}
	if runtime.GOOS != osWindows {
		report.add(LevelWarn, "Windows-specific TCP knobs from tool.ps1 are not available on %s; applying MTU and available TCP equivalents where the OS supports them.", runtime.GOOS)
	}
	m.runPrivilegedWithFallback(ctx, &report, "network optimization", windowsOptimizeNetworkScript(), darwinOptimizeNetworkScript(), linuxOptimizeNetworkScript())
	report.add(LevelSuccess, "Network optimization flow completed. Restart may be required for full effect.")
	return report, nil
}

func (m NetworkManager) ResetNetworkOptimizations(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Reset Network Optimizations"}
	if runtime.GOOS != osWindows {
		report.add(LevelWarn, "Winsock/TCP reset is Windows-specific; applying best-effort reset commands available on %s.", runtime.GOOS)
	}
	m.runPrivilegedWithFallback(ctx, &report, "network optimization reset", windowsResetNetworkOptimizationsScript(), darwinResetNetworkOptimizationsScript(), linuxResetNetworkOptimizationsScript())
	report.add(LevelSuccess, "Network optimization reset flow completed. Restart may be required.")
	return report, nil
}

func (m NetworkManager) ResetDNS(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Reset DNS to Automatic"}
	m.resetDNS(ctx, &report)
	report.add(LevelSuccess, "DNS reset flow completed.")
	return report, nil
}

func (m NetworkManager) ResetToDefaults(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Reset Network Settings to Defaults"}
	report.add(LevelInfo, "Resetting DNS to automatic, disabling DoH where supported, and clearing persistent DNS settings.")
	m.resetDNS(ctx, &report)
	m.disableDoH(ctx, &report)
	m.clearPersistentSettings(ctx, &report)
	report.add(LevelSuccess, "Default reset flow completed.")
	return report, nil
}

func (m NetworkManager) ClearBrowserCache(ctx context.Context, mode BrowserMode) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Clear Browser Cache"}
	report.add(LevelWarn, "Close browsers before clearing cache so locked files can be removed.")
	report.add(LevelInfo, "Clearing %s cache with standard user permissions.", browserModeLabel(mode))

	output, err := m.outputPlatformScript(ctx, windowsBrowserCacheScript(mode), darwinBrowserCacheScript(mode), linuxBrowserCacheScript(mode))
	report.addOutput(LevelInfo, output)
	if err != nil {
		report.add(LevelWarn, "Browser cache cleanup completed with errors: %v", err)
		return report, nil
	}

	report.add(LevelSuccess, "Browser cache cleanup completed.")
	return report, nil
}

func (m NetworkManager) EditHosts(ctx context.Context, opts HostsOptions) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Edit Hosts File"}
	if opts.Mode == HostsView {
		report.add(LevelInfo, "Viewing hosts file with standard user permissions.")
		output, err := m.outputPlatformScript(ctx, windowsViewHostsScript(), darwinViewHostsScript(), linuxViewHostsScript())
		report.addOutput(LevelInfo, output)
		if err != nil {
			report.add(LevelWarn, "Could not read hosts file: %v", err)
		}
		return report, nil
	}

	report.add(LevelInfo, "Inspecting hosts file before write operation.")
	output, err := m.outputPlatformScript(ctx, windowsViewHostsScript(), darwinViewHostsScript(), linuxViewHostsScript())
	report.addOutput(LevelInfo, output)
	if err != nil {
		report.add(LevelWarn, "Could not read hosts file before editing: %v", err)
	}

	windowsScript, darwinScript, linuxScript, label := hostsScripts(opts)
	if label == "" {
		report.add(LevelWarn, "No hosts write operation was selected.")
		return report, nil
	}

	report.add(LevelInfo, "Attempting hosts operation: %s.", label)
	m.runPrivilegedWithFallback(ctx, &report, label, windowsScript, darwinScript, linuxScript)
	report.add(LevelSuccess, "Hosts flow completed. Review warnings if elevated or fallback writes were denied.")
	return report, nil
}

func (m NetworkManager) PersistentStatus(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Persistent DNS Settings"}
	settings, err := m.loadPersistentSettings(ctx, &report)
	if err != nil {
		report.add(LevelWarn, "Could not read persistent settings: %v", err)
		return report, nil
	}
	if !settings.Enabled {
		report.add(LevelInfo, "Persistent DNS mode: disabled.")
		return report, nil
	}

	report.add(LevelSuccess, "Persistent DNS mode: enabled.")
	report.add(LevelInfo, "Saved DNS: %s (%s, %s).", settings.DNSName, settings.DNSPrimary, settings.DNSSecondary)
	return report, nil
}

func (m NetworkManager) SetPersistentMode(ctx context.Context, enabled bool, opts ConfigOptions) (Report, error) {
	m = m.withDefaults()
	opts = normalizeConfigOptions(opts)
	report := Report{Operation: "Toggle Persistent DNS Mode"}
	if !enabled {
		m.clearPersistentSettings(ctx, &report)
		report.add(LevelSuccess, "Persistent DNS mode disabled.")
		return report, nil
	}

	m.savePersistentSettings(ctx, &report, opts)
	report.add(LevelSuccess, "Persistent DNS mode enabled with %s (%s, %s). DNS preset actions can update this saved value.", opts.DNSName, opts.DNSPrimary, opts.DNSSecondary)
	return report, nil
}

func (m NetworkManager) ApplyPersistentSettings(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Apply Persistent DNS Settings"}
	settings, err := m.loadPersistentSettings(ctx, &report)
	if err != nil {
		report.add(LevelWarn, "Could not load persistent settings: %v", err)
		return report, nil
	}
	if !settings.Enabled || settings.DNSPrimary == "" || settings.DNSSecondary == "" {
		report.add(LevelWarn, "No persistent DNS settings are saved.")
		return report, nil
	}

	opts := ConfigOptions{
		DNSName:      settings.DNSName,
		DNSPrimary:   settings.DNSPrimary,
		DNSSecondary: settings.DNSSecondary,
		MTU:          1500,
	}
	m.applyDNS(ctx, &report, opts)
	report.add(LevelSuccess, "Persistent DNS apply flow completed.")
	return report, nil
}

func (m NetworkManager) ClearPersistentSettings(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Clear Persistent DNS Settings"}
	m.clearPersistentSettings(ctx, &report)
	report.add(LevelSuccess, "Persistent DNS settings cleared.")
	return report, nil
}

func (m NetworkManager) withDefaults() NetworkManager {
	if m.commands == nil {
		m.commands = execCommandRunner{}
	}
	return m
}

func (m NetworkManager) applyDNS(ctx context.Context, report *Report, opts ConfigOptions) bool {
	return m.runPrivilegedWithFallback(ctx, report, fmt.Sprintf("%s DNS configuration", opts.DNSName), windowsSetDNSScript(opts), darwinSetDNSScript(opts), linuxSetDNSScript(opts))
}

func (m NetworkManager) flushDNS(ctx context.Context, report *Report) bool {
	return m.runPrivilegedWithFallback(ctx, report, "DNS cache flush", windowsFlushDNSCacheScript(), darwinFlushDNSCacheScript(), linuxFlushDNSCacheScript())
}

func (m NetworkManager) enableDoH(ctx context.Context, report *Report) bool {
	if runtime.GOOS != osWindows {
		report.add(LevelWarn, "OS-level DNS over HTTPS is not configured generically on %s. Use a managed resolver profile or local DoH proxy if required.", runtime.GOOS)
		return false
	}
	return m.runPrivilegedWithFallback(ctx, report, "DNS over HTTPS configuration", windowsEnableDoHScript(), "", "")
}

func (m NetworkManager) disableDoH(ctx context.Context, report *Report) bool {
	if runtime.GOOS != osWindows {
		report.add(LevelWarn, "OS-level DNS over HTTPS disable is not available generically on %s.", runtime.GOOS)
		return false
	}
	return m.runPrivilegedWithFallback(ctx, report, "DNS over HTTPS removal", windowsDisableDoHScript(), "", "")
}

func (m NetworkManager) applyMTU(ctx context.Context, report *Report, mtu int) bool {
	return m.runPrivilegedWithFallback(ctx, report, "MTU configuration", windowsSetMTUScript(mtu), darwinSetMTUScript(mtu), linuxSetMTUScript(mtu))
}

func (m NetworkManager) resetDNS(ctx context.Context, report *Report) bool {
	return m.runPrivilegedWithFallback(ctx, report, "DNS reset", windowsResetDNSScript(), darwinResetDNSScript(), linuxResetDNSScript())
}

func (m NetworkManager) savePersistentSettings(ctx context.Context, report *Report, opts ConfigOptions) bool {
	if err := m.runStandardScript(ctx, platformScript(windowsSavePersistentSettingsScript(opts), darwinSavePersistentSettingsScript(opts), linuxSavePersistentSettingsScript(opts))); err != nil {
		report.add(LevelWarn, "Could not save persistent DNS settings: %v", err)
		return false
	}
	report.add(LevelSuccess, "Saved persistent DNS settings: %s (%s, %s).", opts.DNSName, opts.DNSPrimary, opts.DNSSecondary)
	return true
}

func (m NetworkManager) clearPersistentSettings(ctx context.Context, report *Report) bool {
	if err := m.runStandardScript(ctx, platformScript(windowsClearPersistentSettingsScript(), darwinClearPersistentSettingsScript(), linuxClearPersistentSettingsScript())); err != nil {
		report.add(LevelWarn, "Could not clear persistent DNS settings: %v", err)
		return false
	}
	report.add(LevelSuccess, "Cleared persistent DNS settings.")
	return true
}

func (m NetworkManager) loadPersistentSettings(ctx context.Context, report *Report) (PersistentSettings, error) {
	output, err := m.outputPlatformScript(ctx, windowsPersistentStatusScript(), darwinPersistentStatusScript(), linuxPersistentStatusScript())
	if err != nil {
		return PersistentSettings{}, err
	}
	settings := parsePersistentSettings(output)
	if strings.TrimSpace(output) != "" {
		report.addOutput(LevelInfo, output)
	}
	return settings, nil
}

func (m NetworkManager) outputPlatformScript(ctx context.Context, windowsScript, darwinScript, linuxScript string) (string, error) {
	spec := platformScriptCommand(windowsScript, darwinScript, linuxScript)
	if spec.name == "" {
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	output, err := m.commands.Output(ctx, spec.name, spec.args...)
	return strings.TrimRight(string(output), "\r\n"), err
}

func (m NetworkManager) runPrivilegedWithFallback(ctx context.Context, report *Report, label, windowsScript, darwinScript, linuxScript string) bool {
	if err := ctx.Err(); err != nil {
		report.add(LevelWarn, "Skipping %s because the operation was canceled: %v", label, err)
		return false
	}

	script := platformScript(windowsScript, darwinScript, linuxScript)
	if strings.TrimSpace(script) == "" {
		report.add(LevelWarn, "%s is not supported on %s.", label, runtime.GOOS)
		return false
	}

	if err := m.runElevatedScript(ctx, script); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			report.add(LevelWarn, "Elevated %s stopped because the operation ended: %v", label, ctx.Err())
			return false
		}

		report.add(LevelWarn, "Elevated %s failed or was denied: %v. Trying standard-user fallback.", label, err)
		if fallbackErr := m.runStandardScript(ctx, script); fallbackErr != nil {
			report.add(LevelWarn, "Standard-user fallback for %s could not complete: %v", label, fallbackErr)
			return false
		}

		report.add(LevelSuccess, "Standard-user fallback completed for %s.", label)
		return true
	}

	report.add(LevelSuccess, "Elevated %s completed.", label)
	return true
}

func (m NetworkManager) runElevatedScript(ctx context.Context, script string) error {
	if runtime.GOOS == osWindows {
		encoded := encodePowerShellCommand(script)
		wrapper := fmt.Sprintf(`$ErrorActionPreference = "Stop"; $p = Start-Process -FilePath "powershell" -ArgumentList "-NoProfile -ExecutionPolicy Bypass -EncodedCommand %s" -Verb RunAs -Wait -PassThru; if ($null -eq $p) { exit 1 }; if ($p.ExitCode -ne 0) { exit $p.ExitCode }`, encoded)
		return m.commands.Run(ctx, commandPowerShell, powerShellNoProfile, powerShellExecutionPolicy, powerShellBypass, powerShellCommand, wrapper)
	}

	return m.commands.Run(ctx, commandSudo, "-n", commandShell, "-c", script)
}

func (m NetworkManager) runStandardScript(ctx context.Context, script string) error {
	spec := standardScriptCommand(script)
	return m.commands.Run(ctx, spec.name, spec.args...)
}

func platformScriptCommand(windowsScript, darwinScript, linuxScript string) commandSpec {
	switch runtime.GOOS {
	case osWindows:
		return commandSpec{name: commandPowerShell, args: []string{powerShellNoProfile, powerShellExecutionPolicy, powerShellBypass, powerShellCommand, windowsScript}}
	case osDarwin:
		return commandSpec{name: commandShell, args: []string{"-c", darwinScript}}
	case osLinux:
		return commandSpec{name: commandShell, args: []string{"-c", linuxScript}}
	default:
		return commandSpec{}
	}
}

func standardScriptCommand(script string) commandSpec {
	if runtime.GOOS == osWindows {
		return commandSpec{name: commandPowerShell, args: []string{powerShellNoProfile, powerShellExecutionPolicy, powerShellBypass, powerShellCommand, script}}
	}

	return commandSpec{name: commandShell, args: []string{"-c", script}}
}

func platformScript(windowsScript, darwinScript, linuxScript string) string {
	switch runtime.GOOS {
	case osWindows:
		return windowsScript
	case osDarwin:
		return darwinScript
	case osLinux:
		return linuxScript
	default:
		return ""
	}
}

func normalizeConfigOptions(opts ConfigOptions) ConfigOptions {
	defaults := DefaultConfigOptions()
	if strings.TrimSpace(opts.DNSName) == "" {
		opts.DNSName = defaults.DNSName
	}
	if strings.TrimSpace(opts.DNSPrimary) == "" {
		opts.DNSPrimary = defaults.DNSPrimary
	}
	if strings.TrimSpace(opts.DNSSecondary) == "" {
		opts.DNSSecondary = defaults.DNSSecondary
	}
	if opts.MTU <= 0 {
		opts.MTU = defaults.MTU
	}
	return opts
}

func hostsScripts(opts HostsOptions) (string, string, string, string) {
	switch opts.Mode {
	case HostsView:
		return "", "", "", ""
	case HostsAdd:
		ip := strings.TrimSpace(opts.IP)
		if ip == "" {
			ip = "127.0.0.1"
		}
		domain := strings.TrimSpace(opts.Domain)
		if domain == "" {
			domain = "example.local"
		}
		return windowsAddHostsEntryScript(ip, domain), darwinAddHostsEntryScript(ip, domain), linuxAddHostsEntryScript(ip, domain), fmt.Sprintf("hosts entry add (%s -> %s)", domain, ip)
	case HostsRemoveCustom:
		return windowsRemoveCustomHostsScript(), darwinRemoveCustomHostsScript(), linuxRemoveCustomHostsScript(), "hosts custom-entry cleanup"
	case HostsRestore:
		return windowsRestoreHostsScript(), darwinRestoreHostsScript(), linuxRestoreHostsScript(), "hosts backup restore"
	case HostsBackup:
		fallthrough
	default:
		return windowsBackupHostsScript(), darwinBackupHostsScript(), linuxBackupHostsScript(), "hosts backup"
	}
}

func browserModeLabel(mode BrowserMode) string {
	switch mode {
	case BrowserChrome:
		return "Chrome/Chromium"
	case BrowserFirefox:
		return "Firefox"
	case BrowserEdge:
		return "Edge"
	case BrowserBrave:
		return "Brave"
	case BrowserOpera:
		return "Opera"
	default:
		return "all browser"
	}
}

func parsePersistentSettings(output string) PersistentSettings {
	settings := PersistentSettings{}
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "persistentmode":
			settings.Enabled = strings.EqualFold(strings.TrimSpace(value), "true")
		case "dnsprimary":
			settings.DNSPrimary = strings.TrimSpace(value)
		case "dnssecondary":
			settings.DNSSecondary = strings.TrimSpace(value)
		case "dnsname":
			settings.DNSName = strings.TrimSpace(value)
		}
	}
	return settings
}

func (r *Report) add(level Level, format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch level {
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

func (r *Report) addOutput(level Level, output string) {
	output = strings.TrimSpace(output)
	if output == "" {
		r.add(level, "No command output.")
		return
	}

	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		r.add(level, "%s", line)
	}
}

func encodePowerShellCommand(script string) string {
	encoded := utf16.Encode([]rune(script))
	buffer := bytes.NewBuffer(make([]byte, 0, len(encoded)*2))
	for _, value := range encoded {
		_ = binary.Write(buffer, binary.LittleEndian, value)
	}
	return base64.StdEncoding.EncodeToString(buffer.Bytes())
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func powerShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func windowsCurrentConfigScript() string {
	return `
Write-Output "=== Current Network Configuration ==="
$adapters = Get-NetAdapter | Where-Object {$_.Status -eq "Up"}
if (-not $adapters) { Write-Output "No active adapters found." }
foreach ($adapter in $adapters) {
    Write-Output ("Interface: {0}" -f $adapter.Name)
    $dns = Get-DnsClientServerAddress -InterfaceIndex $adapter.InterfaceIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue
    $dnsServers = if ($dns -and $dns.ServerAddresses) { $dns.ServerAddresses -join ", " } else { "None" }
    Write-Output ("DNS Servers: {0}" -f $dnsServers)
    $ip = Get-NetIPAddress -InterfaceIndex $adapter.InterfaceIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object {$_.IPAddress -notlike "169.254.*"}
    if ($ip) { Write-Output ("IP Address: {0}" -f (($ip | Select-Object -ExpandProperty IPAddress) -join ", ")) }
    $mtu = Get-NetIPInterface -InterfaceIndex $adapter.InterfaceIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue
    if ($mtu) { Write-Output ("MTU: {0}" -f $mtu.NlMtu) }
    Write-Output ""
}
$dohStatus = Get-DnsClientDohServerAddress -ErrorAction SilentlyContinue
if ($dohStatus) { Write-Output "DoH Status: Enabled" } else { Write-Output "DoH Status: Disabled" }
$hostsPath = Join-Path $env:SystemRoot "System32\drivers\etc\hosts"
$hostsContent = Get-Content $hostsPath -ErrorAction SilentlyContinue
$customEntries = $hostsContent | Where-Object { $_ -notmatch "^\s*#" -and $_ -match "\S" -and $_ -notmatch "localhost" }
if ($customEntries) { Write-Output ("Custom Hosts Entries: {0}" -f $customEntries.Count) } else { Write-Output "Custom Hosts Entries: None" }
Write-Output ""
Write-Output "Ping probe:"
ping -n 4 google.com
`
}

func darwinCurrentConfigScript() string {
	return `
echo "=== Current Network Configuration ==="
networksetup -listallhardwareports
echo
echo "DNS Servers:"
networksetup -listallnetworkservices | tail -n +2 | sed 's/^\*//' | while IFS= read -r service; do
  [ -z "$service" ] && continue
  echo "Service: $service"
  networksetup -getdnsservers "$service" 2>/dev/null || true
done
echo
echo "IP and MTU:"
ifconfig
echo
echo "Ping probe:"
ping -c 4 google.com
`
}

func linuxCurrentConfigScript() string {
	return `
echo "=== Current Network Configuration ==="
ip addr
echo
echo "DNS Servers:"
if command -v resolvectl >/dev/null 2>&1; then
  resolvectl dns
elif [ -f /etc/resolv.conf ]; then
  grep -E '^[[:space:]]*nameserver[[:space:]]+' /etc/resolv.conf || true
else
  echo "No resolver configuration found."
fi
echo
echo "Ping probe:"
ping -c 4 google.com
`
}

func windowsDiagnosticsScript() string {
	return `
Write-Output "=== Network Diagnostics ==="
$testSites = @("google.com", "cloudflare.com", "github.com")
foreach ($site in $testSites) {
    $ok = Test-NetConnection -ComputerName $site -Port 80 -InformationLevel Quiet
    $status = if ($ok) { "OK" } else { "FAIL" }
    Write-Output ("{0} : {1}" -f $site, $status)
}
Write-Output ""
Write-Output "=== DNS Resolution Test ==="
foreach ($site in $testSites) {
    try {
        $resolved = Resolve-DnsName $site -ErrorAction Stop
        Write-Output ("{0} : {1}" -f $site, $resolved[0].IPAddress)
    } catch {
        Write-Output ("{0} : FAILED" -f $site)
    }
}
Write-Output ""
Write-Output "=== Connection Quality ==="
$ping = Test-NetConnection google.com -InformationLevel Detailed
if ($ping.PingSucceeded) {
    Write-Output ("Ping to Google: {0}ms" -f $ping.PingReplyDetails.RoundtripTime)
} else {
    Write-Output "Ping to Google: FAILED"
}
`
}

func darwinDiagnosticsScript() string {
	return posixDiagnosticsScript("")
}

func linuxDiagnosticsScript() string {
	return posixDiagnosticsScript("-W 2")
}

func posixDiagnosticsScript(pingTimeout string) string {
	return fmt.Sprintf(`
echo "=== Network Diagnostics ==="
for site in google.com cloudflare.com github.com; do
  if ping -c 1 %s "$site" >/dev/null 2>&1; then
    echo "$site : OK"
  else
    echo "$site : FAIL"
  fi
done
echo
echo "=== DNS Resolution Test ==="
for site in google.com cloudflare.com github.com; do
  if command -v getent >/dev/null 2>&1; then
    ip=$(getent hosts "$site" | awk '{print $1; exit}')
  elif command -v dig >/dev/null 2>&1; then
    ip=$(dig +short "$site" | awk 'NF {print; exit}')
  else
    ip=$(nslookup "$site" 2>/dev/null | awk '/^Address: / {print $2; exit}')
  fi
  if [ -n "$ip" ]; then
    echo "$site : $ip"
  else
    echo "$site : FAILED"
  fi
done
echo
echo "=== Connection Quality ==="
ping -c 4 google.com
`, pingTimeout)
}

func windowsSetDNSScript(opts ConfigOptions) string {
	return fmt.Sprintf(`
$ErrorActionPreference = "Stop"
$adapters = Get-NetAdapter | Where-Object {$_.Status -eq "Up"}
if (-not $adapters) { throw "No active adapters found." }
foreach ($adapter in $adapters) {
    Set-DnsClientServerAddress -InterfaceIndex $adapter.InterfaceIndex -ServerAddresses @("%s", "%s")
}
ipconfig /flushdns | Out-Null
Clear-DnsClientCache -ErrorAction SilentlyContinue
`, opts.DNSPrimary, opts.DNSSecondary)
}

func darwinSetDNSScript(opts ConfigOptions) string {
	return fmt.Sprintf(`
set -eu
DNS_PRIMARY=%s
DNS_SECONDARY=%s
networksetup -listallnetworkservices | tail -n +2 | sed 's/^\*//' | while IFS= read -r service; do
  [ -z "$service" ] && continue
  networksetup -setdnsservers "$service" "$DNS_PRIMARY" "$DNS_SECONDARY"
done
dscacheutil -flushcache || true
killall -HUP mDNSResponder >/dev/null 2>&1 || true
`, shellQuote(opts.DNSPrimary), shellQuote(opts.DNSSecondary))
}

func linuxSetDNSScript(opts ConfigOptions) string {
	return fmt.Sprintf(`
set -eu
DNS_PRIMARY=%s
DNS_SECONDARY=%s
DNS_PAIR="$DNS_PRIMARY $DNS_SECONDARY"
if command -v nmcli >/dev/null 2>&1; then
  nmcli -t -f NAME connection show --active | while IFS= read -r conn; do
    [ -z "$conn" ] && continue
    nmcli connection modify "$conn" ipv4.dns "$DNS_PAIR" ipv4.ignore-auto-dns yes
    nmcli connection up "$conn" >/dev/null 2>&1 || true
  done
elif command -v resolvectl >/dev/null 2>&1; then
  ip -o link show up | awk -F': ' '{print $2}' | cut -d@ -f1 | grep -v '^lo$' | while IFS= read -r iface; do
    resolvectl dns "$iface" "$DNS_PRIMARY" "$DNS_SECONDARY"
  done
else
  cp /etc/resolv.conf /etc/resolv.conf.utils.bak 2>/dev/null || true
  printf 'nameserver %%s\nnameserver %%s\n' "$DNS_PRIMARY" "$DNS_SECONDARY" > /etc/resolv.conf
fi
`, shellQuote(opts.DNSPrimary), shellQuote(opts.DNSSecondary))
}

func windowsFlushDNSCacheScript() string {
	return `
$ErrorActionPreference = "Stop"
ipconfig /flushdns | Out-Null
Clear-DnsClientCache -ErrorAction SilentlyContinue
`
}

func darwinFlushDNSCacheScript() string {
	return `
set -eu
dscacheutil -flushcache || true
killall -HUP mDNSResponder >/dev/null 2>&1 || true
`
}

func linuxFlushDNSCacheScript() string {
	return `
set -eu
if command -v resolvectl >/dev/null 2>&1; then
  resolvectl flush-caches
elif command -v systemd-resolve >/dev/null 2>&1; then
  systemd-resolve --flush-caches
elif command -v nscd >/dev/null 2>&1; then
  nscd -i hosts
elif command -v dnsmasq >/dev/null 2>&1; then
  service dnsmasq restart
else
  echo "No supported DNS cache service found."
fi
`
}

func windowsEnableDoHScript() string {
	return `
$ErrorActionPreference = "Stop"
$dnsServers = @(
    @{Server="1.1.1.1"; Template="https://cloudflare-dns.com/dns-query"},
    @{Server="8.8.8.8"; Template="https://dns.google/dns-query"},
    @{Server="9.9.9.9"; Template="https://dns.quad9.net/dns-query"}
)
foreach ($dns in $dnsServers) {
    Add-DnsClientDohServerAddress -ServerAddress $dns.Server -DohTemplate $dns.Template -AllowFallbackToUdp $true -AutoUpgrade $true -ErrorAction Stop
}
`
}

func windowsDisableDoHScript() string {
	return `
$ErrorActionPreference = "Stop"
$dohServers = Get-DnsClientDohServerAddress -ErrorAction SilentlyContinue
if ($dohServers) {
    foreach ($server in $dohServers) {
        Remove-DnsClientDohServerAddress -ServerAddress $server.ServerAddress -ErrorAction SilentlyContinue
    }
}
`
}

func windowsSetMTUScript(mtu int) string {
	return fmt.Sprintf(`
$ErrorActionPreference = "Stop"
$adapters = Get-NetAdapter | Where-Object {$_.Status -eq "Up"}
if (-not $adapters) { throw "No active adapters found." }
foreach ($adapter in $adapters) {
    Set-NetIPInterface -InterfaceIndex $adapter.InterfaceIndex -AddressFamily IPv4 -NlMtu %d -ErrorAction Stop
}
`, mtu)
}

func darwinSetMTUScript(mtu int) string {
	return fmt.Sprintf(`
set -eu
MTU=%d
networksetup -listallhardwareports | awk -F': ' '/Hardware Port:/{print $2}' | while IFS= read -r port; do
  [ -z "$port" ] && continue
  networksetup -setMTU "$port" "$MTU"
done
`, mtu)
}

func linuxSetMTUScript(mtu int) string {
	return fmt.Sprintf(`
set -eu
MTU=%d
ip -o link show up | awk -F': ' '{print $2}' | cut -d@ -f1 | grep -v '^lo$' | while IFS= read -r iface; do
  ip link set dev "$iface" mtu "$MTU"
done
`, mtu)
}

func windowsOptimizeNetworkScript() string {
	return `
$ErrorActionPreference = "Stop"
netsh int tcp set global autotuninglevel=normal | Out-Null
netsh int tcp set global chimney=enabled | Out-Null
netsh int tcp set global rss=enabled | Out-Null
netsh int tcp set global timestamps=disabled | Out-Null
netsh int tcp set global ecncapability=enabled | Out-Null
$adapters = Get-NetAdapter | Where-Object {$_.Status -eq "Up"}
foreach ($adapter in $adapters) {
    Set-NetIPInterface -InterfaceIndex $adapter.InterfaceIndex -AddressFamily IPv4 -NlMtu 1500 -ErrorAction SilentlyContinue
}
`
}

func darwinOptimizeNetworkScript() string {
	return darwinSetMTUScript(1500)
}

func linuxOptimizeNetworkScript() string {
	return `
set -eu
sysctl -w net.ipv4.tcp_moderate_rcvbuf=1 >/dev/null 2>&1 || true
sysctl -w net.ipv4.tcp_timestamps=0 >/dev/null 2>&1 || true
sysctl -w net.ipv4.tcp_ecn=1 >/dev/null 2>&1 || true
ip -o link show up | awk -F': ' '{print $2}' | cut -d@ -f1 | grep -v '^lo$' | while IFS= read -r iface; do
  ip link set dev "$iface" mtu 1500 || true
done
`
}

func windowsResetNetworkOptimizationsScript() string {
	return `
$ErrorActionPreference = "Stop"
netsh int tcp reset | Out-Null
netsh winsock reset | Out-Null
`
}

func darwinResetNetworkOptimizationsScript() string {
	return `
set -eu
networksetup -listallhardwareports | awk -F': ' '/Hardware Port:/{print $2}' | while IFS= read -r port; do
  [ -z "$port" ] && continue
  networksetup -setMTUAndMediaAutomatically "$port" >/dev/null 2>&1 || networksetup -setMTU "$port" 1500 || true
done
`
}

func linuxResetNetworkOptimizationsScript() string {
	return `
set -eu
sysctl -w net.ipv4.tcp_moderate_rcvbuf=1 >/dev/null 2>&1 || true
sysctl -w net.ipv4.tcp_timestamps=1 >/dev/null 2>&1 || true
sysctl -w net.ipv4.tcp_ecn=2 >/dev/null 2>&1 || true
ip -o link show up | awk -F': ' '{print $2}' | cut -d@ -f1 | grep -v '^lo$' | while IFS= read -r iface; do
  ip link set dev "$iface" mtu 1500 || true
done
`
}

func windowsResetDNSScript() string {
	return `
$ErrorActionPreference = "Stop"
$adapters = Get-NetAdapter | Where-Object {$_.Status -eq "Up"}
if (-not $adapters) { throw "No active adapters found." }
foreach ($adapter in $adapters) {
    Set-DnsClientServerAddress -InterfaceIndex $adapter.InterfaceIndex -ResetServerAddresses
}
ipconfig /flushdns | Out-Null
Clear-DnsClientCache -ErrorAction SilentlyContinue
`
}

func darwinResetDNSScript() string {
	return `
set -eu
networksetup -listallnetworkservices | tail -n +2 | sed 's/^\*//' | while IFS= read -r service; do
  [ -z "$service" ] && continue
  networksetup -setdnsservers "$service" Empty
done
dscacheutil -flushcache || true
killall -HUP mDNSResponder >/dev/null 2>&1 || true
`
}

func linuxResetDNSScript() string {
	return `
set -eu
if command -v nmcli >/dev/null 2>&1; then
  nmcli -t -f NAME connection show --active | while IFS= read -r conn; do
    [ -z "$conn" ] && continue
    nmcli connection modify "$conn" ipv4.ignore-auto-dns no ipv4.dns ""
    nmcli connection up "$conn" >/dev/null 2>&1 || true
  done
elif command -v resolvectl >/dev/null 2>&1; then
  ip -o link show up | awk -F': ' '{print $2}' | cut -d@ -f1 | grep -v '^lo$' | while IFS= read -r iface; do
    resolvectl revert "$iface" || true
  done
elif [ -f /etc/resolv.conf.utils.bak ]; then
  cp /etc/resolv.conf.utils.bak /etc/resolv.conf
else
  echo "No supported DNS reset mechanism found."
fi
`
}

func windowsViewHostsScript() string {
	return `
$hostsPath = Join-Path $env:SystemRoot "System32\drivers\etc\hosts"
Write-Output ("Hosts path: {0}" -f $hostsPath)
Get-Content $hostsPath -ErrorAction SilentlyContinue
`
}

func darwinViewHostsScript() string {
	return posixViewHostsScript()
}

func linuxViewHostsScript() string {
	return posixViewHostsScript()
}

func posixViewHostsScript() string {
	return `
echo "Hosts path: /etc/hosts"
cat /etc/hosts
`
}

func windowsBackupHostsScript() string {
	return `
$ErrorActionPreference = "Stop"
$hostsPath = Join-Path $env:SystemRoot "System32\drivers\etc\hosts"
Copy-Item -Path $hostsPath -Destination "$hostsPath.backup" -Force
`
}

func darwinBackupHostsScript() string {
	return posixBackupHostsScript()
}

func linuxBackupHostsScript() string {
	return posixBackupHostsScript()
}

func posixBackupHostsScript() string {
	return `
set -eu
cp /etc/hosts /etc/hosts.backup
`
}

func windowsRestoreHostsScript() string {
	return `
$ErrorActionPreference = "Stop"
$hostsPath = Join-Path $env:SystemRoot "System32\drivers\etc\hosts"
Copy-Item -Path "$hostsPath.backup" -Destination $hostsPath -Force
`
}

func darwinRestoreHostsScript() string {
	return posixRestoreHostsScript()
}

func linuxRestoreHostsScript() string {
	return posixRestoreHostsScript()
}

func posixRestoreHostsScript() string {
	return `
set -eu
cp /etc/hosts.backup /etc/hosts
`
}

func windowsAddHostsEntryScript(ip, domain string) string {
	return fmt.Sprintf(`
$ErrorActionPreference = "Stop"
$hostsPath = Join-Path $env:SystemRoot "System32\drivers\etc\hosts"
Add-Content -Path $hostsPath -Value (%s + [char]9 + %s)
`, powerShellQuote(ip), powerShellQuote(domain))
}

func darwinAddHostsEntryScript(ip, domain string) string {
	return posixAddHostsEntryScript(ip, domain)
}

func linuxAddHostsEntryScript(ip, domain string) string {
	return posixAddHostsEntryScript(ip, domain)
}

func posixAddHostsEntryScript(ip, domain string) string {
	return fmt.Sprintf(`
set -eu
printf '\n%%s\t%%s\n' %s %s >> /etc/hosts
`, shellQuote(ip), shellQuote(domain))
}

func windowsRemoveCustomHostsScript() string {
	return `
$ErrorActionPreference = "Stop"
$hostsPath = Join-Path $env:SystemRoot "System32\drivers\etc\hosts"
$backup = Get-Content $hostsPath
$defaultContent = $backup | Where-Object { $_ -match "^\s*#" -or $_ -match "localhost" -or $_ -notmatch "\S" }
Set-Content -Path $hostsPath -Value $defaultContent
`
}

func darwinRemoveCustomHostsScript() string {
	return posixRemoveCustomHostsScript()
}

func linuxRemoveCustomHostsScript() string {
	return posixRemoveCustomHostsScript()
}

func posixRemoveCustomHostsScript() string {
	return `
set -eu
tmp=$(mktemp)
awk '/^[[:space:]]*#/ || /localhost/ || /^[[:space:]]*$/ {print}' /etc/hosts > "$tmp"
cat "$tmp" > /etc/hosts
rm -f "$tmp"
`
}

func windowsPersistentStatusScript() string {
	return `
$regPath = "HKCU:\Software\NetworkConfigTool"
if (Test-Path $regPath) {
    $props = Get-ItemProperty -Path $regPath -ErrorAction SilentlyContinue
    Write-Output ("PersistentMode={0}" -f [bool]$props.PersistentMode)
    Write-Output ("DNSPrimary={0}" -f $props.DNSPrimary)
    Write-Output ("DNSSecondary={0}" -f $props.DNSSecondary)
    Write-Output ("DNSName={0}" -f $props.DNSName)
} else {
    Write-Output "PersistentMode=False"
}
`
}

func darwinPersistentStatusScript() string {
	return posixPersistentStatusScript()
}

func linuxPersistentStatusScript() string {
	return posixPersistentStatusScript()
}

func posixPersistentStatusScript() string {
	return `
path="${HOME}/.config/utils/network-persistent.conf"
if [ -f "$path" ]; then
  cat "$path"
else
  echo "PersistentMode=False"
fi
`
}

func windowsSavePersistentSettingsScript(opts ConfigOptions) string {
	return fmt.Sprintf(`
$ErrorActionPreference = "Stop"
$regPath = "HKCU:\Software\NetworkConfigTool"
if (-not (Test-Path $regPath)) {
    New-Item -Path $regPath -Force | Out-Null
}
Set-ItemProperty -Path $regPath -Name "DNSPrimary" -Value %s
Set-ItemProperty -Path $regPath -Name "DNSSecondary" -Value %s
Set-ItemProperty -Path $regPath -Name "DNSName" -Value %s
Set-ItemProperty -Path $regPath -Name "PersistentMode" -Value $true
`, powerShellQuote(opts.DNSPrimary), powerShellQuote(opts.DNSSecondary), powerShellQuote(opts.DNSName))
}

func darwinSavePersistentSettingsScript(opts ConfigOptions) string {
	return posixSavePersistentSettingsScript(opts)
}

func linuxSavePersistentSettingsScript(opts ConfigOptions) string {
	return posixSavePersistentSettingsScript(opts)
}

func posixSavePersistentSettingsScript(opts ConfigOptions) string {
	lines := []string{
		"PersistentMode=True",
		"DNSPrimary=" + opts.DNSPrimary,
		"DNSSecondary=" + opts.DNSSecondary,
		"DNSName=" + opts.DNSName,
	}
	quoted := make([]string, 0, len(lines))
	for _, line := range lines {
		quoted = append(quoted, shellQuote(line))
	}
	return fmt.Sprintf(`
set -eu
dir="${HOME}/.config/utils"
mkdir -p "$dir"
printf '%%s\n' %s > "$dir/network-persistent.conf"
`, strings.Join(quoted, " "))
}

func windowsClearPersistentSettingsScript() string {
	return `
$regPath = "HKCU:\Software\NetworkConfigTool"
if (Test-Path $regPath) {
    Remove-Item -Path $regPath -Recurse -Force
}
`
}

func darwinClearPersistentSettingsScript() string {
	return posixClearPersistentSettingsScript()
}

func linuxClearPersistentSettingsScript() string {
	return posixClearPersistentSettingsScript()
}

func posixClearPersistentSettingsScript() string {
	return `
rm -f "${HOME}/.config/utils/network-persistent.conf"
`
}

func windowsBrowserCacheScript(mode BrowserMode) string {
	sections := windowsBrowserCacheSections(mode)
	return fmt.Sprintf(`
function Clear-CachePath {
    param([string]$Path)
    if (Test-Path $Path) {
        Get-ChildItem -LiteralPath $Path -Force -ErrorAction SilentlyContinue | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue
        Write-Output ("Cleared: {0}" -f $Path)
    } else {
        Write-Output ("Not found: {0}" -f $Path)
    }
}
%s
`, sections)
}

func windowsBrowserCacheSections(mode BrowserMode) string {
	var b strings.Builder
	addChrome := func() {
		b.WriteString(`
Clear-CachePath "$env:LOCALAPPDATA\Google\Chrome\User Data\Default\Cache"
Clear-CachePath "$env:LOCALAPPDATA\Google\Chrome\User Data\Default\Code Cache"
Clear-CachePath "$env:LOCALAPPDATA\Chromium\User Data\Default\Cache"
Clear-CachePath "$env:LOCALAPPDATA\Chromium\User Data\Default\Code Cache"
`)
	}
	addFirefox := func() {
		b.WriteString(`
$firefoxRoot = "$env:LOCALAPPDATA\Mozilla\Firefox\Profiles"
if (Test-Path $firefoxRoot) {
    Get-ChildItem $firefoxRoot -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -match ".*\.default.*" } | ForEach-Object {
        Clear-CachePath (Join-Path $_.FullName "cache2")
    }
} else {
    Write-Output ("Not found: {0}" -f $firefoxRoot)
}
`)
	}
	addEdge := func() {
		b.WriteString(`
Clear-CachePath "$env:LOCALAPPDATA\Microsoft\Edge\User Data\Default\Cache"
Clear-CachePath "$env:LOCALAPPDATA\Microsoft\Edge\User Data\Default\Code Cache"
`)
	}
	addBrave := func() {
		b.WriteString(`
Clear-CachePath "$env:LOCALAPPDATA\BraveSoftware\Brave-Browser\User Data\Default\Cache"
Clear-CachePath "$env:LOCALAPPDATA\BraveSoftware\Brave-Browser\User Data\Default\Code Cache"
`)
	}
	addOpera := func() {
		b.WriteString(`
Clear-CachePath "$env:LOCALAPPDATA\Opera Software\Opera Stable\Cache"
Clear-CachePath "$env:LOCALAPPDATA\Opera Software\Opera Stable\Code Cache"
`)
	}

	switch mode {
	case BrowserChrome:
		addChrome()
	case BrowserFirefox:
		addFirefox()
	case BrowserEdge:
		addEdge()
	case BrowserBrave:
		addBrave()
	case BrowserOpera:
		addOpera()
	default:
		addChrome()
		addFirefox()
		addEdge()
		addBrave()
		addOpera()
	}
	return b.String()
}

func darwinBrowserCacheScript(mode BrowserMode) string {
	return posixBrowserCacheScript(mode, true)
}

func linuxBrowserCacheScript(mode BrowserMode) string {
	return posixBrowserCacheScript(mode, false)
}

func posixBrowserCacheScript(mode BrowserMode, darwin bool) string {
	sections := posixBrowserCacheSections(mode, darwin)
	return fmt.Sprintf(`
clear_path() {
  path="$1"
  if [ -d "$path" ]; then
    rm -rf "$path"/* 2>/dev/null || true
    echo "Cleared: $path"
  else
    echo "Not found: $path"
  fi
}
%s
`, sections)
}

func posixBrowserCacheSections(mode BrowserMode, darwin bool) string {
	var b strings.Builder
	add := func(script string) {
		b.WriteString(script)
	}
	addChrome := func() {
		if darwin {
			add(`
clear_path "$HOME/Library/Caches/Google/Chrome/Default"
clear_path "$HOME/Library/Application Support/Google/Chrome/Default/Code Cache"
clear_path "$HOME/Library/Caches/Chromium/Default"
clear_path "$HOME/Library/Application Support/Chromium/Default/Code Cache"
`)
			return
		}
		add(`
clear_path "$HOME/.cache/google-chrome/Default/Cache"
clear_path "$HOME/.cache/google-chrome/Default/Code Cache"
clear_path "$HOME/.cache/chromium/Default/Cache"
clear_path "$HOME/.cache/chromium/Default/Code Cache"
`)
	}
	addFirefox := func() {
		if darwin {
			add(`
for path in "$HOME"/Library/Caches/Firefox/Profiles/*/cache2 "$HOME"/Library/Caches/Mozilla/Firefox/Profiles/*/cache2; do
  [ -d "$path" ] && clear_path "$path"
done
`)
			return
		}
		add(`
for path in "$HOME"/.cache/mozilla/firefox/*.default*/cache2; do
  [ -d "$path" ] && clear_path "$path"
done
`)
	}
	addEdge := func() {
		if darwin {
			add(`
clear_path "$HOME/Library/Caches/Microsoft Edge/Default"
clear_path "$HOME/Library/Application Support/Microsoft Edge/Default/Code Cache"
`)
			return
		}
		add(`
clear_path "$HOME/.cache/microsoft-edge/Default/Cache"
clear_path "$HOME/.cache/microsoft-edge/Default/Code Cache"
`)
	}
	addBrave := func() {
		if darwin {
			add(`
clear_path "$HOME/Library/Caches/BraveSoftware/Brave-Browser/Default"
clear_path "$HOME/Library/Application Support/BraveSoftware/Brave-Browser/Default/Code Cache"
`)
			return
		}
		add(`
clear_path "$HOME/.cache/BraveSoftware/Brave-Browser/Default/Cache"
clear_path "$HOME/.cache/BraveSoftware/Brave-Browser/Default/Code Cache"
`)
	}
	addOpera := func() {
		if darwin {
			add(`
clear_path "$HOME/Library/Caches/com.operasoftware.Opera"
clear_path "$HOME/Library/Application Support/com.operasoftware.Opera/Code Cache"
`)
			return
		}
		add(`
clear_path "$HOME/.cache/opera"
clear_path "$HOME/.config/opera/Code Cache"
`)
	}

	switch mode {
	case BrowserChrome:
		addChrome()
	case BrowserFirefox:
		addFirefox()
	case BrowserEdge:
		addEdge()
	case BrowserBrave:
		addBrave()
	case BrowserOpera:
		addOpera()
	default:
		addChrome()
		addFirefox()
		addEdge()
		addBrave()
		addOpera()
	}
	return b.String()
}

const (
	osWindows = "windows"
	osDarwin  = "darwin"
	osLinux   = "linux"
)
