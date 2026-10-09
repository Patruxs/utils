package views

import (
	"fmt"
	"runtime"

	corenetwork "utils/internal/core/network"
)

type networkActionID int

const (
	networkActionViewConfig networkActionID = iota
	networkActionDiagnostics
	networkActionSetCloudflareDNS
	networkActionSetGoogleDNS
	networkActionSetOpenDNS
	networkActionSetQuad9DNS
	networkActionResetDNS
	networkActionFlushDNS
	networkActionEnableDoH
	networkActionDisableDoH
	networkActionOptimize
	networkActionResetOptimizations
	networkActionApplyConfig
	networkActionResetDefaults
	networkActionHostsView
	networkActionHostsBackup
	networkActionHostsAdd
	networkActionHostsRemoveCustom
	networkActionHostsRestore
	networkActionBrowserChrome
	networkActionBrowserFirefox
	networkActionBrowserEdge
	networkActionBrowserBrave
	networkActionBrowserOpera
	networkActionBrowserAll
	networkActionPersistentStatus
	networkActionTogglePersistent
	networkActionApplyPersistent
	networkActionClearPersistent
)

const (
	networkNoUndo    networkActionID = -1
	networkOSWindows                 = "windows"
)

const (
	networkGroupInspect    = "INSPECT"
	networkGroupDNS        = "DNS"
	networkGroupDoH        = "DOH"
	networkGroupTuning     = "TUNING"
	networkGroupHosts      = "HOSTS"
	networkGroupBrowser    = "BROWSER CACHES"
	networkGroupPersistent = "PERSISTENT DNS"
)

type networkActionItem struct {
	id      networkActionID
	group   string
	name    string
	column  string
	title   string
	action  corenetwork.Action
	summary string
	undo    networkActionID
}

var networkInlineGroups = []string{networkGroupBrowser, networkGroupPersistent}

