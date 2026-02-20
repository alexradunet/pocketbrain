package tui

// LayoutMode describes the responsive layout tier.
type LayoutMode int

const (
	LayoutCompact LayoutMode = iota // < 60 cols: single column stacked
	LayoutNormal                    // 60-100 cols: 2 columns, 60/40
	LayoutWide                      // > 100 cols: 2 columns, 50/50
)

func layoutMode(width int) LayoutMode {
	switch {
	case width < 60:
		return LayoutCompact
	case width <= 100:
		return LayoutNormal
	default:
		return LayoutWide
	}
}

func layoutColumns(width int, mode LayoutMode) (left, right int) {
	switch mode {
	case LayoutCompact:
		return width, 0
	case LayoutNormal:
		left = width * 60 / 100
		right = width - left
		return left, right
	default: // LayoutWide
		left = width / 2
		right = width - left
		return left, right
	}
}
