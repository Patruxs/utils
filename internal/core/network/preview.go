package network

import (
	"fmt"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

type previewStep struct {
	template string
	vars     map[string]string
}

type previewVars map[string]string

const (
	placeholderConnection   = "<connection-uuid>"
	placeholderLink         = "<interface>"
	placeholderService      = "<network-service>"
	placeholderPort         = "<hardware-port>"
	placeholderAdapter      = "<adapter-index>"
	placeholderDoHServer    = "<doh-server>"
	placeholderTime         = "<time>"
	placeholderSavedPrimary = "<saved-primary>"
	placeholderSavedBackup  = "<saved-secondary>"
	placeholderKeptLines    = "<copy-without-utils-managed-lines>"
	placeholderNewest       = "<newest>"
	persistentConfigPath    = "~/.config/utils/network-persistent.conf"
	persistentConfigDir     = "~/.config/utils"
	homeDisplay             = "~"
)

var diagnosticSites = []string{"google.com", "cloudflare.com", "github.com"}

func Preview(action Action, params Params, status Status) []string {
	steps := previewSteps(runtime.GOOS, action, params, status)
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		lines = append(lines, renderPreviewStep(step))
	}
	return lines
}

func previewSteps(goos string, action Action, params Params, status Status) []previewStep {
	if !action.supportedOn(goos) {
		return nil
	}
	params.DNS = normalizeConfigOptions(params.DNS)
	switch goos {
	case osWindows:
		return windowsPreviewSteps(action, params, status)
	case osDarwin:
		return posixPreviewSteps(darwinPreview{status: status}, action, params, status)
	case osLinux:
		return posixPreviewSteps(linuxPreview{status: status}, action, params, status)
	default:
		return nil
	}
}

type posixPlatform interface {
	viewConfig() []previewStep
	diagnostics() []previewStep
	setDNS(opts ConfigOptions) []previewStep
	resetDNS() []previewStep
	flushDNS() []previewStep
	optimize() []previewStep
	resetOptimizations() []previewStep
	setMTU(mtu int) []previewStep
	browserCaches() []browserCacheTarget
}

func posixPreviewSteps(platform posixPlatform, action Action, params Params, status Status) []previewStep {
	hosts := previewVars{"hosts": posixHostsPath}
	viewHosts := step("cat /etc/hosts", nil)
	switch action {
	case ActionViewConfig:
		return platform.viewConfig()
	case ActionDiagnostics:
		return platform.diagnostics()
	case ActionSetDNS:
		return append(platform.setDNS(params.DNS), posixSavePersistentSteps(params.DNS, params.DNS.Persistent)...)
	case ActionResetDNS:
		return platform.resetDNS()
	case ActionFlushDNS:
		return platform.flushDNS()
	case ActionOptimize:
		return platform.optimize()
	case ActionResetOptimizations:
		return platform.resetOptimizations()
	case ActionApplyConfig:
		steps := append(platform.setDNS(params.DNS), posixSavePersistentSteps(params.DNS, params.DNS.Persistent)...)
		return append(steps, platform.setMTU(params.DNS.MTU)...)
	case ActionResetToDefaults:
		return append(platform.resetDNS(), step(posixClearPersistentCommand, previewVars{"HOME": homeDisplay}))
	case ActionHostsView:
		return []previewStep{viewHosts}
	case ActionHostsBackup:
		return []previewStep{viewHosts, step(posixBackupHostsCommand, hosts.with("backup", posixHostsPath+".backup-"+placeholderTime))}
	case ActionHostsAdd:
		ip, domain := hostsEntry(params.Hosts)
		return []previewStep{viewHosts, step(posixAppendHostsCommand(ip, domain), hosts)}
	case ActionHostsRemoveManaged:
		return []previewStep{viewHosts, step(posixRewriteHostsCommand, hosts.with("kept", placeholderKeptLines))}
	case ActionHostsRestore:
		return []previewStep{
			viewHosts,
			step(posixSaveHostsCommand, hosts.with("saved", posixHostsPath+".before-restore-"+placeholderTime)),
			step(posixRestoreHostsCommand, hosts.with("latest", posixHostsPath+".backup-"+placeholderNewest)),
		}
	case ActionBrowserCache:
		var steps []previewStep
		for _, target := range browserCacheTargets(params.Browser, platform.browserCaches()) {
			for _, path := range target.paths {
				steps = append(steps, step(posixClearCachePathCommand, previewVars{"path": path}))
			}
		}
		return steps
	case ActionPersistentStatus:
		return []previewStep{posixReadPersistentStep()}
	case ActionSetPersistentMode:
		if !params.Persistent {
			return []previewStep{step(posixClearPersistentCommand, previewVars{"HOME": homeDisplay})}
		}
		return posixSavePersistentSteps(params.DNS, true)
	case ActionApplyPersistent:
		return append([]previewStep{posixReadPersistentStep()}, platform.setDNS(savedDNS(status))...)
	case ActionClearPersistent:
		return []previewStep{step(posixClearPersistentCommand, previewVars{"HOME": homeDisplay})}
	default:
		return nil
	}
}