var networkActions = []networkActionItem{
	{networkActionViewConfig, networkGroupInspect, "View config", "", "View config", corenetwork.ActionViewConfig, "Reads adapters, addresses and DNS servers, then pings google.com. Changes nothing.", networkNoUndo},
	{networkActionDiagnostics, networkGroupInspect, "Diagnostics", "", "Diagnostics", corenetwork.ActionDiagnostics, "Checks connectivity, DNS resolution and ping for google.com, cloudflare.com and github.com.", networkNoUndo},
	{networkActionSetCloudflareDNS, networkGroupDNS, "Cloudflare", "1.1.1.1", "Set Cloudflare DNS", corenetwork.ActionSetDNS, "", networkActionResetDNS},
	{networkActionSetGoogleDNS, networkGroupDNS, "Google", "8.8.8.8", "Set Google DNS", corenetwork.ActionSetDNS, "", networkActionResetDNS},
	{networkActionSetOpenDNS, networkGroupDNS, "OpenDNS", "208.67.222.222", "Set OpenDNS", corenetwork.ActionSetDNS, "", networkActionResetDNS},
	{networkActionSetQuad9DNS, networkGroupDNS, "Quad9", "9.9.9.9", "Set Quad9 DNS", corenetwork.ActionSetDNS, "", networkActionResetDNS},
	{networkActionResetDNS, networkGroupDNS, "Reset to automatic", "", "Reset DNS to automatic", corenetwork.ActionResetDNS, "Clears manually set DNS servers on every active connection and returns to automatic DNS.", networkNoUndo},
	{networkActionFlushDNS, networkGroupDNS, "Flush cache", "", "Flush DNS cache", corenetwork.ActionFlushDNS, "Flushes the operating system DNS cache so new lookups go to the DNS servers.", networkNoUndo},
	{networkActionEnableDoH, networkGroupDoH, "Enable", "", "Enable DNS over HTTPS", corenetwork.ActionEnableDoH, "Registers DNS over HTTPS templates for Cloudflare, Google and Quad9.", networkActionDisableDoH},
	{networkActionDisableDoH, networkGroupDoH, "Disable", "", "Disable DNS over HTTPS", corenetwork.ActionDisableDoH, "Removes every registered DoH template, including the Windows built-in ones.", networkActionEnableDoH},
	{networkActionOptimize, networkGroupTuning, "Optimize", "", "Optimize network", corenetwork.ActionOptimize, "", networkActionResetOptimizations},
	{networkActionResetOptimizations, networkGroupTuning, "Reset optimizations", "", "Reset optimizations", corenetwork.ActionResetOptimizations, "", networkNoUndo},
	{networkActionApplyConfig, networkGroupTuning, "Apply full config", "", "Apply full config", corenetwork.ActionApplyConfig, "", networkActionResetDefaults},
	{networkActionResetDefaults, networkGroupTuning, "Reset to defaults", "", "Reset network defaults", corenetwork.ActionResetToDefaults, "Resets DNS to automatic, removes DoH templates where supported and deletes the saved persistent DNS.", networkNoUndo},
	{networkActionHostsView, networkGroupHosts, "View", "", "View hosts file", corenetwork.ActionHostsView, "Shows the hosts file. Changes nothing.", networkNoUndo},
	{networkActionHostsBackup, networkGroupHosts, "Backup", "", "Back up hosts file", corenetwork.ActionHostsBackup, "Copies the hosts file to a timestamped backup next to it.", networkNoUndo},
	{networkActionHostsAdd, networkGroupHosts, "Add entry…", "", "Add hosts entry", corenetwork.ActionHostsAdd, "Appends one line tagged " + corenetwork.HostsManagedMarker + " to the hosts file.", networkActionHostsRemoveCustom},
	{networkActionHostsRemoveCustom, networkGroupHosts, "Remove managed", "", "Remove managed hosts entries", corenetwork.ActionHostsRemoveManaged, "Removes only the lines tagged " + corenetwork.HostsManagedMarker + ". Every other line stays.", networkActionHostsRestore},
	{networkActionHostsRestore, networkGroupHosts, "Restore newest backup", "", "Restore hosts backup", corenetwork.ActionHostsRestore, "Saves the current hosts file aside, then restores the newest backup over it.", networkNoUndo},
	{networkActionBrowserChrome, networkGroupBrowser, "Chrome", "", "Clear Chrome cache", corenetwork.ActionBrowserCache, browserCacheSummary("Chrome and Chromium"), networkNoUndo},
	{networkActionBrowserFirefox, networkGroupBrowser, "Firefox", "", "Clear Firefox cache", corenetwork.ActionBrowserCache, browserCacheSummary("Firefox"), networkNoUndo},
	{networkActionBrowserEdge, networkGroupBrowser, "Edge", "", "Clear Edge cache", corenetwork.ActionBrowserCache, browserCacheSummary("Edge"), networkNoUndo},
	{networkActionBrowserBrave, networkGroupBrowser, "Brave", "", "Clear Brave cache", corenetwork.ActionBrowserCache, browserCacheSummary("Brave"), networkNoUndo},
	{networkActionBrowserOpera, networkGroupBrowser, "Opera", "", "Clear Opera cache", corenetwork.ActionBrowserCache, browserCacheSummary("Opera"), networkNoUndo},
	{networkActionBrowserAll, networkGroupBrowser, "All", "", "Clear all browser caches", corenetwork.ActionBrowserCache, browserCacheSummary("Chrome, Chromium, Firefox, Edge, Brave and Opera"), networkNoUndo},
	{networkActionPersistentStatus, networkGroupPersistent, "Status", "", "Persistent DNS status", corenetwork.ActionPersistentStatus, "Shows whether a DNS preset is saved to reapply later. Changes nothing.", networkNoUndo},
	{networkActionTogglePersistent, networkGroupPersistent, "Toggle", "", "Toggle persistent DNS", corenetwork.ActionSetPersistentMode, "", networkActionTogglePersistent},
	{networkActionApplyPersistent, networkGroupPersistent, "Apply saved", "", "Apply saved DNS", corenetwork.ActionApplyPersistent, "Applies the saved persistent DNS to every active connection.", networkActionResetDNS},
	{networkActionClearPersistent, networkGroupPersistent, "Clear", "", "Clear saved DNS", corenetwork.ActionClearPersistent, "Deletes the saved persistent DNS settings.", networkNoUndo},
}

