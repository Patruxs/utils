package common

import (
	"strings"
	"testing"
)

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

func removeWhitespace(value string) string {
	return strings.Join(strings.Fields(value), "")
}
