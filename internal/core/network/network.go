package network

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
)

type Level string

const (
	LevelInfo    Level = "INFO"
	LevelWarn    Level = "WARN"
	LevelSuccess Level = "SUCCESS"
	LevelError   Level = "ERROR"
)

const HostsManagedMarker = "# utils-managed"

const (
	hostsManagedTag   = "utils-managed"
	posixHostsPath    = "/etc/hosts"
	backupStampLayout = "20060102-150405.000000000"
)

var errUnsupported = errors.New("not supported on " + runtime.GOOS)

type Entry struct {
	Time    time.Time
	Level   Level
	Message string
}

type Report struct {
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
	now      func() time.Time
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

func HostsPath() string {
	if runtime.GOOS == osWindows {
		return `%SystemRoot%\System32\drivers\etc\hosts`
	}
	return posixHostsPath
}

func ElevationNote() string {
	if runtime.GOOS == osWindows {
		return "Write steps run in an elevated PowerShell after you accept the Windows UAC prompt."
	}
	return "Write steps run through sudo -n, which never prompts: run sudo -v in this terminal first so sudo has cached credentials."
}

func ValidateHostsEntry(ip, domain string) error {
	if _, err := parseIPAddress(ip); err != nil {
		return fmt.Errorf("hosts IP %q is not a valid IP address", ip)
	}
	if domain == "" || len(domain) > 253 {
		return fmt.Errorf("hosts domain must be 1 to 253 characters long")
	}
	if strings.HasPrefix(domain, "-") || strings.HasPrefix(domain, ".") {
		return fmt.Errorf("hosts domain %q must start with a letter or digit", domain)
	}
	for _, character := range domain {
		isLetterOrDigit := ('a' <= character && character <= 'z') || ('A' <= character && character <= 'Z') || ('0' <= character && character <= '9')
		if !isLetterOrDigit && character != '.' && character != '-' && character != '_' {
			return fmt.Errorf("hosts domain %q may only contain ASCII letters, digits, '.', '-' and '_'", domain)
		}
	}
	return nil
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
	if err := validateConfigOptions(opts); err != nil {
		report.finish(err, "")
		return report, err
	}
	report.add(LevelInfo, "Applying %s DNS (%s, %s).", opts.DNSName, opts.DNSPrimary, opts.DNSSecondary)
	report.add(LevelInfo, "%s", ElevationNote())

	err := m.applyDNS(ctx, &report, opts)
	if err == nil && opts.Persistent {
		err = m.savePersistentSettings(ctx, &report, opts)
	}

	report.finish(err, fmt.Sprintf("%s DNS applied.", opts.DNSName))
	return report, err
}

func (m NetworkManager) ApplyConfig(ctx context.Context, opts ConfigOptions) (Report, error) {
	m = m.withDefaults()
	opts = normalizeConfigOptions(opts)
	report := Report{Operation: "Apply Network Config"}
	if err := validateConfigOptions(opts); err != nil {
		report.finish(err, "")
		return report, err
	}
	report.add(LevelInfo, "Applying %s DNS (%s, %s), DoH=%t, MTU=%d.", opts.DNSName, opts.DNSPrimary, opts.DNSSecondary, opts.EnableDoH, opts.MTU)
	report.add(LevelInfo, "%s", ElevationNote())

	err := m.applyDNS(ctx, &report, opts)
	if err == nil && opts.Persistent {
		err = m.savePersistentSettings(ctx, &report, opts)
	}
	if err == nil && opts.EnableDoH {
		err = skipUnsupported(&report, m.enableDoH(ctx, &report))
	}
	if err == nil {
		err = m.applyMTU(ctx, &report, opts.MTU)
	}

	report.finish(err, "Network configuration applied. Restart may be required for full effect.")
	return report, err
}

func (m NetworkManager) FlushDNSCache(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Flush DNS Cache"}
	err := m.flushDNS(ctx, &report)
	report.finish(err, "DNS cache flushed.")
	return report, err
}

func (m NetworkManager) EnableDoH(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Enable DNS over HTTPS"}
	err := m.enableDoH(ctx, &report)
	report.finish(err, "DNS over HTTPS templates registered.")
	return report, err
}

func (m NetworkManager) DisableDoH(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Disable DNS over HTTPS"}
	err := m.disableDoH(ctx, &report)
	report.finish(err, "DNS over HTTPS templates removed.")
	return report, err
}

func (m NetworkManager) OptimizeNetworkSettings(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Optimize Network Settings"}
	if runtime.GOOS != osWindows {
		report.add(LevelWarn, "Windows-specific TCP knobs from tool.ps1 are not available on %s; applying MTU and available TCP equivalents where the OS supports them.", runtime.GOOS)
	}
	err := m.runPrivileged(ctx, &report, "network optimization", windowsOptimizeNetworkScript(), darwinOptimizeNetworkScript(), linuxOptimizeNetworkScript())
	report.finish(err, "Network optimization applied. Restart may be required for full effect.")
	return report, err
}

func (m NetworkManager) ResetNetworkOptimizations(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Reset Network Optimizations"}
	if runtime.GOOS != osWindows {
		report.add(LevelWarn, "Winsock/TCP reset is Windows-specific; applying best-effort reset commands available on %s.", runtime.GOOS)
	}
	err := m.runPrivileged(ctx, &report, "network optimization reset", windowsResetNetworkOptimizationsScript(), darwinResetNetworkOptimizationsScript(), linuxResetNetworkOptimizationsScript())
	report.finish(err, "Network optimizations reset. Restart may be required.")
	return report, err
}

func (m NetworkManager) ResetDNS(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Reset DNS to Automatic"}
	err := m.resetDNS(ctx, &report)
	report.finish(err, "DNS reset to automatic.")
	return report, err
}

func (m NetworkManager) ResetToDefaults(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Reset Network Settings to Defaults"}
	report.add(LevelInfo, "Resetting DNS to automatic, disabling DoH where supported, and clearing persistent DNS settings.")
	err := m.resetDNS(ctx, &report)
	if err == nil {
		err = skipUnsupported(&report, m.disableDoH(ctx, &report))
	}
	if err == nil {
		err = m.clearPersistentSettings(ctx, &report)
	}
	report.finish(err, "Network settings reset to defaults.")
	return report, err
}

func (m NetworkManager) ClearBrowserCache(ctx context.Context, mode BrowserMode) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Clear Browser Cache"}
	report.add(LevelWarn, "Close browsers before clearing cache so locked files can be removed.")
	report.add(LevelInfo, "Clearing %s cache with standard user permissions.", browserModeLabel(mode))

	output, err := m.outputPlatformScript(ctx, windowsBrowserCacheScript(mode), darwinBrowserCacheScript(mode), linuxBrowserCacheScript(mode))
	report.addOutput(LevelInfo, output)
	if err != nil {
		err = commandError(output, err)
	}
	report.finish(err, "Browser cache cleanup completed.")
	return report, err
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

	windowsScript, darwinScript, linuxScript, label, err := hostsScripts(opts, m.now().Format(backupStampLayout))
	if err != nil {
		report.finish(err, "")
		return report, err
	}

	report.add(LevelInfo, "Attempting hosts operation: %s.", label)
	err = m.runPrivileged(ctx, &report, label, windowsScript, darwinScript, linuxScript)
	report.finish(err, fmt.Sprintf("Hosts operation completed: %s.", label))
	return report, err
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
		err := m.clearPersistentSettings(ctx, &report)
		report.finish(err, "Persistent DNS mode disabled.")
		return report, err
	}

	err := validateConfigOptions(opts)
	if err == nil {
		err = m.savePersistentSettings(ctx, &report, opts)
	}
	report.finish(err, fmt.Sprintf("Persistent DNS mode enabled with %s (%s, %s). DNS preset actions can update this saved value.", opts.DNSName, opts.DNSPrimary, opts.DNSSecondary))
	return report, err
}

