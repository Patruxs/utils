package common

const (
	TwoColumnMinWidth  = 100 - 2*MarginX
	ColumnGap          = 1
	LeftColumnShare    = 45
	LeftColumnMaxWidth = 72
)

type ColumnSplit struct {
	TwoColumns bool
	Left       int
	Right      int
}

func SplitColumns(width int) ColumnSplit {
	if width < TwoColumnMinWidth {
		return ColumnSplit{Left: width}
	}
	left := MinInt(width*LeftColumnShare/100, LeftColumnMaxWidth)
	return ColumnSplit{TwoColumns: true, Left: left, Right: width - left - ColumnGap}
}