func posixReadPersistentStep() previewStep {
	return step(`cat "$path"`, previewVars{"path": persistentConfigPath})
}

func posixSavePersistentSteps(opts ConfigOptions, save bool) []previewStep {
	if !save {
		return nil
	}
	dir := previewVars{"dir": persistentConfigDir}
	return []previewStep{step(posixMakePersistentDirCommand, dir), step(posixWritePersistentCommand(opts), dir)}
}

type linuxPreview struct {
	status Status
}

func (p linuxPreview) dnsBackend() string {
	switch {
	case !p.status.toolsKnown || p.status.HasTool("nmcli"):
		return dnsSourceNetworkManager
	case p.status.HasTool("resolvectl"):
		return dnsSourceResolved
	default:
		return dnsSourceResolvConf
	}
}

func (p linuxPreview) connections() []string {
	var uuids []string
	for _, connection := range p.status.connections {
		if connection.connectionType != "loopback" {
			uuids = append(uuids, connection.uuid)
		}
	}
	return orPlaceholder(uuids, placeholderConnection)
}

func (p linuxPreview) links() []string {
	return orPlaceholder(p.status.links, placeholderLink)
}

func (p linuxPreview) viewConfig() []previewStep {
	dns := step(`grep -E '^[[:space:]]*nameserver[[:space:]]+' /etc/resolv.conf`, nil)
	if !p.status.toolsKnown || p.status.HasTool("resolvectl") {
		dns = step("resolvectl dns", nil)
	}
	return []previewStep{step("ip addr", nil), dns, step("ping -c 4 google.com", nil)}
}

func (p linuxPreview) diagnostics() []previewStep {
	return posixDiagnosticsSteps("-W 2", `getent hosts "$site"`)
}

func (p linuxPreview) setDNS(opts ConfigOptions) []previewStep {
	dns := dnsVars(opts)
	var steps []previewStep
	switch p.dnsBackend() {
	case dnsSourceNetworkManager:
		for _, uuid := range p.connections() {
			vars := dns.with("uuid", uuid)
			steps = append(steps, step(linuxNMSetDNSCommand, vars), step(linuxNMUpCommand, vars))
		}
	case dnsSourceResolved:
		for _, link := range p.links() {
			steps = append(steps, step(linuxResolvedSetDNSCommand, dns.with("link", link)))
		}
	default:
		steps = append(steps, step(linuxResolvConfBackupCommand, nil), step(linuxResolvConfWriteCommand, dns))
	}
	return steps
}