func (m NetworkManager) ApplyPersistentSettings(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Apply Persistent DNS Settings"}
	settings, err := m.loadPersistentSettings(ctx, &report)
	if err != nil {
		err = fmt.Errorf("could not load persistent settings: %w", err)
		report.finish(err, "")
		return report, err
	}
	if !settings.Enabled || settings.DNSPrimary == "" || settings.DNSSecondary == "" {
		err = errors.New("no persistent DNS settings are saved")
		report.finish(err, "")
		return report, err
	}

	opts := ConfigOptions{
		DNSName:      settings.DNSName,
		DNSPrimary:   settings.DNSPrimary,
		DNSSecondary: settings.DNSSecondary,
		MTU:          1500,
	}
	if err := validateConfigOptions(opts); err != nil {
		err = fmt.Errorf("saved persistent DNS settings were rejected: %w", err)
		report.finish(err, "")
		return report, err
	}
	err = m.applyDNS(ctx, &report, opts)
	report.finish(err, fmt.Sprintf("Persistent %s DNS applied.", opts.DNSName))
	return report, err
}

func (m NetworkManager) ClearPersistentSettings(ctx context.Context) (Report, error) {
	m = m.withDefaults()
	report := Report{Operation: "Clear Persistent DNS Settings"}
	err := m.clearPersistentSettings(ctx, &report)
	report.finish(err, "Persistent DNS settings cleared.")
	return report, err
}

