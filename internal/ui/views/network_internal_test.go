package views

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	corenetwork "utils/internal/core/network"
)

func TestNetworkBatchRunsHostsBackupBeforeRemove(t *testing.T) {
	model := NewNetworkModel()
	model.actions, _ = model.actions.SetChecked(networkActionKey(networkActionHostsRemoveCustom), true)
	model.actions, _ = model.actions.SetChecked(networkActionKey(networkActionHostsBackup), true)

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

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next, cmd = next.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(NetworkModel)
	if model.state != networkStateSelectingOptions || cmd != nil {
		t.Fatalf("expected enter on the default choice to cancel without running, state=%v cmd=%v", model.state, cmd != nil)
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

func TestNetworkBatchResultListsProblemsFirstThenEachActionInOrder(t *testing.T) {
	sections := networkLogSections([]networkActionResult{
		{action: networkActionHostsBackup, report: corenetwork.Report{Entries: []corenetwork.Entry{{Level: corenetwork.LevelSuccess, Message: "backed up"}}}},
		{action: networkActionFlushDNS, report: corenetwork.Report{Warnings: 1, Entries: []corenetwork.Entry{{Level: corenetwork.LevelWarn, Message: "no resolver"}}}},
		{action: networkActionHostsRemoveCustom, err: errors.New("permission denied")},
	})

	if len(sections) != 4 || sections[0].ID != "problems" || len(sections[0].Lines) != 2 {
		t.Fatalf("expected a problems section with the warning and the error, then one section per action, got %+v", sections)
	}
	for index, action := range []networkActionID{networkActionHostsBackup, networkActionFlushDNS, networkActionHostsRemoveCustom} {
		if !strings.Contains(sections[index+1].Title, actionTitle(action)) {
			t.Fatalf("expected section %d to be %q, got %q", index+1, actionTitle(action), sections[index+1].Title)
		}
	}

	single := networkLogSections([]networkActionResult{{action: networkActionViewConfig, report: corenetwork.Report{Entries: []corenetwork.Entry{{Level: corenetwork.LevelInfo, Message: "dns 1.1.1.1"}}}}})
	if len(single) != 1 || single[0].Title != "" {
		t.Fatalf("expected a single action without problems to have one untitled section, got %+v", single)
	}
}

func TestNetworkHostsFormKeepsAnInvalidEntryInTheForm(t *testing.T) {
	model := networkModelAt(networkActionHostsAdd)
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyTab})
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	next, cmd := next.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(NetworkModel)

	if model.state != networkStateEditingHostsAdd || model.hostsError == "" || cmd != nil {
		t.Fatalf("expected an invalid IP to keep the form open with its error, state=%v error=%q", model.state, model.hostsError)
	}
}

func networkModelAt(target networkActionID) NetworkModel {
	model := NewNetworkModel()
	model.actions.SelectByID(networkActionKey(target))
	return model
}
