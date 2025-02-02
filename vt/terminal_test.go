package vt

import (
	"fmt"
	"slices"
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
		{&Terminal{fb: fb1, vertMargin: newMargin(5, 15), cur: cursor{10, 0}}, paramsFromInts([]int{2}), CSI_CUU, 8, 0},
		{&Terminal{fb: fb1, vertMargin: newMargin(10, 15), cur: cursor{10, 0}}, paramsFromInts([]int{2}), CSI_CUU, 10, 0},
		{&Terminal{fb: fb1, vertMargin: newMargin(5, 15), cur: cursor{10, 0}}, paramsFromInts([]int{6}), CSI_CUU, 5, 0},
		{&Terminal{fb: fb1, vertMargin: newMargin(5, 15), cur: cursor{4, 0}}, paramsFromInts([]int{2}), CSI_CUU, 2, 0},
		// CUD - cursor down
		{&Terminal{fb: fb1, cur: cursor{fb1.getRows() - 1, 1}}, paramsFromInts([]int{}), CSI_CUD, fb1.getRows() - 1, 1},
		{&Terminal{fb: fb1, cur: cursor{fb1.getRows() - 1, 1}}, paramsFromInts([]int{3}), CSI_CUD, fb1.getRows() - 1, 1},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{}), CSI_CUD, 1, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{3}), CSI_CUD, 3, 0},
		{&Terminal{fb: fb1, vertMargin: newMargin(5, 15), cur: cursor{10, 0}}, paramsFromInts([]int{2}), CSI_CUD, 12, 0},
		{&Terminal{fb: fb1, vertMargin: newMargin(5, 10), cur: cursor{10, 0}}, paramsFromInts([]int{2}), CSI_CUD, 10, 0},
		{&Terminal{fb: fb1, vertMargin: newMargin(5, 15), cur: cursor{10, 0}}, paramsFromInts([]int{6}), CSI_CUD, 15, 0},
		{&Terminal{fb: fb1, vertMargin: newMargin(5, 15), cur: cursor{16, 0}}, paramsFromInts([]int{6}), CSI_CUD, 22, 0},
		// CUB - cursor back
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, paramsFromInts([]int{}), CSI_CUB, 15, 0},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, paramsFromInts([]int{2}), CSI_CUB, 15, 0},
		{&Terminal{fb: fb1, cur: cursor{15, 3}}, paramsFromInts([]int{2}), CSI_CUB, 15, 1},
		{&Terminal{fb: fb1, cur: cursor{15, 79}}, paramsFromInts([]int{}), CSI_CUB, 15, 78},
		{&Terminal{fb: fb1, horizMargin: newMargin(5, 15), cur: cursor{15, 0}}, paramsFromInts([]int{2}), CSI_CUB, 15, 0},
		{&Terminal{fb: fb1, horizMargin: newMargin(5, 15), cur: cursor{15, 5}}, paramsFromInts([]int{2}), CSI_CUB, 15, 5},
		{&Terminal{fb: fb1, horizMargin: newMargin(5, 15), cur: cursor{10, 5}}, paramsFromInts([]int{6}), CSI_CUB, 10, 5},
		{&Terminal{fb: fb1, horizMargin: newMargin(5, 15), cur: cursor{10, 4}}, paramsFromInts([]int{2}), CSI_CUB, 10, 2},

		// CUF - cursor forward
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getCols() - 1}}, paramsFromInts([]int{}), CSI_CUF, 15, fb1.getCols() - 1},
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getCols() - 1}}, paramsFromInts([]int{10}), CSI_CUF, 15, fb1.getCols() - 1},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, paramsFromInts([]int{}), CSI_CUF, 15, 1},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, paramsFromInts([]int{10}), CSI_CUF, 15, 10},
		{&Terminal{fb: fb1, horizMargin: newMargin(5, 15), cur: cursor{15, 0}}, paramsFromInts([]int{2}), CSI_CUF, 15, 2},
		{&Terminal{fb: fb1, horizMargin: newMargin(5, 15), cur: cursor{15, 5}}, paramsFromInts([]int{2}), CSI_CUF, 15, 7},
		{&Terminal{fb: fb1, horizMargin: newMargin(5, 15), cur: cursor{10, 10}}, paramsFromInts([]int{6}), CSI_CUF, 10, 15},
		{&Terminal{fb: fb1, horizMargin: newMargin(5, 15), cur: cursor{10, 16}}, paramsFromInts([]int{2}), CSI_CUF, 10, 18},
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
	defTerm := func() *Terminal {
		t := &Terminal{fb: newFramebuffer(10, 10)}
		t.fb.setCell(9, 5, xc)
		return t
	}

	cases := []struct {
		row, col         int //current cursor
		t                *Terminal
		wantRow, wantCol int
	}{
		{0, 0, defTerm(), 1, 0},
		{1, 1, defTerm(), 2, 1},
		{9, 5, defTerm(), 9, 5}, // should scroll.
	}

	for i, c := range cases {
		c.t.cur.row = c.row
		c.t.cur.col = c.col
		if c.row == c.wantRow {
			gc, _ := c.t.fb.getCell(c.row, c.col)
			if !gc.equal(xc) {
				t.Errorf("%d: Invalid cell setup. Got %v, wanted %v", i, gc, xc)
			}
		}
		c.t.lineFeed()
		if c.row == c.wantRow {
			gc, _ := c.t.fb.getCell(c.row-1, c.col)
			if !gc.equal(xc) {
				t.Errorf("%d: Invalid linefeed scroll (old line). Got %v, wanted %v", i, gc, xc)
			}
			gc, _ = c.t.fb.getCell(c.row, c.col)
			if !gc.equal(defaultCell()) {
				t.Errorf("%d: Invalid linefeed scroll (new line). Got %v, wanted %v", i, gc, xc)
			}
		}
		if row, col := c.t.cur.row, c.t.cur.col; row != c.wantRow || col != c.wantCol {
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

	dterm := func(c cursor, fb *framebuffer, wrap bool) *Terminal {
		t := NewTerminal(nil, 10, 10)
		t.fb = fb
		t.cur = c
		t.privAutowrap = wrap
		return t
	}

	cases := []struct {
		t       *Terminal
		r       []rune
		wantCur cursor
		wantFb  *framebuffer
	}{
		{dterm(cursor{0, 0}, dfb(), false), []rune("a"), cursor{0, 1}, wfb1},
		{dterm(cursor{0, 0}, dfb(), false), []rune("ab"), cursor{0, 2}, wfb2},
		{dterm(cursor{0, 0}, dfb(), false), []rune("u\u0308"), cursor{0, 1}, wfb3},
		{dterm(cursor{0, 9}, dfb(), false), []rune("u\u0308"), cursor{0, 10}, wfb4},
		{dterm(cursor{0, 9}, dfb(), true), []rune("u\u0308"), cursor{0, 10}, wfb4},
		{dterm(cursor{0, 10}, dfb(), false), []rune("z"), cursor{0, 10}, wfb5},
		{dterm(cursor{0, 10}, dfb(), true), []rune("z"), cursor{1, 1}, wfb6},
		{dterm(cursor{0, 10}, dfb(), false), []rune("世"), cursor{0, 10}, wfb7},
		{dterm(cursor{0, 10}, dfb(), true), []rune("世"), cursor{1, 2}, wfb8},
		{dterm(cursor{0, 5}, dfb(), true), []rune("世"), cursor{0, 7}, wfb9},
		{dterm(cursor{5, 6}, ffb, false), []rune("u\u0308"), cursor{5, 7}, wffb},
		{dterm(cursor{5, 6}, ffb2, false), []rune("u\u0308"), cursor{5, 7}, wffb2},
		{dterm(cursor{9, 10}, sfb, true), []rune("u\u0308"), cursor{9, 1}, wsfb},
		{dterm(cursor{9, 10}, dfb(), false), []rune("u\u0308"), cursor{9, 10}, wfb10},
		{dterm(cursor{5, 5}, fb13, false), []rune("a"), cursor{5, 6}, wfb13},
	}

	for i, c := range cases {
		for _, r := range c.r {
			c.t.print(r)
		}

		if !c.t.cur.equal(c.wantCur) {
			t.Errorf("%d: Got %q, wanted %q", i, c.t.cur, c.wantCur)
		}

		if !c.t.fb.equal(c.wantFb) {
			t.Errorf("%d: Got:\n%s\nWant:\n%s", i, c.t.fb, c.wantFb)
		}
	}
}

func TestCarriageReturn(t *testing.T) {
	cases := []struct {
		cur  cursor
		m    margin
		want cursor
	}{
		{cursor{10, 10}, margin{}, cursor{10, 0}},
		{cursor{10, 10}, newMargin(5, 15), cursor{10, 5}},
		{cursor{10, 4}, newMargin(5, 15), cursor{10, 0}},
	}

	for i, c := range cases {
		term := &Terminal{fb: newFramebuffer(24, 80), horizMargin: c.m, cur: c.cur}
		term.carriageReturn()
		if got := term.cur; !got.equal(c.want) {
			t.Errorf("%d: Got %v, wanted %v", i, got, c.want)
		}
	}
}

func TestCursorMoveTo(t *testing.T) {
	cases := []struct {
		cur  cursor
		want string
	}{
		{cursor{0, 0}, fmt.Sprintf("%c%c;%c", ESC, ESC_CSI, CSI_CUP)},
		{cursor{1, 1}, fmt.Sprintf("%c%c2;2%c", ESC, ESC_CSI, CSI_CUP)},
		{cursor{30, 0}, fmt.Sprintf("%c%c31;%c", ESC, ESC_CSI, CSI_CUP)},
		{cursor{0, 15}, fmt.Sprintf("%c%c;16%c", ESC, ESC_CSI, CSI_CUP)},
	}

	for i, c := range cases {
		if got := c.cur.getMoveToAnsi(); got != c.want {
			t.Errorf("%d: Got %q, wanted %q", i, got, c.want)
		}
	}
}

func TestTerminalDiff(t *testing.T) {
	t1 := NewTerminal(nil, 10, 10)
	t2 := NewTerminal(nil, 10, 10)
	t3 := NewTerminal(nil, 20, 15)
	t4 := t3.Copy()
	t4.fb.setCell(5, 7, newCell('a', format{fg: standardColors[FG_RED]}))
	t5 := t4.Copy()
	t5.title = "mytitle"
	t6 := t5.Copy()
	t6.icon = "mytitle"
	t7 := t4.Copy()
	t7.icon = "myicon"
	t8 := t7.Copy()
	t8.Resize(10, 6)
	t9 := t8.Copy()
	t9.privAutowrap = true
	t10 := t9.Copy()
	t10.privAutowrap = false
	t10.privNewLineMode = true
	t11 := t10.Copy()
	t11.vertMargin = newMargin(2, 5)
	t12 := t10.Copy()
	t12.horizMargin = newMargin(3, 7)
	t13 := t10.Copy()
	t13.horizMargin = newMargin(0, 4)
	t13.vertMargin = newMargin(1, 6)

	cases := []struct {
		src, dest *Terminal
		want      []byte
	}{
		{t1, t2, []byte{}},
		{t2, t3, []byte(fmt.Sprintf("%c%c%s;%d;%d%c%c%c%c%c", ESC, ESC_OSC, OSC_SETSIZE, 20, 15, ESC_ST, ESC, ESC_CSI, ';', CSI_CUP))},
		{t3, t4, []byte(fmt.Sprintf("%s%c%c%d%c%c%s", cursor{5, 7}.getMoveToAnsi(), ESC, ESC_CSI, FG_RED, CSI_SGR, 'a', cursor{}.getMoveToAnsi()))},
		{t4, t5, []byte(fmt.Sprintf("%c%c%s;%s%c", ESC, ESC_OSC, OSC_TITLE, "mytitle", ESC_ST))},
		{t4, t6, []byte(fmt.Sprintf("%c%c%s;%s%c", ESC, ESC_OSC, OSC_ICON_TITLE, "mytitle", ESC_ST))},
		{t4, t7, []byte(fmt.Sprintf("%c%c%s;%s%c", ESC, ESC_OSC, OSC_ICON, "myicon", ESC_ST))},
		{t1, t8, []byte(fmt.Sprintf("%c%c%s;%s%c%c%c%s;%d;%d%c%c%c;%c", ESC, ESC_OSC, OSC_ICON, "myicon", ESC_ST, ESC, ESC_OSC, OSC_SETSIZE, 10, 6, ESC_ST, ESC, ESC_CSI, CSI_CUP))},
		{t8, t9, []byte(fmt.Sprintf("%c%c%d%c", ESC, ESC_CSI, PRIV_CSI_DECAWM, CSI_PRIV_ENABLE))},
		{t9, t10, []byte(fmt.Sprintf("%c%c%d%c%c%c%d%c", ESC, ESC_CSI, PRIV_CSI_DECAWM, CSI_PRIV_DISABLE, ESC, ESC_CSI, PRIV_CSI_LNM, CSI_PRIV_ENABLE))},
		{t10, t11, []byte(fmt.Sprintf("%c%c%d;%d%c", ESC, ESC_CSI, 3, 6, CSI_DECSTBM))},
		{t10, t12, []byte(fmt.Sprintf("%c%c%d;%d%c", ESC, ESC_CSI, 4, 8, CSI_DECSLRM))},
		{t9, t11, []byte(fmt.Sprintf("%c%c%d;%d%c%c%c%d%c%c%c%d%c", ESC, ESC_CSI, 3, 6, CSI_DECSTBM, ESC, ESC_CSI, PRIV_CSI_DECAWM, CSI_PRIV_DISABLE, ESC, ESC_CSI, PRIV_CSI_LNM, CSI_PRIV_ENABLE))},
		{t9, t12, []byte(fmt.Sprintf("%c%c%d;%d%c%c%c%d%c%c%c%d%c", ESC, ESC_CSI, 4, 8, CSI_DECSLRM, ESC, ESC_CSI, PRIV_CSI_DECAWM, CSI_PRIV_DISABLE, ESC, ESC_CSI, PRIV_CSI_LNM, CSI_PRIV_ENABLE))},
		{t9, t13, []byte(fmt.Sprintf("%c%c%d;%d%c%c%c%d;%d%c%c%c%d%c%c%c%d%c", ESC, ESC_CSI, 1, 5, CSI_DECSLRM, ESC, ESC_CSI, 2, 7, CSI_DECSTBM, ESC, ESC_CSI, PRIV_CSI_DECAWM, CSI_PRIV_DISABLE, ESC, ESC_CSI, PRIV_CSI_LNM, CSI_PRIV_ENABLE))},
	}

	for i, c := range cases {
		if got := c.src.Diff(c.dest); !slices.Equal(got, c.want) {
			t.Errorf("%d: Got\n\t%q, wanted\n\t%q", i, string(got), string(c.want))
		}
	}
}

func TestMarginEqual(t *testing.T) {
	cases := []struct {
		m1, m2 margin
		want   bool
	}{
		{margin{}, margin{}, true},
		{newMargin(0, 1), margin{}, false}, // one set specifically
		{margin{}, newMargin(0, 1), false}, // one set specifically
		{newMargin(1, 2), newMargin(1, 2), true},
		{newMargin(1, 2), newMargin(1, 3), false},
	}

	for i, c := range cases {
		if got := c.m1.equal(c.m2); got != c.want {
			t.Errorf("%d: Got %t, wanted %t when comparing m1:%s and m2:%s", i, got, c.want, c.m1, c.m2)
		}
	}
}

func TestMarginContains(t *testing.T) {
	cases := []struct {
		m    margin
		v    int
		want bool
	}{
		{margin{}, 10, true}, // isSet == false, so everything contained
		{newMargin(5, 7), 10, false},
		{newMargin(5, 7), 6, true},
	}

	for i, c := range cases {
		if got := c.m.contains(c.v); got != c.want {
			t.Errorf("%d: Got %t, wanted %t when calling %q.contains(%d)", i, got, c.want, c.m, c.v)
		}
	}
}
