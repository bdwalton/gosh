package vt

import (
	"testing"
)

func TestCursorMove(t *testing.T) {
	fb1 := newFramebuffer(24, 80)

	cases := []struct {
		t            *Terminal
		params       *parameters
		mt           byte // move type
		wantY, wantX int
	}{
		// CUU - cursor up
		{&Terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_CUU, 0, 0},
		{&Terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{2}), CSI_CUU, 0, 0},
		{&Terminal{fb: fb1, curY: 10, curX: 0}, paramsFromInts([]int{}), CSI_CUU, 9, 0},
		{&Terminal{fb: fb1, curY: 10, curX: 0}, paramsFromInts([]int{2}), CSI_CUU, 8, 0},
		// CUD - cursor down
		{&Terminal{fb: fb1, curY: fb1.getRows() - 1, curX: 1}, paramsFromInts([]int{}), CSI_CUD, fb1.getRows() - 1, 1},
		{&Terminal{fb: fb1, curY: fb1.getRows() - 1, curX: 1}, paramsFromInts([]int{3}), CSI_CUD, fb1.getRows() - 1, 1},
		{&Terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_CUD, 1, 0},
		{&Terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{3}), CSI_CUD, 3, 0},
		// CUB - cursor back
		{&Terminal{fb: fb1, curY: 15, curX: 0}, paramsFromInts([]int{}), CSI_CUB, 15, 0},
		{&Terminal{fb: fb1, curY: 15, curX: 0}, paramsFromInts([]int{2}), CSI_CUB, 15, 0},
		{&Terminal{fb: fb1, curY: 15, curX: 3}, paramsFromInts([]int{2}), CSI_CUB, 15, 1},
		{&Terminal{fb: fb1, curY: 15, curX: 79}, paramsFromInts([]int{}), CSI_CUB, 15, 78},
		// CUF - cursor forward
		{&Terminal{fb: fb1, curY: 15, curX: fb1.getCols() - 1}, paramsFromInts([]int{}), CSI_CUF, 15, fb1.getCols() - 1},
		{&Terminal{fb: fb1, curY: 15, curX: fb1.getCols() - 1}, paramsFromInts([]int{10}), CSI_CUF, 15, fb1.getCols() - 1},
		{&Terminal{fb: fb1, curY: 15, curX: 0}, paramsFromInts([]int{}), CSI_CUF, 15, 1},
		{&Terminal{fb: fb1, curY: 15, curX: 0}, paramsFromInts([]int{10}), CSI_CUF, 15, 10},
		// CPL - previous line
		{&Terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_CPL, 0, 0},
		{&Terminal{fb: fb1, curY: 0, curX: 10}, paramsFromInts([]int{20}), CSI_CPL, 0, 0},
		{&Terminal{fb: fb1, curY: 15, curX: 20}, paramsFromInts([]int{}), CSI_CPL, 14, 0},
		{&Terminal{fb: fb1, curY: 21, curX: 10}, paramsFromInts([]int{20}), CSI_CPL, 1, 0},

		// CNL - next line
		{&Terminal{fb: fb1, curY: fb1.getRows() - 1, curX: 0}, paramsFromInts([]int{}), CSI_CNL, fb1.getRows() - 1, 0},
		{&Terminal{fb: fb1, curY: fb1.getRows() - 1, curX: 0}, paramsFromInts([]int{2}), CSI_CNL, fb1.getRows() - 1, 0},
		{&Terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_CNL, 1, 0},
		{&Terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{10}), CSI_CNL, 10, 0},
		// CHA - cursor horizontal absolute
		{&Terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_CHA, 0, 0},
		{&Terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{10}), CSI_CHA, 0, 9},  // 0 vs 1 based
		{&Terminal{fb: fb1, curY: 0, curX: 10}, paramsFromInts([]int{10}), CSI_CHA, 0, 9}, // 0 vs 1 based
		{&Terminal{fb: fb1, curY: 0, curX: 10}, paramsFromInts([]int{fb1.getCols() + 10}), CSI_CHA, 0, fb1.getCols() - 1},
		// CUP - cursor position
		{&Terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_CUP, 0, 0},
		{&Terminal{fb: fb1, curY: 10, curX: 25}, paramsFromInts([]int{}), CSI_CUP, 0, 0},
		{&Terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_CUP, 0, 0},
		{&Terminal{fb: fb1, curY: 10, curX: 25}, paramsFromInts([]int{0, 16}), CSI_CUP, 0, 15},
		{&Terminal{fb: fb1, curY: 10, curX: 25}, paramsFromInts([]int{16}), CSI_CUP, 15, 0},
		{&Terminal{fb: fb1, curY: 10, curX: 25}, paramsFromInts([]int{1000, 1000}), CSI_CUP, fb1.getRows() - 1, fb1.getCols() - 1},
		// HVP - horizontal vertical position
		{&Terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_HVP, 0, 0},
		{&Terminal{fb: fb1, curY: 10, curX: 25}, paramsFromInts([]int{}), CSI_HVP, 0, 0},
		{&Terminal{fb: fb1, curY: 0, curX: 0}, paramsFromInts([]int{}), CSI_HVP, 0, 0},
		{&Terminal{fb: fb1, curY: 10, curX: 25}, paramsFromInts([]int{0, 16}), CSI_HVP, 0, 15},
		{&Terminal{fb: fb1, curY: 10, curX: 25}, paramsFromInts([]int{16}), CSI_HVP, 15, 0},
		{&Terminal{fb: fb1, curY: 10, curX: 25}, paramsFromInts([]int{1000, 1000}), CSI_HVP, fb1.getRows() - 1, fb1.getCols() - 1},
	}

	for i, c := range cases {
		c.t.cursorMove(c.params, c.mt)
		if c.t.curX != c.wantX || c.t.curY != c.wantY {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, c.t.curY, c.t.curX, c.wantY, c.wantX)
		}
	}
}
