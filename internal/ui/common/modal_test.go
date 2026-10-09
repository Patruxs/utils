package common

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestConfirmDefaultsToCancel(t *testing.T) {
	confirm, result := NewConfirm("Delete 7 files", "Delete").Update(tea.KeyMsg{Type: tea.KeyEnter})
	if result != ConfirmCanceled || confirm.Open() {
		t.Fatalf("enter on a fresh confirm must cancel and close it, got result %d open %v", result, confirm.Open())
	}
}

func TestConfirmKeepsWhatItConfirmsVisibleWhenShort(t *testing.T) {
	lines := []string{"Set Cloudflare DNS", "nmcli connection modify a", "nmcli connection up a", "", "Needs sudo"}
	view := ansi.Strip(NewConfirm("Change system settings", "Run", lines...).View(48, 7))
	if rows := strings.Count(view, "\n") + 1; rows > 7 {
		t.Fatalf("expected the confirm to fit 7 rows, got %d:\n%s", rows, view)
	}
	if !strings.Contains(view, "Set Cloudflare DNS") || !strings.Contains(view, "Cancel") {
		t.Fatalf("expected the action name and the buttons in a short confirm:\n%s", view)
	}
}
