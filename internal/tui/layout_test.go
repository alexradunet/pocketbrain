package tui

import "testing"

func TestLayoutModeBreakpoints(t *testing.T) {
	tests := []struct {
		width int
		want  LayoutMode
	}{
		{40, LayoutCompact},
		{59, LayoutCompact},
		{60, LayoutNormal},
		{80, LayoutNormal},
		{100, LayoutNormal},
		{101, LayoutWide},
		{120, LayoutWide},
	}
	for _, tt := range tests {
		got := layoutMode(tt.width)
		if got != tt.want {
			t.Errorf("layoutMode(%d) = %d, want %d", tt.width, got, tt.want)
		}
	}
}

func TestLayoutColumnsCompact(t *testing.T) {
	left, right := layoutColumns(40, LayoutCompact)
	if left != 40 || right != 0 {
		t.Errorf("compact: got %d,%d want 40,0", left, right)
	}
}

func TestLayoutColumnsNormal(t *testing.T) {
	left, right := layoutColumns(80, LayoutNormal)
	if left+right != 80 {
		t.Errorf("normal: columns don't sum to 80: %d+%d=%d", left, right, left+right)
	}
	if left != 48 {
		t.Errorf("normal: expected left=48 (60%% of 80), got %d", left)
	}
}

func TestLayoutColumnsWide(t *testing.T) {
	left, right := layoutColumns(120, LayoutWide)
	if left+right != 120 {
		t.Errorf("wide: columns don't sum to 120: %d+%d=%d", left, right, left+right)
	}
	if left != 60 || right != 60 {
		t.Errorf("wide: expected 60/60, got %d/%d", left, right)
	}
}
