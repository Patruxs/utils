package network

import (
	"context"
	"runtime"
)

type Action int

const (
	ActionViewConfig Action = iota
	ActionDiagnostics
	ActionSetDNS
	ActionResetDNS
	ActionFlushDNS
	ActionEnableDoH
	ActionDisableDoH
	ActionOptimize
	ActionResetOptimizations
	ActionApplyConfig
	ActionResetToDefaults
	ActionHostsView
	ActionHostsBackup
	ActionHostsAdd
	ActionHostsRemoveManaged
	ActionHostsRestore
	ActionBrowserCache
	ActionPersistentStatus
	ActionSetPersistentMode
	ActionApplyPersistent
	ActionClearPersistent
)

type Params struct {
	DNS        ConfigOptions
	Hosts      HostsOptions
	Browser    BrowserMode
	Persistent bool
}

func (m NetworkManager) Run(ctx context.Context, action Action, params Params) (Report, error) {
	switch action {
	case ActionDiagnostics:
		return m.Diagnostics(ctx)
	case ActionSetDNS:
		return m.SetDNS(ctx, params.DNS)
	case ActionResetDNS:
		return m.ResetDNS(ctx)
	case ActionFlushDNS:
		return m.FlushDNSCache(ctx)
	case ActionEnableDoH:
		return m.EnableDoH(ctx)
	case ActionDisableDoH:
		return m.DisableDoH(ctx)
	case ActionOptimize:
		return m.OptimizeNetworkSettings(ctx)
	case ActionResetOptimizations:
		return m.ResetNetworkOptimizations(ctx)
	case ActionApplyConfig:
		return m.ApplyConfig(ctx, params.DNS)
	case ActionResetToDefaults:
		return m.ResetToDefaults(ctx)
	case ActionHostsView:
		return m.EditHosts(ctx, HostsOptions{Mode: HostsView})
	case ActionHostsBackup:
		return m.EditHosts(ctx, HostsOptions{Mode: HostsBackup})
	case ActionHostsAdd:
		return m.EditHosts(ctx, HostsOptions{Mode: HostsAdd, IP: params.Hosts.IP, Domain: params.Hosts.Domain})
	case ActionHostsRemoveManaged:
		return m.EditHosts(ctx, HostsOptions{Mode: HostsRemoveCustom})
	case ActionHostsRestore:
		return m.EditHosts(ctx, HostsOptions{Mode: HostsRestore})
	case ActionBrowserCache:
		return m.ClearBrowserCache(ctx, params.Browser)
	case ActionPersistentStatus:
		return m.PersistentStatus(ctx)
	case ActionSetPersistentMode:
		return m.SetPersistentMode(ctx, params.Persistent, params.DNS)
	case ActionApplyPersistent:
		return m.ApplyPersistentSettings(ctx)
	case ActionClearPersistent:
		return m.ClearPersistentSettings(ctx)
	default:
		return m.CurrentConfig(ctx)
	}
}

func (a Action) Writes() bool {
	switch a {
	case ActionViewConfig, ActionDiagnostics, ActionHostsView, ActionPersistentStatus:
		return false
	default:
		return true
	}
}

func (a Action) Elevated() bool {
	if !a.Writes() {
		return false
	}
	switch a {
	case ActionBrowserCache, ActionSetPersistentMode, ActionClearPersistent:
		return false
	default:
		return true
	}
}

func (a Action) Supported() bool {
	return a.supportedOn(runtime.GOOS)
}

func (a Action) supportedOn(goos string) bool {
	switch a {
	case ActionEnableDoH, ActionDisableDoH:
		return goos == osWindows
	default:
		return true
	}
}
