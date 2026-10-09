package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	corenetwork "utils/internal/core/network"
)

func TestNetworkBatchRunsHostsBackupBeforeRemove(t *testing.T) {
	model := NewNetworkModel()
	model.checkedActions[networkActionHostsRemoveCustom] = true
	model.checkedActions[networkActionHostsBackup] = true

	actions := model.selectedActionIDs()

	backupIndex, removeIndex := -1, -1
	for index, action := range actions {
		switch action {
		case networkActionHostsBackup:
			backupIndex = index
		case networkActionHostsRemoveCustom:
			removeIndex = index
		}
	}
	if backupIndex < 0 || removeIndex < 0 || backupIndex > removeIndex {
		t.Fatalf("expected hosts backup to run before remove, got %v", actions)
	}
}

func TestNetworkWriteActionAsksForConfirmationBeforeRunning(t *testing.T) {
	model := networkModelAt(networkActionSetCloudflareDNS)

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(NetworkModel)
	if model.state != networkStateConfirmingWrite || cmd != nil {
		t.Fatalf("expected enter on a write action to ask for confirmation without running, state=%v cmd=%v", model.state, cmd != nil)
	}

	next, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = next.(NetworkModel)
	if model.state != networkStateSelectingOptions || cmd != nil {
		t.Fatalf("expected n to cancel without running, state=%v cmd=%v", model.state, cmd != nil)
	}
}

func TestNetworkEnterAfterRunDoesNotRepeatAction(t *testing.T) {
	model := networkModelAt(networkActionDiagnostics)
	next, _ := model.Update(networkFinishedMsg{results: []networkActionResult{{action: networkActionDiagnostics, report: corenetwork.Report{Operation: "Run Network Diagnostics"}}}})
	model = next.(NetworkModel)

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(NetworkModel)

	if model.state == networkStateRunning || cmd != nil {
		t.Fatalf("expected enter after a finished run not to start another run, state=%v", model.state)
	}
}

func networkModelAt(target networkActionID) NetworkModel {
	model := NewNetworkModel()
	for index, action := range networkActions {
		if action.id == target {
			model.action = index
		}
	}
	return model
}
