package vt

import (
	"testing"
)

func TestCursorMove(t *testing.T) {
	fb1 := newFramebuffer(24, 80)

	cases := []struct {
		t            *terminal
		params       *parameters
		mt           byte // move type
		wantY, wantX int
	}{
		// CUU - cursor up
		{&terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_CUU, 0, 0},
		{&terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{2}), CSI_CUU, 0, 0},
		{&terminal{fb: fb1, curY: 10, curX: 0}, paramsFromInts([]int{}), CSI_CUU, 9, 0},
		{&terminal{fb: fb1, curY: 10, curX: 0}, paramsFromInts([]int{2}), CSI_CUU, 8, 0},
		// CUD - cursor down
		{&terminal{fb: fb1, curY: fb1.rows - 1, curX: 1}, paramsFromInts([]int{}), CSI_CUD, fb1.rows - 1, 1},
		{&terminal{fb: fb1, curY: fb1.rows - 1, curX: 1}, paramsFromInts([]int{3}), CSI_CUD, fb1.rows - 1, 1},
		{&terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_CUD, 1, 0},
		{&terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{3}), CSI_CUD, 3, 0},
		// CUB - cursor back
		{&terminal{fb: fb1, curY: 15, curX: 0}, paramsFromInts([]int{}), CSI_CUB, 15, 0},
		{&terminal{fb: fb1, curY: 15, curX: 0}, paramsFromInts([]int{2}), CSI_CUB, 15, 0},
		{&terminal{fb: fb1, curY: 15, curX: 3}, paramsFromInts([]int{2}), CSI_CUB, 15, 1},
		{&terminal{fb: fb1, curY: 15, curX: 79}, paramsFromInts([]int{}), CSI_CUB, 15, 78},
		// CUF - cursor forward
		{&terminal{fb: fb1, curY: 15, curX: fb1.cols - 1}, paramsFromInts([]int{}), CSI_CUF, 15, fb1.cols - 1},
		{&terminal{fb: fb1, curY: 15, curX: fb1.cols - 1}, paramsFromInts([]int{10}), CSI_CUF, 15, fb1.cols - 1},
		{&terminal{fb: fb1, curY: 15, curX: 0}, paramsFromInts([]int{}), CSI_CUF, 15, 1},
		{&terminal{fb: fb1, curY: 15, curX: 0}, paramsFromInts([]int{10}), CSI_CUF, 15, 10},
		// CPL - previous line
		{&terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_CPL, 0, 0},
		{&terminal{fb: fb1, curY: 0, curX: 10}, paramsFromInts([]int{20}), CSI_CPL, 0, 0},
		{&terminal{fb: fb1, curY: 15, curX: 20}, paramsFromInts([]int{}), CSI_CPL, 14, 0},
		{&terminal{fb: fb1, curY: 21, curX: 10}, paramsFromInts([]int{20}), CSI_CPL, 1, 0},

		// CNL - next line
		{&terminal{fb: fb1, curY: fb1.rows - 1, curX: 0}, paramsFromInts([]int{}), CSI_CNL, fb1.rows - 1, 0},
		{&terminal{fb: fb1, curY: fb1.rows - 1, curX: 0}, paramsFromInts([]int{2}), CSI_CNL, fb1.rows - 1, 0},
		{&terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_CNL, 1, 0},
		{&terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{10}), CSI_CNL, 10, 0},
		// CHA - cursor horizontal absolute
		{&terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_CHA, 0, 0},
		{&terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{10}), CSI_CHA, 0, 9},  // 0 vs 1 based
		{&terminal{fb: fb1, curY: 0, curX: 10}, paramsFromInts([]int{10}), CSI_CHA, 0, 9}, // 0 vs 1 based
		{&terminal{fb: fb1, curY: 0, curX: 10}, paramsFromInts([]int{fb1.cols + 10}), CSI_CHA, 0, fb1.cols - 1},
		// CUP - cursor position
		{&terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_CUP, 0, 0},
		{&terminal{fb: fb1, curY: 10, curX: 25}, paramsFromInts([]int{}), CSI_CUP, 0, 0},
		{&terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_CUP, 0, 0},
		{&terminal{fb: fb1, curY: 10, curX: 25}, paramsFromInts([]int{0, 16}), CSI_CUP, 0, 15},
		{&terminal{fb: fb1, curY: 10, curX: 25}, paramsFromInts([]int{16}), CSI_CUP, 15, 0},
		{&terminal{fb: fb1, curY: 10, curX: 25}, paramsFromInts([]int{1000, 1000}), CSI_CUP, fb1.rows - 1, fb1.cols - 1},
		// HVP - horizontal vertical position
		{&terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_HVP, 0, 0},
		{&terminal{fb: fb1, curY: 10, curX: 25}, paramsFromInts([]int{}), CSI_HVP, 0, 0},
		{&terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_HVP, 0, 0},
		{&terminal{fb: fb1, curY: 10, curX: 25}, paramsFromInts([]int{0, 16}), CSI_HVP, 0, 15},
		{&terminal{fb: fb1, curY: 10, curX: 25}, paramsFromInts([]int{16}), CSI_HVP, 15, 0},
		{&terminal{fb: fb1, curY: 10, curX: 25}, paramsFromInts([]int{1000, 1000}), CSI_HVP, fb1.rows - 1, fb1.cols - 1},
	}

	for i, c := range cases {
		c.t.cursorMove(c.params, c.mt)
		if c.t.curX != c.wantX || c.t.curY != c.wantY {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, c.t.curY, c.t.curX, c.wantY, c.wantX)
		}
	}
}
