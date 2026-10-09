package network

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/bits"
	"net/netip"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type State int

const (
	StateUnknown State = iota
	StateOff
	StateOn
	StateUnsupported
)

type Status struct {
	Adapter    string
	Address    string
	Gateway    string
	DNS        []string
	DNSSource  string
	MTU        int
	DoH        State
	Persistent State
	Saved      PersistentSettings
	Sudo       State

	connections []activeConnection
	links       []string
	tools       []string
	toolsKnown  bool
	services    []string
	ports       []string
	adapters    []windowsAdapter
	dohServers  []string
}

type activeConnection struct {
	uuid           string
	connectionType string
}

type windowsAdapter struct {
	index string
	name  string
}

const (
	dnsSourceNetworkManager = "NetworkManager"
	dnsSourceResolved       = "systemd-resolved"
	dnsSourceResolvConf     = "resolv.conf"
	dnsSourceWindows        = "DnsClient"
	dnsSourceDarwin         = "networksetup"
)

func ReadStatus(ctx context.Context) (Status, error) {
	return NewNetworkManager(nil).ReadStatus(ctx)
}

func (m NetworkManager) ReadStatus(ctx context.Context) (Status, error) {
	m = m.withDefaults()
	output, _ := m.outputPlatformScript(ctx, windowsStatusScript(), darwinStatusScript(), linuxStatusScript())
	if err := ctx.Err(); err != nil {
		return Status{}, err
	}
	status := parseStatus(runtime.GOOS, output)
	status.Sudo = m.sudoState(ctx)
	return status, ctx.Err()
}

func (m NetworkManager) sudoState(ctx context.Context) State {
	if runtime.GOOS == osWindows {
		return StateUnsupported
	}
	_, err := m.commands.Output(ctx, commandSudo, "-n", "true")
	switch {
	case ctx.Err() != nil:
		return StateUnknown
	case err == nil:
		return StateOn
	case errors.Is(err, exec.ErrNotFound):
		return StateUnknown
	default:
		return StateOff
	}
}

func (s Status) HasTool(name string) bool {
	for _, tool := range s.tools {
		if tool == name {
			return true
		}
	}
	return false
}

func parseStatus(goos, output string) Status {
	status := Status{}
	var routes, addresses, linkLines, resolved, nameservers, persistent []string
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch key {
		case "route":
			routes = append(routes, value)
		case "addr":
			addresses = append(addresses, value)
		case "link":
			linkLines = append(linkLines, value)
		case "tools":
			status.tools = strings.Fields(value)
			status.toolsKnown = true
		case "connection":
			if uuid, connectionType, ok := strings.Cut(value, ":"); ok && uuid != "" {
				status.connections = append(status.connections, activeConnection{uuid: uuid, connectionType: connectionType})
			}
		case "resolved":
			resolved = append(resolved, value)
		case "resolvconf":
			if fields := strings.Fields(value); len(fields) >= 2 {
				nameservers = append(nameservers, fields[1])
			}
		case "persistent":
			persistent = append(persistent, value)
		case "adapter":
			status.Adapter = value
		case "gateway":
			status.Gateway = value
		case "address":
			status.Address = value
		case "inet":
			status.Address = darwinAddress(value)
		case "mtu":
			status.MTU, _ = strconv.Atoi(value)
		case "dns":
			status.DNS = appendUnique(status.DNS, strings.Fields(value)...)
		case "doh":
			status.DoH = StateOff
			if strings.EqualFold(value, "true") {
				status.DoH = StateOn
			}
		case "dohserver":
			status.dohServers = append(status.dohServers, value)
		case "adapterindex":
			if index, name, ok := strings.Cut(value, " "); ok {
				status.adapters = append(status.adapters, windowsAdapter{index: index, name: name})
			}
		case "service":
			status.services = append(status.services, value)
		case "port":
			status.ports = append(status.ports, value)
		}
	}

	if len(persistent) > 0 {
		status.Saved = parsePersistentSettings(strings.Join(persistent, "\n"))
		status.Persistent = StateOff
		if status.Saved.Enabled {
			status.Persistent = StateOn
		}
	}

	switch goos {
	case osLinux:
		parseLinuxStatus(&status, routes, addresses, linkLines, resolved, nameservers)
		status.DoH = StateUnsupported
	case osDarwin:
		status.DoH = StateUnsupported
		if len(status.services) > 0 || status.Adapter != "" {
			status.DNSSource = dnsSourceDarwin
		}
	case osWindows:
		if status.Adapter != "" {
			status.DNSSource = dnsSourceWindows
		}
	}
	return status
}

