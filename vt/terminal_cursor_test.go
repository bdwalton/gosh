package vt

import (
	"testing"
)

func TestCursorVPA(t *testing.T) {
	fb1 := newFramebuffer(24, 80)
	cases := []struct {
		t                *Terminal
		n                int
		wantRow, wantCol int
	}{
		// VPA - vertical position absolute
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, 0, 0, fb1.getNumCols() - 1},
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, 1, 1, fb1.getNumCols() - 1},
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, 9, 9, 79},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, 3, 3, 0},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, 1000, fb1.getNumRows() - 1, 0},
	}

	for i, c := range cases {
		c.t.cursorVPA(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorHPR(t *testing.T) {
	fb1 := newFramebuffer(24, 80)
	cases := []struct {
		t                *Terminal
		n                int
		wantRow, wantCol int
	}{
		// HPR - horizontal position relative
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, 0, 15, fb1.getNumCols() - 1},
		{&Terminal{fb: fb1, cur: cursor{15, 5}}, 1, 15, 6},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, 1, 15, 1},
		{&Terminal{fb: fb1, cur: cursor{15, 5}}, 10, 15, 15},
	}

	for i, c := range cases {
		c.t.cursorHPR(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorVPR(t *testing.T) {
	fb1 := newFramebuffer(24, 80)
	cases := []struct {
		t                *Terminal
		n                int
		wantRow, wantCol int
	}{
		// VPR - vertical position relative
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, 0, 15, fb1.getNumCols() - 1},
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, 1, 16, fb1.getNumCols() - 1},
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, 5, 20, 79},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, 3, 18, 0},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, 1000, fb1.getNumRows() - 1, 0},
	}

	for i, c := range cases {
		c.t.cursorVPR(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorCHAorHPA(t *testing.T) {
	fb1 := newFramebuffer(24, 80)
	cases := []struct {
		t                *Terminal
		n                int
		wantRow, wantCol int
	}{
		// HPA - horizontal position absolute
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, 0, 15, 0},
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, 10, 15, 10},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, 3, 15, 3},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, 1000, 15, fb1.getNumCols() - 1},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, 0, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, 1, 0, 1},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, 10, 0, 10},
		{&Terminal{fb: fb1, cur: cursor{0, 10}}, 10, 0, 10},
		{&Terminal{fb: fb1, cur: cursor{0, 10}}, fb1.getNumCols() + 10, 0, fb1.getNumCols() - 1},
	}

	for i, c := range cases {
		c.t.cursorCHAorHPA(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorCUPorHVP(t *testing.T) {
	fb1 := newFramebuffer(24, 80)
	cases := []struct {
		t                *Terminal
		m, n             int
		wantRow, wantCol int
	}{
		// CUP - cursor position
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, 0, 0, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, 0, 0, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, 0, 15, 0, 15},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, 15, 0, 15, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, 15, 16, 15, 16},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, 1000, 1000, fb1.getNumRows() - 1, fb1.getNumCols() - 1},
	}

	for i, c := range cases {
		c.t.cursorCUPorHVP(c.m, c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorUp(t *testing.T) {
	fb1 := newFramebuffer(24, 80)
	cases := []struct {
		t                *Terminal
		n                int
		wantRow, wantCol int
	}{
		// CUU - cursor up
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, 1, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{1, 0}}, 0, 1, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, 2, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 0}}, 1, 9, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 0}}, 2, 8, 0},
		{&Terminal{fb: fb1, vertMargin: newMargin(5, 15), cur: cursor{10, 0}}, 2, 8, 0},
		{&Terminal{fb: fb1, vertMargin: newMargin(10, 15), cur: cursor{10, 0}}, 2, 10, 0},
		{&Terminal{fb: fb1, vertMargin: newMargin(5, 15), cur: cursor{10, 0}}, 6, 5, 0},
		{&Terminal{fb: fb1, vertMargin: newMargin(5, 15), cur: cursor{4, 0}}, 2, 2, 0},
	}

	for i, c := range cases {
		c.t.cursorUp(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorDown(t *testing.T) {
	fb1 := newFramebuffer(24, 80)
	cases := []struct {
		t                *Terminal
		n                int
		wantRow, wantCol int
	}{
		// CUD - cursor down
		{&Terminal{fb: fb1, cur: cursor{fb1.getNumRows() - 1, 1}}, 0, fb1.getNumRows() - 1, 1},
		{&Terminal{fb: fb1, cur: cursor{15, 1}}, 0, 15, 1},
		{&Terminal{fb: fb1, cur: cursor{fb1.getNumRows() - 1, 1}}, 1, fb1.getNumRows() - 1, 1},
		{&Terminal{fb: fb1, cur: cursor{fb1.getNumRows() - 1, 1}}, 3, fb1.getNumRows() - 1, 1},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, 1, 1, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, 3, 3, 0},
		{&Terminal{fb: fb1, vertMargin: newMargin(5, 15), cur: cursor{10, 0}}, 2, 12, 0},
		{&Terminal{fb: fb1, vertMargin: newMargin(5, 10), cur: cursor{10, 0}}, 2, 10, 0},
		{&Terminal{fb: fb1, vertMargin: newMargin(5, 15), cur: cursor{10, 0}}, 6, 15, 0},
		{&Terminal{fb: fb1, vertMargin: newMargin(5, 15), cur: cursor{16, 0}}, 6, 22, 0},
	}

	for i, c := range cases {
		c.t.cursorDown(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorBack(t *testing.T) {
	fb1 := newFramebuffer(24, 80)
	cases := []struct {
		t                *Terminal
		n                int
		wantRow, wantCol int
	}{
		// CUB - cursor back
		{&Terminal{fb: fb1, cur: cursor{15, 15}}, 0, 15, 15},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, 1, 15, 0},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, 2, 15, 0},
		{&Terminal{fb: fb1, cur: cursor{15, 3}}, 2, 15, 1},
		{&Terminal{fb: fb1, cur: cursor{15, 79}}, 1, 15, 78},
		{&Terminal{fb: fb1, horizMargin: newMargin(5, 15), cur: cursor{15, 0}}, 2, 15, 0},
		{&Terminal{fb: fb1, horizMargin: newMargin(5, 15), cur: cursor{15, 5}}, 2, 15, 5},
		{&Terminal{fb: fb1, horizMargin: newMargin(5, 15), cur: cursor{10, 5}}, 6, 10, 5},
		{&Terminal{fb: fb1, horizMargin: newMargin(5, 15), cur: cursor{10, 4}}, 2, 10, 2},
	}

	for i, c := range cases {
		c.t.cursorBack(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorForward(t *testing.T) {
	fb1 := newFramebuffer(24, 80)
	cases := []struct {
		t                *Terminal
		n                int
		wantRow, wantCol int
	}{
		// CUF - cursor forward
		{&Terminal{fb: fb1, cur: cursor{15, 1}}, 0, 15, 1},
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, 0, 15, fb1.getNumCols() - 1},
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, 1, 15, fb1.getNumCols() - 1},
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, 10, 15, fb1.getNumCols() - 1},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, 1, 15, 1},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, 10, 15, 10},
		{&Terminal{fb: fb1, horizMargin: newMargin(5, 15), cur: cursor{15, 0}}, 2, 15, 2},
		{&Terminal{fb: fb1, horizMargin: newMargin(5, 15), cur: cursor{15, 5}}, 2, 15, 7},
		{&Terminal{fb: fb1, horizMargin: newMargin(5, 15), cur: cursor{10, 10}}, 6, 10, 15},
		{&Terminal{fb: fb1, horizMargin: newMargin(5, 15), cur: cursor{10, 16}}, 2, 10, 18},
	}

	for i, c := range cases {
		c.t.cursorForward(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorCPL(t *testing.T) {
	fb1 := newFramebuffer(24, 80)
	cases := []struct {
		t                *Terminal
		n                int
		wantRow, wantCol int
	}{
		// CPL - previous line
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, 1, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, 0, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{1, 0}}, 0, 1, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 10}}, 20, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{15, 20}}, 1, 14, 0},
		{&Terminal{fb: fb1, cur: cursor{21, 10}}, 20, 1, 0},
	}

	for i, c := range cases {
		c.t.cursorCPL(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorCNL(t *testing.T) {
	fb1 := newFramebuffer(24, 80)
	cases := []struct {
		t                *Terminal
		n                int
		wantRow, wantCol int
	}{
		// CNL - next line
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, 0, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{15, 1}}, 0, 15, 0},
		{&Terminal{fb: fb1, cur: cursor{fb1.getNumRows() - 1, 0}}, 0, fb1.getNumRows() - 1, 0},
		{&Terminal{fb: fb1, cur: cursor{fb1.getNumRows() - 1, 0}}, 1, fb1.getNumRows() - 1, 0},
		{&Terminal{fb: fb1, cur: cursor{fb1.getNumRows() - 1, 0}}, 2, fb1.getNumRows() - 1, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, 1, 1, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, 10, 10, 0},
	}

	for i, c := range cases {
		c.t.cursorCNL(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}
