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
	dfb := func() *framebuffer {
		return newFramebuffer(10, 10)
	}

	wfb1 := newFramebuffer(10, 10)
	wfb1.setCell(0, 0, newCell('a', defFmt))

	wfb2 := newFramebuffer(10, 10)
	wfb2.setCell(0, 0, newCell('a', defFmt))
	wfb2.setCell(0, 1, newCell('b', defFmt))

	wfb3 := newFramebuffer(10, 10)
	wfb3.setCell(0, 0, newCell('ü', defFmt))

	wfb4 := newFramebuffer(10, 10)
	wfb4.setCell(0, 9, newCell('ü', defFmt))

	wfb5 := newFramebuffer(10, 10)
	wfb5.setCell(0, 9, newCell('z', defFmt))

	wfb6 := newFramebuffer(10, 10)
	wfb6.setCell(1, 0, newCell('z', defFmt))

	wfb7 := newFramebuffer(10, 10)
	wfb7.setCell(0, 8, fragCell('世', defFmt, FRAG_PRIMARY))
	wfb7.setCell(0, 9, fragCell(0, defFmt, FRAG_SECONDARY))

	wfb8 := newFramebuffer(10, 10)
	wfb8.setCell(1, 0, fragCell('世', defFmt, FRAG_PRIMARY))
	wfb8.setCell(1, 1, fragCell(0, defFmt, FRAG_SECONDARY))

	wfb9 := newFramebuffer(10, 10)
	wfb9.setCell(0, 5, fragCell('世', defFmt, FRAG_PRIMARY))
	wfb9.setCell(0, 6, fragCell(0, defFmt, FRAG_SECONDARY))

	ffb := newFramebuffer(10, 10)
	ffb.setCell(5, 5, fragCell('世', defFmt, FRAG_PRIMARY))
	ffb.setCell(5, 6, fragCell(0, defFmt, FRAG_SECONDARY))

	// We'll write a combining character at 5,6, which is the
	// fragmented second half of the wide cell in 5,5 (from
	// ffb). That should demonstrate overwritting a frag cell with
	// a complex case.
	wffb := newFramebuffer(10, 10)
	wffb.setCell(5, 5, defaultCell())
	wffb.setCell(5, 6, newCell('ü', defFmt))

	ffb2 := newFramebuffer(10, 10)
	ffb2.setCell(5, 5, fragCell('世', defFmt, FRAG_PRIMARY))
	ffb2.setCell(5, 6, fragCell(0, defFmt, FRAG_SECONDARY))

	// We'll write a combining character at 5,5, which is the
	// fragmented second half of the wide cell in 5,5 (from
	// ffb2). That should demonstrate overwritting a frag cell with
	// a complex case.
	wffb2 := newFramebuffer(10, 10)
	wffb2.setCell(5, 6, newCell('ü', defFmt))

	sfb := newFramebuffer(10, 10)
	sfb.setCell(8, 9, newCell('b', defFmt))
	sfb.setCell(9, 0, newCell('a', defFmt))

	wsfb := newFramebuffer(10, 10)
	wsfb.setCell(7, 9, newCell('b', defFmt))
	wsfb.setCell(8, 0, newCell('a', defFmt))
	wsfb.setCell(9, 0, newCell('ü', defFmt))

	wfb10 := newFramebuffer(10, 10)
	wfb10.setCell(9, 9, newCell('ü', defFmt))

	fb13 := dfb()
	fb13.setCell(5, 5, fragCell('世', defFmt, FRAG_PRIMARY))
	fb13.setCell(5, 6, fragCell(0, defFmt, FRAG_SECONDARY))
	wfb13 := dfb()
	wfb13.setCell(5, 5, newCell('a', defFmt))

	cases := []struct {
		cur      cursor
		autowrap bool
		fb       *framebuffer
		r        []rune
		wantCur  cursor
		wantFb   *framebuffer
	}{
		{cursor{0, 0}, false, dfb(), []rune("a"), cursor{0, 1}, wfb1},
		{cursor{0, 0}, false, dfb(), []rune("ab"), cursor{0, 2}, wfb2},
		{cursor{0, 0}, false, dfb(), []rune("u\u0308"), cursor{0, 1}, wfb3},
		{cursor{0, 9}, false, dfb(), []rune("u\u0308"), cursor{0, 10}, wfb4},
		{cursor{0, 9}, true, dfb(), []rune("u\u0308"), cursor{0, 10}, wfb4},
		{cursor{0, 10}, false, dfb(), []rune("z"), cursor{0, 10}, wfb5},
		{cursor{0, 10}, true, dfb(), []rune("z"), cursor{1, 1}, wfb6},
		{cursor{0, 10}, false, dfb(), []rune("世"), cursor{0, 10}, wfb7},
		{cursor{0, 10}, true, dfb(), []rune("世"), cursor{1, 2}, wfb8},
		{cursor{0, 5}, true, dfb(), []rune("世"), cursor{0, 7}, wfb9},
		{cursor{5, 6}, false, ffb, []rune("u\u0308"), cursor{5, 7}, wffb},
		{cursor{5, 6}, false, ffb2, []rune("u\u0308"), cursor{5, 7}, wffb2},
		{cursor{9, 10}, true, sfb, []rune("u\u0308"), cursor{9, 1}, wsfb},
		{cursor{9, 10}, false, dfb(), []rune("u\u0308"), cursor{9, 10}, wfb10},
		{cursor{5, 5}, false, fb13, []rune("a"), cursor{5, 6}, wfb13},
	}

	for i, c := range cases {
		term := NewTerminal(10, 10)
		term.fb = c.fb
		term.privAutowrap = c.autowrap
		term.cur = c.cur

		for _, r := range c.r {
			term.print(r)
		}

		if !term.cur.equal(c.wantCur) {
			t.Errorf("%d: Got %q, wanted %q", i, term.cur, c.wantCur)
		}

		if !term.fb.equal(c.wantFb) {
			t.Errorf("%d: Got:\n%s\nWant:\n%s", i, term.fb, c.wantFb)
		}
	}
}