func (m NetworkManager) withDefaults() NetworkManager {
	if m.commands == nil {
		m.commands = execCommandRunner{}
	}
	if m.now == nil {
		m.now = time.Now
	}
	return m
}

func (m NetworkManager) applyDNS(ctx context.Context, report *Report, opts ConfigOptions) error {
	return m.runPrivileged(ctx, report, fmt.Sprintf("%s DNS configuration", opts.DNSName), windowsSetDNSScript(opts), darwinSetDNSScript(opts), linuxSetDNSScript(opts))
}

func (m NetworkManager) flushDNS(ctx context.Context, report *Report) error {
	return m.runPrivileged(ctx, report, "DNS cache flush", windowsFlushDNSCacheScript(), darwinFlushDNSCacheScript(), linuxFlushDNSCacheScript())
}

func (m NetworkManager) enableDoH(ctx context.Context, report *Report) error {
	return m.runPrivileged(ctx, report, "DNS over HTTPS configuration", windowsEnableDoHScript(), "", "")
}

func (m NetworkManager) disableDoH(ctx context.Context, report *Report) error {
	return m.runPrivileged(ctx, report, "DNS over HTTPS removal", windowsDisableDoHScript(), "", "")
}

func (m NetworkManager) applyMTU(ctx context.Context, report *Report, mtu int) error {
	return m.runPrivileged(ctx, report, "MTU configuration", windowsSetMTUScript(mtu), darwinSetMTUScript(mtu), linuxSetMTUScript(mtu))
}

func (m NetworkManager) resetDNS(ctx context.Context, report *Report) error {
	return m.runPrivileged(ctx, report, "DNS reset", windowsResetDNSScript(), darwinResetDNSScript(), linuxResetDNSScript())
}

func (m NetworkManager) savePersistentSettings(ctx context.Context, report *Report, opts ConfigOptions) error {
	output, err := m.runStandardScript(ctx, platformScript(windowsSavePersistentSettingsScript(opts), darwinSavePersistentSettingsScript(opts), linuxSavePersistentSettingsScript(opts)))
	if err != nil {
		return fmt.Errorf("could not save persistent DNS settings: %w", commandError(output, err))
	}
	report.add(LevelInfo, "Saved persistent DNS settings: %s (%s, %s).", opts.DNSName, opts.DNSPrimary, opts.DNSSecondary)
	return nil
}

func (m NetworkManager) clearPersistentSettings(ctx context.Context, report *Report) error {
	output, err := m.runStandardScript(ctx, platformScript(windowsClearPersistentSettingsScript(), darwinClearPersistentSettingsScript(), linuxClearPersistentSettingsScript()))
	if err != nil {
		return fmt.Errorf("could not clear persistent DNS settings: %w", commandError(output, err))
	}
	report.add(LevelInfo, "Cleared persistent DNS settings.")
	return nil
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

func (m NetworkManager) runPrivileged(ctx context.Context, report *Report, label, windowsScript, darwinScript, linuxScript string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s was not started: %w", label, err)
	}

	script := platformScript(windowsScript, darwinScript, linuxScript)
	if strings.TrimSpace(script) == "" {
		return fmt.Errorf("%s: %w", label, errUnsupported)
	}

	output, err := m.runElevatedScript(ctx, script)
	if strings.TrimSpace(output) != "" {
		report.addOutput(LevelInfo, output)
	}
	if err != nil {
		return fmt.Errorf("%s failed: %w", label, err)
	}

	report.add(LevelInfo, "Elevated %s completed.", label)
	return nil
}

func (m NetworkManager) runElevatedScript(ctx context.Context, script string) (string, error) {
	if runtime.GOOS == osWindows {
		return m.runWindowsElevatedScript(ctx, script)
	}

	output, err := m.commands.Output(ctx, commandSudo, "-n", commandShell, "-c", script)
	if err == nil {
		return string(output), nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return string(output), fmt.Errorf("stopped because the operation ended: %w", ctxErr)
	}
	if _, probeErr := m.commands.Output(ctx, commandSudo, "-n", "true"); probeErr != nil {
		return string(output), fmt.Errorf("sudo -n could not get root without a password (%w); run sudo -v in this terminal, then retry", commandError(string(output), err))
	}
	return string(output), commandError(string(output), err)
}

