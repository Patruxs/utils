package common

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestContentWindowSizeReservesAppChrome(t *testing.T) {
	size := ContentWindowSize(tea.WindowSizeMsg{Width: 100, Height: 30}, 1)

	if size.Width != 96 {
		t.Fatalf("expected content width after horizontal padding, got %d", size.Width)
	}
	if size.Height != 27 {
		t.Fatalf("expected content height after app chrome and reserved title, got %d", size.Height)
	}
}

func TestWrapLineKeepsLongPathVisibleWithinWidth(t *testing.T) {
	const width = 44
	path := `C:\Users\Infra_IT_Intership_P\AppData\Local\Microsoft\Edge\User Data\Default\Cookies`

	lines := WrapLine("  log: ", path, width)
	for _, line := range lines {
		if len([]rune(line)) > width {
			t.Fatalf("expected line <= %d columns, got %d: %q", width, len([]rune(line)), line)
		}
	}

	if !strings.Contains(removeWhitespace(strings.Join(lines, "")), removeWhitespace(path)) {
		t.Fatalf("expected wrapped path to preserve full path:\n%s", strings.Join(lines, "\n"))
	}
}

func TestLayoutHeightsClampToAvailableSpace(t *testing.T) {
	layout := NewLayout(80, 18)

	if got := layout.ListHeight(100, 12, 3); got != 6 {
		t.Fatalf("expected list height to use remaining space, got %d", got)
	}
	if got := layout.ViewportHeight(10, 5, 20); got != 8 {
		t.Fatalf("expected viewport height to use remaining space, got %d", got)
	}
}

func removeWhitespace(value string) string {
	return strings.Join(strings.Fields(value), "")
}
