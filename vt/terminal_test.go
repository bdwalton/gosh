package vt

import (
	"testing"
)

func TestCursorMove(t *testing.T) {
	fb1 := newFramebuffer(24, 80)

	cases := []struct {
		t                *Terminal
		params           *parameters
		mt               byte // move type
		wantRow, wantCol int
	}{
		// CUU - cursor up
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{}), CSI_CUU, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{2}), CSI_CUU, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 0}}, paramsFromInts([]int{}), CSI_CUU, 9, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 0}}, paramsFromInts([]int{2}), CSI_CUU, 8, 0},
		// CUD - cursor down
		{&Terminal{fb: fb1, cur: cursor{fb1.getRows() - 1, 1}}, paramsFromInts([]int{}), CSI_CUD, fb1.getRows() - 1, 1},
		{&Terminal{fb: fb1, cur: cursor{fb1.getRows() - 1, 1}}, paramsFromInts([]int{3}), CSI_CUD, fb1.getRows() - 1, 1},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{}), CSI_CUD, 1, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{3}), CSI_CUD, 3, 0},
		// CUB - cursor back
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, paramsFromInts([]int{}), CSI_CUB, 15, 0},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, paramsFromInts([]int{2}), CSI_CUB, 15, 0},
		{&Terminal{fb: fb1, cur: cursor{15, 3}}, paramsFromInts([]int{2}), CSI_CUB, 15, 1},
		{&Terminal{fb: fb1, cur: cursor{15, 79}}, paramsFromInts([]int{}), CSI_CUB, 15, 78},
		// CUF - cursor forward
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getCols() - 1}}, paramsFromInts([]int{}), CSI_CUF, 15, fb1.getCols() - 1},
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getCols() - 1}}, paramsFromInts([]int{10}), CSI_CUF, 15, fb1.getCols() - 1},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, paramsFromInts([]int{}), CSI_CUF, 15, 1},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, paramsFromInts([]int{10}), CSI_CUF, 15, 10},
		// CPL - previous line
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{}), CSI_CPL, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 10}}, paramsFromInts([]int{20}), CSI_CPL, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{15, 20}}, paramsFromInts([]int{}), CSI_CPL, 14, 0},
		{&Terminal{fb: fb1, cur: cursor{21, 10}}, paramsFromInts([]int{20}), CSI_CPL, 1, 0},

		// CNL - next line
		{&Terminal{fb: fb1, cur: cursor{fb1.getRows() - 1, 0}}, paramsFromInts([]int{}), CSI_CNL, fb1.getRows() - 1, 0},
		{&Terminal{fb: fb1, cur: cursor{fb1.getRows() - 1, 0}}, paramsFromInts([]int{2}), CSI_CNL, fb1.getRows() - 1, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{}), CSI_CNL, 1, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{10}), CSI_CNL, 10, 0},
		// CHA - cursor horizontal absolute
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{}), CSI_CHA, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{10}), CSI_CHA, 0, 9},  // 0 vs 1 based
		{&Terminal{fb: fb1, cur: cursor{0, 10}}, paramsFromInts([]int{10}), CSI_CHA, 0, 9}, // 0 vs 1 based
		{&Terminal{fb: fb1, cur: cursor{0, 10}}, paramsFromInts([]int{fb1.getCols() + 10}), CSI_CHA, 0, fb1.getCols() - 1},
		// CUP - cursor position
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{}), CSI_CUP, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, paramsFromInts([]int{}), CSI_CUP, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{}), CSI_CUP, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, paramsFromInts([]int{0, 16}), CSI_CUP, 0, 15},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, paramsFromInts([]int{16}), CSI_CUP, 15, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, paramsFromInts([]int{1000, 1000}), CSI_CUP, fb1.getRows() - 1, fb1.getCols() - 1},
		// HVP - horizontal vertical position
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{}), CSI_HVP, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, paramsFromInts([]int{}), CSI_HVP, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{}), CSI_HVP, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, paramsFromInts([]int{0, 16}), CSI_HVP, 0, 15},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, paramsFromInts([]int{16}), CSI_HVP, 15, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, paramsFromInts([]int{1000, 1000}), CSI_HVP, fb1.getRows() - 1, fb1.getCols() - 1},
	}

	for i, c := range cases {
		c.t.cursorMove(c.params, c.mt)
		if c.t.cur.col != c.wantCol || c.t.cur.row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, c.t.cur.row, c.t.cur.col, c.wantRow, c.wantCol)
		}
	}
}