func browserCacheSummary(browsers string) string {
	return "Empties the " + browsers + " cache folders of the current user. Close the browser first."
}

func networkItem(id networkActionID) networkActionItem {
	for _, item := range networkActions {
		if item.id == id {
			return item
		}
	}
	return networkActions[0]
}

func actionTitle(id networkActionID) string {
	return networkItem(id).title
}

func (item networkActionItem) writes() bool {
	return item.action.Writes()
}

func (item networkActionItem) supported() bool {
	return item.action.Supported()
}

func (m NetworkModel) actionSummary(id networkActionID) string {
	item := networkItem(id)
	switch id {
	case networkActionSetCloudflareDNS, networkActionSetGoogleDNS, networkActionSetOpenDNS, networkActionSetQuad9DNS:
		preset := m.params(id).DNS
		summary := fmt.Sprintf("Sets %s and %s on every active connection except loopback.", preset.DNSPrimary, preset.DNSSecondary)
		if preset.Persistent {
			summary += " Also saves them as the persistent DNS."
		}
		return summary
	case networkActionOptimize:
		if runtime.GOOS == networkOSWindows {
			return "Tunes TCP auto-tuning, RSS, timestamps and ECN, and sets MTU 1500 on every active adapter."
		}
		return "Tunes TCP buffer, timestamp and ECN settings and sets MTU 1500 on every active interface."
	case networkActionResetOptimizations:
		if runtime.GOOS == networkOSWindows {
			return "Resets the TCP and Winsock settings. Restart afterwards."
		}
		return "Restores the TCP buffer, timestamp and ECN defaults and sets MTU 1500 on every active interface."
	case networkActionApplyConfig:
		preset := m.params(id).DNS
		return fmt.Sprintf("Sets %s DNS (%s, %s), registers DoH templates where supported and sets MTU %d.", preset.DNSName, preset.DNSPrimary, preset.DNSSecondary, preset.MTU)
	case networkActionTogglePersistent:
		if m.persistentOn() {
			return "Turns persistent DNS off and deletes the saved DNS."
		}
		preset := corenetwork.DefaultConfigOptions()
		return fmt.Sprintf("Turns persistent DNS on and saves %s (%s, %s). DNS presets then update the saved value.", preset.DNSName, preset.DNSPrimary, preset.DNSSecondary)
	}
	return item.summary
}

func (m NetworkModel) params(id networkActionID) corenetwork.Params {
	params := corenetwork.Params{Hosts: m.hostsEntry()}
	withPersistence := func(opts corenetwork.ConfigOptions) corenetwork.ConfigOptions {
		opts.Persistent = m.persistentOn()
		return opts
	}
	switch id {
	case networkActionSetCloudflareDNS, networkActionApplyConfig:
		params.DNS = withPersistence(corenetwork.CloudflareDNSOptions())
	case networkActionSetGoogleDNS:
		params.DNS = withPersistence(corenetwork.GoogleDNSOptions())
	case networkActionSetOpenDNS:
		params.DNS = withPersistence(corenetwork.OpenDNSOptions())
	case networkActionSetQuad9DNS:
		params.DNS = withPersistence(corenetwork.Quad9DNSOptions())
	case networkActionTogglePersistent:
		params.DNS = corenetwork.DefaultConfigOptions()
		params.Persistent = !m.persistentOn()
	case networkActionBrowserChrome:
		params.Browser = corenetwork.BrowserChrome
	case networkActionBrowserFirefox:
		params.Browser = corenetwork.BrowserFirefox
	case networkActionBrowserEdge:
		params.Browser = corenetwork.BrowserEdge
	case networkActionBrowserBrave:
		params.Browser = corenetwork.BrowserBrave
	case networkActionBrowserOpera:
		params.Browser = corenetwork.BrowserOpera
	case networkActionBrowserAll:
		params.Browser = corenetwork.BrowserAll
	}
	return params
}

func (m NetworkModel) persistentOn() bool {
	if m.status.Persistent == corenetwork.StateUnknown {
		return m.persistentMode
	}
	return m.status.Persistent == corenetwork.StateOn
}

func isInlineGroup(group string) bool {
	for _, inline := range networkInlineGroups {
		if inline == group {
			return true
		}
	}
	return false
}
