package common

import "testing"

func TestSplitColumnsAtBoundaryWidths(t *testing.T) {
	for _, check := range []struct {
		width      int
		twoColumns bool
		left       int
	}{
		{width: TwoColumnMinWidth - 1, left: TwoColumnMinWidth - 1},
		{width: TwoColumnMinWidth, twoColumns: true, left: TwoColumnMinWidth * LeftColumnShare / 100},
		{width: 118, twoColumns: true, left: 53},
		{width: 198, twoColumns: true, left: LeftColumnMaxWidth},
	} {
		split := SplitColumns(check.width)
		if split.TwoColumns != check.twoColumns || split.Left != check.left {
			t.Fatalf("SplitColumns(%d) = %+v, want two columns %v with left %d", check.width, split, check.twoColumns, check.left)
		}
		if split.TwoColumns && split.Left+ColumnGap+split.Right != check.width {
			t.Fatalf("SplitColumns(%d) = %+v does not fill the width", check.width, split)
		}
		if !split.TwoColumns && split.Right != 0 {
			t.Fatalf("SplitColumns(%d) = %+v should leave no right column", check.width, split)
		}
	}
}