func (m NetworkManager) runWindowsElevatedScript(ctx context.Context, script string) (string, error) {
	logDir, err := os.MkdirTemp("", "utils-network-")
	if err != nil {
		return "", fmt.Errorf("create elevated log directory: %w", err)
	}
	defer os.RemoveAll(logDir)

	logPath := filepath.Join(logDir, "elevated.log")
	encoded := encodePowerShellCommand(windowsElevatedScript(script, logPath))
	wrapper := fmt.Sprintf(`$ErrorActionPreference = "Stop"; $p = Start-Process -FilePath "powershell" -ArgumentList "-NoProfile -ExecutionPolicy Bypass -EncodedCommand %s" -Verb RunAs -Wait -PassThru; if ($null -eq $p) { exit 1 }; if ($p.ExitCode -ne 0) { exit $p.ExitCode }`, encoded)
	wrapperOutput, err := m.commands.Output(ctx, commandPowerShell, powerShellNoProfile, powerShellExecutionPolicy, powerShellBypass, powerShellCommand, wrapper)

	elevatedOutput, _ := os.ReadFile(logPath)
	output := strings.TrimSpace(strings.Join([]string{string(elevatedOutput), string(wrapperOutput)}, "\n"))
	if err == nil {
		return output, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return output, fmt.Errorf("stopped waiting because the operation ended (an accepted elevated window may still be running): %w", ctxErr)
	}
	return output, fmt.Errorf("administrator elevation was denied or the elevated script failed: %w", commandError(output, err))
}

func (m NetworkManager) runStandardScript(ctx context.Context, script string) (string, error) {
	spec := standardScriptCommand(script)
	output, err := m.commands.Output(ctx, spec.name, spec.args...)
	return string(output), err
}

func windowsElevatedScript(script, logPath string) string {
	return fmt.Sprintf(`
$ErrorActionPreference = "Stop"
$logWriter = New-Object System.IO.StreamWriter([System.IO.File]::Open(%s, [System.IO.FileMode]::CreateNew, [System.IO.FileAccess]::Write))
$exitCode = 0
try {
    & {
%s
    } *>&1 | ForEach-Object { $logWriter.WriteLine([string]$_) }
} catch {
    $logWriter.WriteLine([string]$_)
    $exitCode = 1
} finally {
    $logWriter.Dispose()
}
exit $exitCode
`, powerShellQuote(logPath), script)
}

func commandError(output string, err error) error {
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(output, "\r\n", "\n")), "\n")
	lastLine := strings.TrimSpace(lines[len(lines)-1])
	if lastLine == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, lastLine)
}

func skipUnsupported(report *Report, err error) error {
	if errors.Is(err, errUnsupported) {
		report.add(LevelWarn, "Skipped: %v.", err)
		return nil
	}
	return err
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
	opts.DNSName = strings.TrimSpace(opts.DNSName)
	opts.DNSPrimary = strings.TrimSpace(opts.DNSPrimary)
	opts.DNSSecondary = strings.TrimSpace(opts.DNSSecondary)
	if opts.DNSName == "" {
		opts.DNSName = defaults.DNSName
	}
	if opts.DNSPrimary == "" {
		opts.DNSPrimary = defaults.DNSPrimary
	}
	if opts.DNSSecondary == "" {
		opts.DNSSecondary = defaults.DNSSecondary
	}
	if opts.MTU <= 0 {
		opts.MTU = defaults.MTU
	}
	return opts
}

func validateConfigOptions(opts ConfigOptions) error {
	if _, err := parseIPAddress(opts.DNSPrimary); err != nil {
		return fmt.Errorf("primary DNS %q is not a valid IP address", opts.DNSPrimary)
	}
	if _, err := parseIPAddress(opts.DNSSecondary); err != nil {
		return fmt.Errorf("secondary DNS %q is not a valid IP address", opts.DNSSecondary)
	}
	if strings.IndexFunc(opts.DNSName, unicode.IsControl) >= 0 {
		return fmt.Errorf("DNS name %q contains control characters", opts.DNSName)
	}
	return nil
}

func parseIPAddress(value string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, err
	}
	if addr.Zone() != "" {
		return netip.Addr{}, fmt.Errorf("IP address %q has a zone", value)
	}
	return addr, nil
}