func (p linuxPreview) resetDNS() []previewStep {
	var steps []previewStep
	switch p.dnsBackend() {
	case dnsSourceNetworkManager:
		for _, uuid := range p.connections() {
			vars := previewVars{"uuid": uuid}
			steps = append(steps, step(linuxNMResetDNSCommand, vars), step(linuxNMUpCommand, vars))
		}
	case dnsSourceResolved:
		for _, link := range p.links() {
			steps = append(steps, step(linuxResolvedRevertCommand, previewVars{"link": link}))
		}
	default:
		steps = append(steps, step(linuxResolvConfRestoreCommand, nil), step(linuxResolvConfDropBackupCommand, nil))
	}
	return steps
}

func (p linuxPreview) flushDNS() []previewStep {
	for _, flush := range linuxDNSFlushTools {
		if !p.status.toolsKnown || p.status.HasTool(flush.tool) {
			return []previewStep{step(flush.command, nil)}
		}
	}
	return nil
}

func (p linuxPreview) optimize() []previewStep {
	return p.sysctlAndMTU(linuxOptimizeSysctlCommands)
}

func (p linuxPreview) resetOptimizations() []previewStep {
	return p.sysctlAndMTU(linuxResetSysctlCommands)
}

func (p linuxPreview) sysctlAndMTU(sysctls []string) []previewStep {
	steps := literalSteps(sysctls)
	for _, link := range p.links() {
		steps = append(steps, step(linuxSetMTU1500Command, previewVars{"link": link}))
	}
	return steps
}

func (p linuxPreview) setMTU(mtu int) []previewStep {
	var steps []previewStep
	for _, link := range p.links() {
		steps = append(steps, step(linuxSetMTUCommand, previewVars{"link": link, "MTU": fmt.Sprint(mtu)}))
	}
	return steps
}

func (p linuxPreview) browserCaches() []browserCacheTarget {
	return linuxBrowserCaches
}

type darwinPreview struct {
	status Status
}

func (p darwinPreview) viewConfig() []previewStep {
	steps := []previewStep{step("networksetup -listallhardwareports", nil)}
	for _, service := range orPlaceholder(p.status.services, placeholderService) {
		steps = append(steps, step(`networksetup -getdnsservers "$service"`, previewVars{"service": service}))
	}
	return append(steps, step("ifconfig", nil), step("ping -c 4 google.com", nil))
}

func (p darwinPreview) diagnostics() []previewStep {
	return posixDiagnosticsSteps("", `dig +short "$site"`)
}

func (p darwinPreview) setDNS(opts ConfigOptions) []previewStep {
	var steps []previewStep
	for _, service := range orPlaceholder(p.status.services, placeholderService) {
		steps = append(steps, step(darwinSetDNSServersCommand, dnsVars(opts).with("service", service)))
	}
	return append(steps, step(darwinFlushCacheCommand, nil), step(darwinRestartResponderCommand, nil))
}

func (p darwinPreview) resetDNS() []previewStep {
	var steps []previewStep
	for _, service := range orPlaceholder(p.status.services, placeholderService) {
		steps = append(steps, step(darwinResetDNSServersCommand, previewVars{"service": service}))
	}
	return append(steps, step(darwinFlushCacheCommand, nil), step(darwinRestartResponderCommand, nil))
}

func (p darwinPreview) flushDNS() []previewStep {
	return []previewStep{step(darwinFlushCacheCommand, nil), step(darwinRestartResponderCommand, nil)}
}

func (p darwinPreview) optimize() []previewStep {
	return p.setMTU(1500)
}

func (p darwinPreview) resetOptimizations() []previewStep {
	var steps []previewStep
	for _, port := range orPlaceholder(p.status.ports, placeholderPort) {
		steps = append(steps, step(darwinResetMTUCommand, previewVars{"port": port}))
	}
	return steps
}

func (p darwinPreview) setMTU(mtu int) []previewStep {
	var steps []previewStep
	for _, port := range orPlaceholder(p.status.ports, placeholderPort) {
		steps = append(steps, step(darwinSetMTUCommand, previewVars{"port": port, "MTU": fmt.Sprint(mtu)}))
	}
	return steps
}