func TestLineFeed(t *testing.T) {
	xc := newCell('x', defFmt)
	term := &Terminal{fb: newFramebuffer(10, 10)}
	term.fb.setCell(9, 5, xc)

	cases := []struct {
		row, col         int //current cursor
		wantRow, wantCol int
	}{
		{0, 0, 1, 0},
		{1, 1, 2, 1},
		{9, 5, 9, 5}, // should scroll.
	}

	for i, c := range cases {
		term.cur.row = c.row
		term.cur.col = c.col
		if c.row == c.wantRow {
			gc, _ := term.fb.getCell(c.row, c.col)
			if !gc.equal(xc) {
				t.Errorf("%d: Invalid cell setup. Got %v, wanted %v", i, gc, xc)
			}
		}
		term.lineFeed()
		if c.row == c.wantRow {
			gc, _ := term.fb.getCell(c.row-1, c.col)
			if !gc.equal(xc) {
				t.Errorf("%d: Invalid linefeed scroll (old line). Got %v, wanted %v", i, gc, xc)
			}
			gc, _ = term.fb.getCell(c.row, c.col)
			if !gc.equal(defaultCell()) {
				t.Errorf("%d: Invalid linefeed scroll (new line). Got %v, wanted %v", i, gc, xc)
			}
		}
		if row, col := term.cur.row, term.cur.col; row != c.wantRow || col != c.wantCol {
			t.Errorf("%d: Got (%d, %d), wanted (%d, %d)", i, row, col, c.wantRow, c.wantCol)
		}

	}
}

func TestPrint(t *testing.T) {
	cases := []struct {
		cur      cursor
		autowrap bool
		d        cell
		r        []rune
		wantCur  cursor
		cellCur  cursor
		wantCell cell
	}{
		{
			cursor{0, 0},
			false,
			defaultCell(),
			[]rune("a"),
			cursor{0, 1},
			cursor{0, 0},
			newCell('a', defFmt),
		},
		{
			cursor{0, 0},
			false,
			defaultCell(),
			[]rune("ab"),
			cursor{0, 2},
			cursor{0, 1},
			newCell('b', defFmt),
		},
		{
			cursor{0, 0},
			false,
			defaultCell(),
			[]rune("u\u0308"),
			cursor{0, 1},
			cursor{0, 0},
			newCell('ü', defFmt),
		},
		{
			cursor{0, 9},
			false,
			defaultCell(),
			[]rune("u\u0308"),
			cursor{0, 10},
			cursor{0, 9},
			newCell('ü', defFmt),
		},
		{
			cursor{0, 9},
			true,
			defaultCell(),
			[]rune("u\u0308"),
			cursor{0, 10},
			cursor{0, 9},
			newCell('ü', defFmt),
		},
		{
			cursor{0, 10}, // already printed last char in row
			false,         // no autowrap
			defaultCell(),
			[]rune("z"),
			cursor{0, 10},
			cursor{0, 9},
			newCell('z', defFmt), // we should overwrite the last cell
		},
		{
			cursor{0, 10}, // already printed last char in row
			true,          // autowrap
			defaultCell(),
			[]rune("z"),
			cursor{1, 1},
			cursor{1, 0},
			newCell('z', defFmt),
		},
		{
			cursor{0, 10}, // already printed last char in row
			false,         // no autowrap
			defaultCell(),
			[]rune("世"),
			cursor{0, 10},
			cursor{0, 8},
			newCell('世', defFmt),
		},
		{
			cursor{0, 10}, // already printed last char in row
			true,          // no autowrap
			defaultCell(),
			[]rune("世"),
			cursor{1, 2},
			cursor{1, 0},
			newCell('世', defFmt),
		},
		{
			cursor{0, 5}, // already printed last char in row
			true,         // no autowrap
			defaultCell(),
			[]rune("世"),
			cursor{0, 7},
			cursor{0, 5},
			newCell('世', defFmt),
		},
	}

	for i, c := range cases {
		term := NewTerminal(10, 10)
		term.privAutowrap = c.autowrap
		term.cur = c.cur
		term.fb.setCell(c.cur.row, c.cur.col, c.d)

		for _, r := range c.r {
			term.print(r)
		}

		if !term.cur.equal(c.wantCur) {
			t.Errorf("%d: Got %q, wanted %q", i, term.cur, c.wantCur)
		}

		if w, _ := term.fb.getCell(c.cellCur.row, c.cellCur.col); !w.equal(c.wantCell) {
			t.Errorf("%d: Got %v, wanted %v; %v", i, w, c.wantCell, term.fb.data[c.wantCur.row])
		}
	}
}
