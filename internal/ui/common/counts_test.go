package common

import (
	"strings"
	"testing"
)

func TestRenderCountsUsesTheSingularOnlyForOne(t *testing.T) {
	view := stripANSIForCheckboxTest(RenderCounts(80, []Count{
		{Label: "warnings", Singular: "warning", N: 1},
		{Label: "errors", Singular: "error", N: 2},
		{Label: "skipped", N: 1},
	}))
	for _, want := range []string{"1 warning ", "2 errors", "1 skipped"} {
		if !strings.Contains(view+" ", want) {
			t.Fatalf("expected %q in %q", want, view)
		}
	}
}