func (p darwinPreview) browserCaches() []browserCacheTarget {
	return darwinBrowserCaches
}

func posixDiagnosticsSteps(pingTimeout, resolveTemplate string) []previewStep {
	var steps []previewStep
	for _, site := range diagnosticSites {
		steps = append(steps, step(fmt.Sprintf(`ping -c 1 %s "$site"`, pingTimeout), previewVars{"site": site}))
	}
	for _, site := range diagnosticSites {
		steps = append(steps, step(resolveTemplate, previewVars{"site": site}))
	}
	return append(steps, step("ping -c 4 google.com", nil))
}

func windowsPreviewSteps(action Action, params Params, status Status) []previewStep {
	hosts := previewVars{"hostsPath": HostsPath()}
	viewHosts := step("Get-Content $hostsPath", hosts)
	registry := previewVars{"regPath": windowsPersistentRegistryPath}
	switch action {
	case ActionViewConfig:
		steps := []previewStep{step(`Get-NetAdapter | Where-Object {$_.Status -eq "Up"}`, nil)}
		for _, index := range windowsAdapterIndexes(status) {
			vars := previewVars{"adapter.InterfaceIndex": index}
			steps = append(steps,
				step("Get-DnsClientServerAddress -InterfaceIndex $adapter.InterfaceIndex -AddressFamily IPv4", vars),
				step("Get-NetIPAddress -InterfaceIndex $adapter.InterfaceIndex -AddressFamily IPv4", vars),
				step("Get-NetIPInterface -InterfaceIndex $adapter.InterfaceIndex -AddressFamily IPv4", vars))
		}
		return append(steps, step("Get-DnsClientDohServerAddress", nil), step("Get-Content $hostsPath", previewVars{"hostsPath": HostsPath()}), step("ping -n 4 google.com", nil))
	case ActionDiagnostics:
		var steps []previewStep
		for _, site := range diagnosticSites {
			steps = append(steps, step("Test-NetConnection -ComputerName $site -Port 80 -InformationLevel Quiet", previewVars{"site": site}))
		}
		for _, site := range diagnosticSites {
			steps = append(steps, step("Resolve-DnsName $site -ErrorAction Stop", previewVars{"site": site}))
		}
		return append(steps, step("Test-NetConnection google.com -InformationLevel Detailed", nil))
	case ActionSetDNS:
		return append(windowsSetDNSSteps(params.DNS, status), windowsSavePersistentSteps(params.DNS, params.DNS.Persistent)...)
	case ActionResetDNS:
		return windowsResetDNSSteps(status)
	case ActionFlushDNS:
		return windowsFlushSteps()
	case ActionEnableDoH:
		return windowsEnableDoHSteps(status)
	case ActionDisableDoH:
		return windowsDisableDoHSteps(status)
	case ActionOptimize:
		var steps []previewStep
		for _, setting := range windowsOptimizeTCPSettings {
			steps = append(steps, step(windowsTCPSetGlobalCommand, previewVars{"setting": setting}))
		}
		return append(steps, windowsSetMTUSteps(windowsMTUOptimizeValue, status)...)
	case ActionResetOptimizations:
		return literalSteps([]string{windowsTCPResetCommand, windowsWinsockResetCommand})
	case ActionApplyConfig:
		steps := append(windowsSetDNSSteps(params.DNS, status), windowsSavePersistentSteps(params.DNS, params.DNS.Persistent)...)
		if params.DNS.EnableDoH {
			steps = append(steps, windowsEnableDoHSteps(status)...)
		}
		return append(steps, windowsSetMTUSteps(params.DNS.MTU, status)...)
	case ActionResetToDefaults:
		steps := append(windowsResetDNSSteps(status), windowsDisableDoHSteps(status)...)
		return append(steps, step(windowsClearPersistentCommand, registry))
	case ActionHostsView:
		return []previewStep{viewHosts}
	case ActionHostsBackup:
		return []previewStep{viewHosts, step(windowsBackupHostsCommand, hosts.with("backupPath", HostsPath()+".backup-"+placeholderTime))}
	case ActionHostsAdd:
		ip, domain := hostsEntry(params.Hosts)
		return []previewStep{viewHosts, step(windowsHostsEntryLine(ip, domain), nil), step(windowsAppendHostsCommand, hosts)}
	case ActionHostsRemoveManaged:
		return []previewStep{viewHosts, step(windowsRewriteHostsCommand, hosts.with("kept", placeholderKeptLines))}
	case ActionHostsRestore:
		return []previewStep{
			viewHosts,
			step(windowsSaveHostsCommand, hosts.with("savedPath", HostsPath()+".before-restore-"+placeholderTime)),
			step(windowsRestoreHostsCommand, hosts.with("latest.FullName", HostsPath()+".backup-"+placeholderNewest)),
		}
	case ActionBrowserCache:
		var steps []previewStep
		for _, target := range browserCacheTargets(params.Browser, windowsBrowserCaches) {
			for _, path := range target.paths {
				steps = append(steps, step(windowsClearCachePathCommand, previewVars{"Path": `"` + path + `"`}))
			}
		}
		return steps
	case ActionPersistentStatus:
		return []previewStep{step("Get-ItemProperty -Path $regPath", registry)}
	case ActionSetPersistentMode:
		if !params.Persistent {
			return []previewStep{step(windowsClearPersistentCommand, registry)}
		}
		return windowsSavePersistentSteps(params.DNS, true)
	case ActionApplyPersistent:
		return append([]previewStep{step("Get-ItemProperty -Path $regPath", registry)}, windowsSetDNSSteps(savedDNS(status), status)...)
	case ActionClearPersistent:
		return []previewStep{step(windowsClearPersistentCommand, registry)}
	default:
		return nil
	}
}