func parseLinuxStatus(status *Status, routes, addresses, linkLines, resolved, nameservers []string) {
	for _, route := range routes {
		fields := strings.Fields(route)
		status.Gateway = fieldAfter(fields, "via")
		status.Adapter = fieldAfter(fields, "dev")
		if status.Adapter != "" {
			break
		}
	}

	for _, address := range addresses {
		fields := strings.Fields(address)
		if len(fields) < 4 || fields[2] != "inet" || fields[1] == "lo" {
			continue
		}
		if status.Adapter == "" {
			status.Adapter = fields[1]
		}
		if fields[1] == status.Adapter {
			status.Address = fields[3]
			break
		}
	}

	for _, line := range linkLines {
		parts := strings.SplitN(line, ": ", 3)
		if len(parts) < 3 {
			continue
		}
		name, _, _ := strings.Cut(parts[1], "@")
		if name == "lo" {
			continue
		}
		status.links = append(status.links, name)
		if name == status.Adapter {
			status.MTU, _ = strconv.Atoi(fieldAfter(strings.Fields(parts[2]), "mtu"))
		}
	}

	var global []string
	label := ""
	for _, line := range resolved {
		servers := line
		if strings.HasPrefix(line, "Link ") || strings.HasPrefix(line, "Global") {
			label, servers, _ = strings.Cut(line, ":")
		}
		switch {
		case status.Adapter != "" && strings.Contains(label, "("+status.Adapter+")"):
			status.DNS = appendUnique(status.DNS, strings.Fields(servers)...)
		case label == "Global":
			global = appendUnique(global, strings.Fields(servers)...)
		}
	}
	if len(status.DNS) == 0 {
		status.DNS = global
	}
	if len(status.DNS) == 0 {
		status.DNS = nameservers
	}

	if status.toolsKnown {
		switch {
		case status.HasTool("nmcli"):
			status.DNSSource = dnsSourceNetworkManager
		case status.HasTool("resolvectl"):
			status.DNSSource = dnsSourceResolved
		default:
			status.DNSSource = dnsSourceResolvConf
		}
	}
}

func fieldAfter(fields []string, name string) string {
	for index := 0; index+1 < len(fields); index++ {
		if fields[index] == name {
			return fields[index+1]
		}
	}
	return ""
}

func darwinAddress(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	if _, err := netip.ParseAddr(fields[0]); err != nil {
		return ""
	}
	if len(fields) < 2 {
		return fields[0]
	}
	mask, err := hex.DecodeString(strings.TrimPrefix(fields[1], "0x"))
	if err != nil || len(mask) != 4 {
		return fields[0]
	}
	prefix := 0
	for _, part := range mask {
		prefix += bits.OnesCount8(part)
	}
	return fmt.Sprintf("%s/%d", fields[0], prefix)
}

func appendUnique(values []string, more ...string) []string {
	for _, value := range more {
		seen := false
		for _, existing := range values {
			if existing == value {
				seen = true
				break
			}
		}
		if !seen {
			values = append(values, value)
		}
	}
	return values
}

