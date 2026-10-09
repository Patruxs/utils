package common

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestConfirmDefaultsToCancel(t *testing.T) {
	confirm, result := NewConfirm("Delete 7 files", "Delete").Update(tea.KeyMsg{Type: tea.KeyEnter})
	if result != ConfirmCanceled || confirm.Open() {
		t.Fatalf("enter on a fresh confirm must cancel and close it, got result %d open %v", result, confirm.Open())
	}
}