func windowsAdapterIndexes(status Status) []string {
	indexes := make([]string, 0, len(status.adapters))
	for _, adapter := range status.adapters {
		indexes = append(indexes, adapter.index)
	}
	return orPlaceholder(indexes, placeholderAdapter)
}

func windowsSetDNSSteps(opts ConfigOptions, status Status) []previewStep {
	var steps []previewStep
	for _, index := range windowsAdapterIndexes(status) {
		steps = append(steps, step(windowsSetDNSServersCommand(opts), previewVars{"adapter.InterfaceIndex": index}))
	}
	return append(steps, windowsFlushSteps()...)
}

func windowsResetDNSSteps(status Status) []previewStep {
	var steps []previewStep
	for _, index := range windowsAdapterIndexes(status) {
		steps = append(steps, step(windowsResetDNSServersCommand, previewVars{"adapter.InterfaceIndex": index}))
	}
	return append(steps, windowsFlushSteps()...)
}

func windowsSetMTUSteps(mtu int, status Status) []previewStep {
	var steps []previewStep
	for _, index := range windowsAdapterIndexes(status) {
		steps = append(steps, step(windowsSetMTUCommand(mtu), previewVars{"adapter.InterfaceIndex": index}))
	}
	return steps
}

func windowsFlushSteps() []previewStep {
	return literalSteps([]string{windowsFlushDNSCommand, windowsClearDNSCacheCommand})
}

func windowsEnableDoHSteps(status Status) []previewStep {
	var steps []previewStep
	for _, doh := range windowsDoHServers {
		template := windowsAddDoHCommand
		if containsString(status.dohServers, doh.server) {
			template = windowsSetDoHCommand
		}
		steps = append(steps, step(template, previewVars{"dns.Server": doh.server, "dns.Template": doh.template}))
	}
	return steps
}