func linuxStatusScript() string {
	return fmt.Sprintf(`
ip -o -4 route show default 2>/dev/null | sed 's/^/route=/'
ip -o -4 addr show up 2>/dev/null | sed 's/^/addr=/'
ip -o link show up 2>/dev/null | sed 's/^/link=/'
tools=""
for tool in nmcli resolvectl %s; do
  if command -v "$tool" >/dev/null 2>&1; then
    tools="$tools $tool"
  fi
done
echo "tools=$tools"
if command -v nmcli >/dev/null 2>&1; then
  nmcli -t -f UUID,TYPE connection show --active 2>/dev/null | sed 's/^/connection=/'
fi
if command -v resolvectl >/dev/null 2>&1; then
  resolvectl dns 2>/dev/null | sed 's/^/resolved=/'
fi
if [ -f /etc/resolv.conf ]; then
  grep -E '^[[:space:]]*nameserver[[:space:]]+' /etc/resolv.conf 2>/dev/null | sed 's/^/resolvconf=/'
fi
(%s) 2>/dev/null | sed 's/^/persistent=/'
`, strings.Join(linuxFlushToolNames(), " "), posixPersistentStatusScript())
}

func linuxFlushToolNames() []string {
	names := make([]string, 0, len(linuxDNSFlushTools))
	for _, flush := range linuxDNSFlushTools {
		if flush.tool != "resolvectl" {
			names = append(names, flush.tool)
		}
	}
	return names
}

func darwinStatusScript() string {
	return fmt.Sprintf(`
route -n get default 2>/dev/null | awk '/gateway:/ {print "gateway=" $2} /interface:/ {print "adapter=" $2}'
iface=$(route -n get default 2>/dev/null | awk '/interface:/ {print $2; exit}')
if [ -n "$iface" ]; then
  ifconfig "$iface" 2>/dev/null | awk '{for (i = 1; i < NF; i++) if ($i == "mtu") print "mtu=" $(i+1)} $1 == "inet" {print "inet=" $2 " " $4}'
fi
scutil --dns 2>/dev/null | awk '/nameserver\[[0-9]+\]/ {print "dns=" $3}'
networksetup -listallnetworkservices 2>/dev/null | tail -n +2 | sed 's/^\*//' | sed 's/^/service=/'
networksetup -listallhardwareports 2>/dev/null | awk -F': ' '/Hardware Port:/ {print "port=" $2}'
(%s) 2>/dev/null | sed 's/^/persistent=/'
`, posixPersistentStatusScript())
}

func windowsStatusScript() string {
	return fmt.Sprintf(`
$route = Get-NetRoute -DestinationPrefix "0.0.0.0/0" -ErrorAction SilentlyContinue | Sort-Object RouteMetric | Select-Object -First 1
if ($route) {
    $index = $route.InterfaceIndex
    Write-Output ("gateway={0}" -f $route.NextHop)
    $adapter = Get-NetAdapter -InterfaceIndex $index -ErrorAction SilentlyContinue
    if ($adapter) { Write-Output ("adapter={0}" -f $adapter.Name) }
    $ip = Get-NetIPAddress -InterfaceIndex $index -AddressFamily IPv4 -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($ip) { Write-Output ("address={0}/{1}" -f $ip.IPAddress, $ip.PrefixLength) }
    $iface = Get-NetIPInterface -InterfaceIndex $index -AddressFamily IPv4 -ErrorAction SilentlyContinue
    if ($iface) { Write-Output ("mtu={0}" -f $iface.NlMtu) }
    $dns = Get-DnsClientServerAddress -InterfaceIndex $index -AddressFamily IPv4 -ErrorAction SilentlyContinue
    if ($dns) { Write-Output ("dns={0}" -f ($dns.ServerAddresses -join " ")) }
}
Get-NetAdapter -ErrorAction SilentlyContinue | Where-Object {$_.Status -eq "Up"} | ForEach-Object { Write-Output ("adapterindex={0} {1}" -f $_.InterfaceIndex, $_.Name) }
$doh = @(Get-DnsClientDohServerAddress -ErrorAction SilentlyContinue)
Write-Output ("doh={0}" -f ($doh.Count -gt 0))
foreach ($server in $doh) { Write-Output ("dohserver={0}" -f $server.ServerAddress) }
& {
%s
} | ForEach-Object { "persistent=" + $_ }
`, windowsPersistentStatusScript())
}
