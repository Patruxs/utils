package views

import (
	"errors"
	"regexp"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	corenetwork "utils/internal/core/network"
	"utils/internal/ui/common"
)

func TestNetworkBatchRunsHostsBackupBeforeRemove(t *testing.T) {
	model := NewNetworkModel()
	model.toggleBatch(networkActionHostsRemoveCustom)
	model.toggleBatch(networkActionHostsBackup)

	actions := model.selectedOrCurrentActionIDs()

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
	if !model.confirm.Open() || model.state == networkStateRunning || cmd != nil {
		t.Fatalf("expected enter on a write action to ask for confirmation without running, state=%v cmd=%v", model.state, cmd != nil)
	}

	next, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = next.(NetworkModel)
	assertCanceledWithoutRunning(t, model, cmd)

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next, cmd = next.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(NetworkModel)
	assertCanceledWithoutRunning(t, model, cmd)
}

func assertCanceledWithoutRunning(t *testing.T, model NetworkModel, cmd tea.Cmd) {
	t.Helper()
	if model.confirm.Open() || model.state != networkStateSelectingOptions || cmd == nil {
		t.Fatalf("expected the confirmation to close back to the actions, state=%v open=%v", model.state, model.confirm.Open())
	}
	if _, ok := cmd().(common.NoticeMsg); !ok {
		t.Fatal("expected canceling to only show a notice, not start a run")
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
	if sections[3].Glyph != common.ToneDanger.Glyph() || sections[1].Glyph != common.ToneSuccess.Glyph() {
		t.Fatalf("expected each batch section to carry its own result glyph, got %q and %q", sections[1].Glyph, sections[3].Glyph)
	}

	single := networkLogSections([]networkActionResult{{action: networkActionViewConfig, report: corenetwork.Report{Entries: []corenetwork.Entry{{Level: corenetwork.LevelInfo, Message: "dns 1.1.1.1"}}}}})
	if len(single) != 1 || single[0].Title != "" {
		t.Fatalf("expected a single action without problems to have one untitled section, got %+v", single)
	}
}

func TestNetworkHostsFormRefusesInvalidInput(t *testing.T) {
	model := networkModelAt(networkActionHostsAdd)
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyTab})
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	next, cmd := next.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(NetworkModel)

	if model.state != networkStateEditingHostsAdd || model.confirm.Open() || cmd != nil {
		t.Fatalf("expected an invalid IP to keep the form open, state=%v confirm=%v", model.state, model.confirm.Open())
	}
	if view := plainText(model.View()); !strings.Contains(view, common.ToneDanger.Glyph()+" hosts IP") {
		t.Fatalf("expected the IP error under the field:\n%s", view)
	}
}

func TestNetworkStatusCardRendersWithMissingFields(t *testing.T) {
	model := sizedNetworkModel(120, 37)
	next, _ := model.Update(common.ActivatedMsg{})
	model = next.(NetworkModel)
	if view := plainText(model.View()); !strings.Contains(view, "loading…") {
		t.Fatalf("expected the card to show loading before the status arrives:\n%s", view)
	}

	next, _ = model.Update(networkStatusMsg{seq: model.statusSeq, status: corenetwork.Status{Adapter: "eth0", Sudo: corenetwork.StateOff}})
	view := plainText(next.(NetworkModel).View())

	card := strings.Join(strings.Split(view, "\n")[:networkCardRows], "\n")
	if !strings.Contains(card, "eth0") || !strings.Contains(card, "gw ?") || !strings.Contains(card, "DNS ?") || strings.Contains(card, "loading") {
		t.Fatalf("expected the known adapter and ? for each unknown field:\n%s", card)
	}
}

func TestNetworkBatchModeAppearsOnFirstSpace(t *testing.T) {
	model := sizedNetworkModel(120, 37)
	if model.FooterStatus() != "" || strings.Contains(plainText(model.View()), "[ ]") {
		t.Fatal("expected no batch column before the first space")
	}

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	model = next.(NetworkModel)

	if model.FooterStatus() != "1 in batch" || !strings.Contains(plainText(model.View()), "[x] View config") {
		t.Fatalf("expected the first space to reveal the batch column and count, footer=%q:\n%s", model.FooterStatus(), plainText(model.View()))
	}
}

func TestNetworkRowsForOtherSystemsCannotBeSelected(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("DoH rows apply on Windows")
	}
	model := sizedNetworkModel(120, 37)
	model.setFocus(networkActionFlushDNS)

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})

	if focus := next.(NetworkModel).focus; focus != networkActionOptimize {
		t.Fatalf("expected the cursor to skip the Windows-only DoH rows, got %q", actionTitle(focus))
	}
}

func networkModelAt(target networkActionID) NetworkModel {
	model := NewNetworkModel()
	model.setFocus(target)
	return model
}

func sizedNetworkModel(width, height int) NetworkModel {
	next, _ := NewNetworkModel().Update(tea.WindowSizeMsg{Width: width, Height: height})
	return next.(NetworkModel)
}

func plainText(view string) string {
	return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(view, "")
}