func windowsDisableDoHSteps(status Status) []previewStep {
	servers := status.dohServers
	if status.DoH == StateUnknown {
		servers = []string{placeholderDoHServer}
	}
	var steps []previewStep
	for _, server := range servers {
		steps = append(steps, step(windowsRemoveDoHCommand, previewVars{"server.ServerAddress": server}))
	}
	return steps
}

func windowsSavePersistentSteps(opts ConfigOptions, save bool) []previewStep {
	if !save {
		return nil
	}
	var steps []previewStep
	for _, command := range windowsPersistentValueCommands(opts) {
		steps = append(steps, step(command, previewVars{"regPath": windowsPersistentRegistryPath}))
	}
	return steps
}

func savedDNS(status Status) ConfigOptions {
	if status.Persistent == StateOn && status.Saved.DNSPrimary != "" && status.Saved.DNSSecondary != "" {
		return ConfigOptions{DNSName: status.Saved.DNSName, DNSPrimary: status.Saved.DNSPrimary, DNSSecondary: status.Saved.DNSSecondary}
	}
	return ConfigOptions{DNSPrimary: placeholderSavedPrimary, DNSSecondary: placeholderSavedBackup}
}

func hostsEntry(hosts HostsOptions) (string, string) {
	ip := strings.TrimSpace(hosts.IP)
	if ip == "" {
		ip = "127.0.0.1"
	}
	return ip, strings.TrimSpace(hosts.Domain)
}

func dnsVars(opts ConfigOptions) previewVars {
	return previewVars{"DNS_PRIMARY": opts.DNSPrimary, "DNS_SECONDARY": opts.DNSSecondary}
}

func (v previewVars) with(name, value string) previewVars {
	next := make(previewVars, len(v)+1)
	for key, existing := range v {
		next[key] = existing
	}
	next[name] = value
	return next
}

func step(template string, vars previewVars) previewStep {
	return previewStep{template: template, vars: vars}
}

func literalSteps(commands []string) []previewStep {
	steps := make([]previewStep, 0, len(commands))
	for _, command := range commands {
		steps = append(steps, step(command, nil))
	}
	return steps
}

func orPlaceholder(values []string, placeholder string) []string {
	if len(values) == 0 {
		return []string{placeholder}
	}
	return values
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

var (
	quotedHomePath    = regexp.MustCompile(`"~/([^"]*)"`)
	plainValue        = regexp.MustCompile(`^[A-Za-z0-9_./~*@%+,:=<>\-]+$`)
	repeatedSpaces    = regexp.MustCompile(` {2,}`)
	homeVariableForms = []string{`"$HOME"`, "${HOME}", "$HOME"}
)

func renderPreviewStep(s previewStep) string {
	names := make([]string, 0, len(s.vars))
	for name := range s.vars {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })

	line := s.template
	for _, name := range names {
		value := displayHome(s.vars[name])
		if plainValue.MatchString(value) {
			line = strings.ReplaceAll(line, `"$`+name+`"`, value)
		}
		line = strings.ReplaceAll(line, "${"+name+"}", value)
		line = replaceVariable(line, "$"+name, value)
	}
	line = displayHome(line)
	line = quotedHomePath.ReplaceAllStringFunc(line, func(match string) string {
		return "~/" + strings.ReplaceAll(match[3:len(match)-1], " ", `\ `)
	})
	return repeatedSpaces.ReplaceAllString(line, " ")
}
func displayHome(value string) string {
	for _, form := range homeVariableForms {
		value = strings.ReplaceAll(value, form, homeDisplay)
	}
	return value
}

func replaceVariable(line, variable, value string) string {
	var b strings.Builder
	for {
		index := strings.Index(line, variable)
		if index < 0 {
			b.WriteString(line)
			return b.String()
		}
		end := index + len(variable)
		b.WriteString(line[:index])
		if end < len(line) && isIdentifierByte(line[end]) {
			b.WriteString(variable)
		} else {
			b.WriteString(value)
		}
		line = line[end:]
	}
}

func isIdentifierByte(c byte) bool {
	return c == '_' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9')
}