func hostsScripts(opts HostsOptions, stamp string) (string, string, string, string, error) {
	switch opts.Mode {
	case HostsView:
		return "", "", "", "", errors.New("viewing the hosts file is not a write operation")
	case HostsAdd:
		ip := strings.TrimSpace(opts.IP)
		if ip == "" {
			ip = "127.0.0.1"
		}
		domain := strings.TrimSpace(opts.Domain)
		if err := ValidateHostsEntry(ip, domain); err != nil {
			return "", "", "", "", err
		}
		return windowsAddHostsEntryScript(ip, domain), posixAddHostsEntryScript(posixHostsPath, ip, domain), posixAddHostsEntryScript(posixHostsPath, ip, domain), fmt.Sprintf("hosts entry add (%s -> %s)", domain, ip), nil
	case HostsRemoveCustom:
		return windowsRemoveManagedHostsScript(), posixRemoveManagedHostsScript(posixHostsPath), posixRemoveManagedHostsScript(posixHostsPath), "hosts utils-managed entry cleanup", nil
	case HostsRestore:
		return windowsRestoreHostsScript(stamp), posixRestoreHostsScript(posixHostsPath, stamp), posixRestoreHostsScript(posixHostsPath, stamp), "hosts restore from newest backup", nil
	case HostsBackup:
		fallthrough
	default:
		return windowsBackupHostsScript(stamp), posixBackupHostsScript(posixHostsPath, stamp), posixBackupHostsScript(posixHostsPath, stamp), "hosts backup", nil
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

func (r *Report) finish(err error, successMessage string) {
	if err != nil {
		r.add(LevelError, "%s failed: %v", r.Operation, err)
		return
	}
	r.add(LevelSuccess, "%s", successMessage)
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
    Set-DnsClientServerAddress -InterfaceIndex $adapter.InterfaceIndex -ServerAddresses @(%s, %s)
    Write-Output ("Set DNS on {0}" -f $adapter.Name)
}
ipconfig /flushdns | Out-Null
if ($LASTEXITCODE -ne 0) { throw "ipconfig /flushdns failed with exit code $LASTEXITCODE" }
Clear-DnsClientCache
`, powerShellQuote(opts.DNSPrimary), powerShellQuote(opts.DNSSecondary))
}

const darwinNetworkServicesSnippet = `listing=$(networksetup -listallnetworkservices)
services=$(printf '%s\n' "$listing" | tail -n +2 | sed 's/^\*//')
if [ -z "$services" ]; then
  echo "No network services found." >&2
  exit 1
fi`

const darwinHardwarePortsSnippet = `listing=$(networksetup -listallhardwareports)
ports=$(printf '%s\n' "$listing" | awk -F': ' '/Hardware Port:/{print $2}')
if [ -z "$ports" ]; then
  echo "No hardware ports found." >&2
  exit 1
fi`

const linuxActiveLinksSnippet = `listing=$(ip -o link show up)
links=$(printf '%s\n' "$listing" | awk -F': ' '{print $2}' | cut -d@ -f1 | grep -v '^lo$' || true)
if [ -z "$links" ]; then
  echo "No active network interface found." >&2
  exit 1
fi`

const linuxActiveConnectionsSnippet = `connections=$(nmcli -t -f UUID,TYPE connection show --active)
if [ -z "$connections" ]; then
  echo "No active NetworkManager connection found." >&2
  exit 1
fi`

func darwinSetDNSScript(opts ConfigOptions) string {
	return fmt.Sprintf(`
set -eu
DNS_PRIMARY=%s
DNS_SECONDARY=%s
%s
while IFS= read -r service; do
  [ -z "$service" ] && continue
  networksetup -setdnsservers "$service" "$DNS_PRIMARY" "$DNS_SECONDARY"
  echo "Set DNS on $service"
done <<EOF
$services
EOF
dscacheutil -flushcache
killall -HUP mDNSResponder
`, shellQuote(opts.DNSPrimary), shellQuote(opts.DNSSecondary), darwinNetworkServicesSnippet)
}

func linuxSetDNSScript(opts ConfigOptions) string {
	return fmt.Sprintf(`
set -eu
DNS_PRIMARY=%s
DNS_SECONDARY=%s
if command -v nmcli >/dev/null 2>&1; then
  %s
  changed=0
  while IFS=: read -r uuid type; do
    if [ "$type" = loopback ]; then
      continue
    fi
    nmcli connection modify "$uuid" ipv4.dns "$DNS_PRIMARY $DNS_SECONDARY" ipv4.ignore-auto-dns yes
    nmcli connection up "$uuid" >/dev/null
    echo "Set DNS on NetworkManager connection $uuid ($type)"
    changed=$((changed + 1))
  done <<EOF
$connections
EOF
  if [ "$changed" -eq 0 ]; then
    echo "No active NetworkManager connection besides loopback." >&2
    exit 1
  fi
elif command -v resolvectl >/dev/null 2>&1; then
  %s
  for link in $links; do
    resolvectl dns "$link" "$DNS_PRIMARY" "$DNS_SECONDARY"
    echo "Set DNS on $link through systemd-resolved (runtime only, lost on reboot)"
  done
else
  if [ ! -e /etc/resolv.conf.utils.bak ]; then
    cp -p /etc/resolv.conf /etc/resolv.conf.utils.bak
    echo "Saved the original /etc/resolv.conf as /etc/resolv.conf.utils.bak"
  fi
  printf 'nameserver %%s\nnameserver %%s\n' "$DNS_PRIMARY" "$DNS_SECONDARY" > /etc/resolv.conf
  echo "Wrote /etc/resolv.conf"
fi
`, shellQuote(opts.DNSPrimary), shellQuote(opts.DNSSecondary), linuxActiveConnectionsSnippet, linuxActiveLinksSnippet)
}

func windowsFlushDNSCacheScript() string {
	return `
$ErrorActionPreference = "Stop"
ipconfig /flushdns | Out-Null
if ($LASTEXITCODE -ne 0) { throw "ipconfig /flushdns failed with exit code $LASTEXITCODE" }
Clear-DnsClientCache
`
}

func darwinFlushDNSCacheScript() string {
	return `
set -eu
dscacheutil -flushcache
killall -HUP mDNSResponder
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
  echo "No supported DNS cache service found; nothing to flush."
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
    $existing = Get-DnsClientDohServerAddress -ServerAddress $dns.Server -ErrorAction SilentlyContinue
    if ($existing) {
        Set-DnsClientDohServerAddress -ServerAddress $dns.Server -DohTemplate $dns.Template -AllowFallbackToUdp $true -AutoUpgrade $true
    } else {
        Add-DnsClientDohServerAddress -ServerAddress $dns.Server -DohTemplate $dns.Template -AllowFallbackToUdp $true -AutoUpgrade $true
    }
    Write-Output ("Registered DoH template for {0}" -f $dns.Server)
}
`
}

func windowsDisableDoHScript() string {
	return `
$ErrorActionPreference = "Stop"
$dohServers = Get-DnsClientDohServerAddress -ErrorAction SilentlyContinue
foreach ($server in $dohServers) {
    Remove-DnsClientDohServerAddress -ServerAddress $server.ServerAddress
    Write-Output ("Removed DoH template for {0}" -f $server.ServerAddress)
}
`
}

func windowsSetMTUScript(mtu int) string {
	return fmt.Sprintf(`
$ErrorActionPreference = "Stop"
$adapters = Get-NetAdapter | Where-Object {$_.Status -eq "Up"}
if (-not $adapters) { throw "No active adapters found." }
foreach ($adapter in $adapters) {
    Set-NetIPInterface -InterfaceIndex $adapter.InterfaceIndex -AddressFamily IPv4 -NlMtu %d
    Write-Output ("Set MTU %d on {0}" -f $adapter.Name)
}
`, mtu, mtu)
}

func darwinSetMTUScript(mtu int) string {
	return fmt.Sprintf(`
set -eu
MTU=%d
%s
while IFS= read -r port; do
  [ -z "$port" ] && continue
  networksetup -setMTU "$port" "$MTU"
  echo "Set MTU $MTU on $port"
done <<EOF
$ports
EOF
`, mtu, darwinHardwarePortsSnippet)
}

func linuxSetMTUScript(mtu int) string {
	return fmt.Sprintf(`
set -eu
MTU=%d
%s
for link in $links; do
  ip link set dev "$link" mtu "$MTU"
  echo "Set MTU $MTU on $link"
done
`, mtu, linuxActiveLinksSnippet)
}

func windowsOptimizeNetworkScript() string {
	return `
$ErrorActionPreference = "Stop"
$tcpSettings = @("autotuninglevel=normal", "chimney=enabled", "rss=enabled", "timestamps=disabled", "ecncapability=enabled")
foreach ($setting in $tcpSettings) {
    netsh int tcp set global $setting
    if ($LASTEXITCODE -ne 0) { throw "netsh int tcp set global $setting failed with exit code $LASTEXITCODE" }
}
$adapters = Get-NetAdapter | Where-Object {$_.Status -eq "Up"}
foreach ($adapter in $adapters) {
    Set-NetIPInterface -InterfaceIndex $adapter.InterfaceIndex -AddressFamily IPv4 -NlMtu 1500
    Write-Output ("Set MTU 1500 on {0}" -f $adapter.Name)
}
`
}

func darwinOptimizeNetworkScript() string {
	return darwinSetMTUScript(1500)
}

func linuxOptimizeNetworkScript() string {
	return fmt.Sprintf(`
set -eu
sysctl -w net.ipv4.tcp_moderate_rcvbuf=1
sysctl -w net.ipv4.tcp_timestamps=0
sysctl -w net.ipv4.tcp_ecn=1
%s
for link in $links; do
  ip link set dev "$link" mtu 1500
  echo "Set MTU 1500 on $link"
done
`, linuxActiveLinksSnippet)
}

func windowsResetNetworkOptimizationsScript() string {
	return `
$ErrorActionPreference = "Stop"
netsh int tcp reset
if ($LASTEXITCODE -ne 0) { throw "netsh int tcp reset failed with exit code $LASTEXITCODE" }
netsh winsock reset
if ($LASTEXITCODE -ne 0) { throw "netsh winsock reset failed with exit code $LASTEXITCODE" }
`
}

func darwinResetNetworkOptimizationsScript() string {
	return fmt.Sprintf(`
set -eu
%s
while IFS= read -r port; do
  [ -z "$port" ] && continue
  networksetup -setMTUAndMediaAutomatically "$port" >/dev/null 2>&1 || networksetup -setMTU "$port" 1500
  echo "Reset MTU on $port"
done <<EOF
$ports
EOF
`, darwinHardwarePortsSnippet)
}

func linuxResetNetworkOptimizationsScript() string {
	return fmt.Sprintf(`
set -eu
sysctl -w net.ipv4.tcp_moderate_rcvbuf=1
sysctl -w net.ipv4.tcp_timestamps=1
sysctl -w net.ipv4.tcp_ecn=2
%s
for link in $links; do
  ip link set dev "$link" mtu 1500
  echo "Set MTU 1500 on $link"
done
`, linuxActiveLinksSnippet)
}

func windowsResetDNSScript() string {
	return `
$ErrorActionPreference = "Stop"
$adapters = Get-NetAdapter | Where-Object {$_.Status -eq "Up"}
if (-not $adapters) { throw "No active adapters found." }
foreach ($adapter in $adapters) {
    Set-DnsClientServerAddress -InterfaceIndex $adapter.InterfaceIndex -ResetServerAddresses
    Write-Output ("Reset DNS on {0}" -f $adapter.Name)
}
ipconfig /flushdns | Out-Null
if ($LASTEXITCODE -ne 0) { throw "ipconfig /flushdns failed with exit code $LASTEXITCODE" }
Clear-DnsClientCache
`
}

func darwinResetDNSScript() string {
	return fmt.Sprintf(`
set -eu
%s
while IFS= read -r service; do
  [ -z "$service" ] && continue
  networksetup -setdnsservers "$service" Empty
  echo "Reset DNS on $service"
done <<EOF
$services
EOF
dscacheutil -flushcache
killall -HUP mDNSResponder
`, darwinNetworkServicesSnippet)
}

func linuxResetDNSScript() string {
	return fmt.Sprintf(`
set -eu
if command -v nmcli >/dev/null 2>&1; then
  %s
  changed=0
  while IFS=: read -r uuid type; do
    if [ "$type" = loopback ]; then
      continue
    fi
    nmcli connection modify "$uuid" ipv4.ignore-auto-dns no ipv4.dns ""
    nmcli connection up "$uuid" >/dev/null
    echo "Reset DNS on NetworkManager connection $uuid ($type)"
    changed=$((changed + 1))
  done <<EOF
$connections
EOF
  if [ "$changed" -eq 0 ]; then
    echo "No active NetworkManager connection besides loopback." >&2
    exit 1
  fi
elif command -v resolvectl >/dev/null 2>&1; then
  %s
  for link in $links; do
    resolvectl revert "$link"
    echo "Reverted DNS on $link through systemd-resolved"
  done
elif [ -f /etc/resolv.conf.utils.bak ]; then
  cp /etc/resolv.conf.utils.bak /etc/resolv.conf
  rm -f /etc/resolv.conf.utils.bak
  echo "Restored /etc/resolv.conf from /etc/resolv.conf.utils.bak"
else
  echo "No supported DNS reset mechanism found." >&2
  exit 1
fi
`, linuxActiveConnectionsSnippet, linuxActiveLinksSnippet)
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

const windowsHostsPathLine = `$hostsPath = Join-Path $env:SystemRoot "System32\drivers\etc\hosts"`

func windowsBackupHostsScript(stamp string) string {
	return fmt.Sprintf(`
$ErrorActionPreference = "Stop"
%s
$backupPath = $hostsPath + ".backup-" + %s
Copy-Item -LiteralPath $hostsPath -Destination $backupPath
Write-Output ("Backed up {0} to {1}" -f $hostsPath, $backupPath)
`, windowsHostsPathLine, powerShellQuote(stamp))
}

func posixBackupHostsScript(hostsPath, stamp string) string {
	return fmt.Sprintf(`
set -eu
hosts=%s
backup="$hosts.backup-"%s
cp -p "$hosts" "$backup"
echo "Backed up $hosts to $backup"
`, shellQuote(hostsPath), shellQuote(stamp))
}

func windowsRestoreHostsScript(stamp string) string {
	return fmt.Sprintf(`
$ErrorActionPreference = "Stop"
%s
$latest = Get-ChildItem -LiteralPath (Split-Path $hostsPath) -Filter "hosts.backup-*" -File | Sort-Object Name | Select-Object -Last 1
if (-not $latest) { throw ("No hosts backup matching {0}.backup-* was found." -f $hostsPath) }
$savedPath = $hostsPath + ".before-restore-" + %s
Copy-Item -LiteralPath $hostsPath -Destination $savedPath
Copy-Item -LiteralPath $latest.FullName -Destination $hostsPath -Force
Write-Output ("Saved the current {0} as {1}" -f $hostsPath, $savedPath)
Write-Output ("Restored {0} from {1}" -f $hostsPath, $latest.FullName)
`, windowsHostsPathLine, powerShellQuote(stamp))
}

func posixRestoreHostsScript(hostsPath, stamp string) string {
	return fmt.Sprintf(`
set -eu
hosts=%s
latest=""
for candidate in "$hosts".backup-*; do
  if [ -f "$candidate" ]; then
    latest="$candidate"
  fi
done
if [ -z "$latest" ]; then
  echo "No hosts backup matching $hosts.backup-* was found." >&2
  exit 1
fi
saved="$hosts.before-restore-"%s
cp -p "$hosts" "$saved"
cat "$latest" > "$hosts"
echo "Saved the current $hosts as $saved"
echo "Restored $hosts from $latest"
`, shellQuote(hostsPath), shellQuote(stamp))
}

func windowsAddHostsEntryScript(ip, domain string) string {
	return fmt.Sprintf(`
$ErrorActionPreference = "Stop"
%s
$entry = %s + [char]9 + %s + [char]9 + %s
$lineBreak = [string][char]13 + [char]10
$current = [System.IO.File]::ReadAllText($hostsPath)
$prefix = if ($current.Length -gt 0 -and -not $current.EndsWith([string][char]10)) { $lineBreak } else { "" }
[System.IO.File]::AppendAllText($hostsPath, $prefix + $entry + $lineBreak, [System.Text.Encoding]::ASCII)
Write-Output ("Added to {0}: {1}" -f $hostsPath, $entry)
`, windowsHostsPathLine, powerShellQuote(ip), powerShellQuote(domain), powerShellQuote(HostsManagedMarker))
}

func posixAddHostsEntryScript(hostsPath, ip, domain string) string {
	return fmt.Sprintf(`
set -eu
hosts=%s
if [ -s "$hosts" ] && [ -n "$(tail -c 1 "$hosts")" ]; then
  printf '\n' >> "$hosts"
fi
printf '%%s\t%%s\t%%s\n' %s %s %s >> "$hosts"
echo "Added to $hosts:" %s %s
`, shellQuote(hostsPath), shellQuote(ip), shellQuote(domain), shellQuote(HostsManagedMarker), shellQuote(ip), shellQuote(domain))
}

func windowsRemoveManagedHostsScript() string {
	return fmt.Sprintf(`
$ErrorActionPreference = "Stop"
%s
$kept = New-Object System.Collections.Generic.List[string]
$removed = 0
foreach ($line in [System.IO.File]::ReadAllLines($hostsPath)) {
    if ($line -match '\s#\s*%s\s*$') {
        $removed++
        Write-Output ("Removed: {0}" -f $line)
        continue
    }
    $kept.Add($line)
    if ($line -notmatch '^\s*#' -and $line -match '\S') {
        Write-Output ("Left unmanaged: {0}" -f $line)
    }
}
[System.IO.File]::WriteAllLines($hostsPath, $kept, (New-Object System.Text.UTF8Encoding $false))
Write-Output ("Removed {0} utils-managed entries." -f $removed)
`, windowsHostsPathLine, hostsManagedTag)
}

func posixRemoveManagedHostsScript(hostsPath string) string {
	return fmt.Sprintf(`
set -eu
hosts=%s
kept=$(mktemp)
trap 'rm -f "$kept"' EXIT
awk -v kept="$kept" '
/[[:space:]]#[[:space:]]*%s[[:space:]]*$/ { removed++; print "Removed: " $0; next }
{ print > kept }
/^[[:space:]]*#/ || /^[[:space:]]*$/ { next }
{ print "Left unmanaged: " $0 }
END { printf "Removed %%d utils-managed entries.\n", removed }
' "$hosts"
cat "$kept" > "$hosts"
`, shellQuote(hostsPath), hostsManagedTag)
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
