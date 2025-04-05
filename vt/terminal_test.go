package vt

import (
	"fmt"
	"slices"
	"testing"
)

func TestCursorInScrollingRegion(t *testing.T) {
	cases := []struct {
		t    *Terminal
		want bool
	}{
		{&Terminal{}, false},
		{&Terminal{horizMargin: newMargin(0, 10)}, false},
		{&Terminal{vertMargin: newMargin(0, 10)}, false},
		{&Terminal{vertMargin: newMargin(5, 10), horizMargin: newMargin(5, 10)}, false},
		{&Terminal{cur: cursor{5, 5}, vertMargin: newMargin(5, 10), horizMargin: newMargin(5, 10)}, true},
		{&Terminal{cur: cursor{10, 10}, vertMargin: newMargin(5, 10), horizMargin: newMargin(5, 10)}, true},
		{&Terminal{cur: cursor{11, 11}, vertMargin: newMargin(5, 10), horizMargin: newMargin(5, 10)}, false},
	}

	for i, c := range cases {
		if got := c.t.cursorInScrollingRegion(); got != c.want {
			t.Errorf("%d: Got %t, wanted %t for %v", i, got, c.want, c.t)
		}
	}
}

func TestCursorMove(t *testing.T) {
	fb1 := newFramebuffer(24, 80)

	cases := []struct {
		t                *Terminal
		params           *parameters
		mt               rune // move type
		wantRow, wantCol int
	}{
		// HPA - horizontal position absolute
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, paramsFromInts([]int{}), CSI_HPA, 15, 0},
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, paramsFromInts([]int{10}), CSI_HPA, 15, 9},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, paramsFromInts([]int{3}), CSI_HPA, 15, 2},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, paramsFromInts([]int{1000}), CSI_HPA, 15, fb1.getNumCols() - 1},
		// HPR - horizontal position relative
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, paramsFromInts([]int{}), CSI_HPR, 15, fb1.getNumCols() - 1},
		{&Terminal{fb: fb1, cur: cursor{15, 5}}, paramsFromInts([]int{}), CSI_HPR, 15, 6},
		{&Terminal{fb: fb1, cur: cursor{15, 0}}, paramsFromInts([]int{}), CSI_HPR, 15, 1},
		{&Terminal{fb: fb1, cur: cursor{15, 5}}, paramsFromInts([]int{10}), CSI_HPR, 15, 15},
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
		{&Terminal{fb: fb1, cur: cursor{fb1.getNumRows() - 1, 1}}, paramsFromInts([]int{}), CSI_CUD, fb1.getNumRows() - 1, 1},
		{&Terminal{fb: fb1, cur: cursor{fb1.getNumRows() - 1, 1}}, paramsFromInts([]int{3}), CSI_CUD, fb1.getNumRows() - 1, 1},
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
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, paramsFromInts([]int{}), CSI_CUF, 15, fb1.getNumCols() - 1},
		{&Terminal{fb: fb1, cur: cursor{15, fb1.getNumCols() - 1}}, paramsFromInts([]int{10}), CSI_CUF, 15, fb1.getNumCols() - 1},
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
		{&Terminal{fb: fb1, cur: cursor{fb1.getNumRows() - 1, 0}}, paramsFromInts([]int{}), CSI_CNL, fb1.getNumRows() - 1, 0},
		{&Terminal{fb: fb1, cur: cursor{fb1.getNumRows() - 1, 0}}, paramsFromInts([]int{2}), CSI_CNL, fb1.getNumRows() - 1, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{}), CSI_CNL, 1, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{10}), CSI_CNL, 10, 0},
		// CHA - cursor horizontal absolute
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{}), CSI_CHA, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{10}), CSI_CHA, 0, 9},  // 0 vs 1 based
		{&Terminal{fb: fb1, cur: cursor{0, 10}}, paramsFromInts([]int{10}), CSI_CHA, 0, 9}, // 0 vs 1 based
		{&Terminal{fb: fb1, cur: cursor{0, 10}}, paramsFromInts([]int{fb1.getNumCols() + 10}), CSI_CHA, 0, fb1.getNumCols() - 1},
		// CUP - cursor position
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{}), CSI_CUP, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, paramsFromInts([]int{}), CSI_CUP, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{}), CSI_CUP, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, paramsFromInts([]int{0, 16}), CSI_CUP, 0, 15},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, paramsFromInts([]int{16}), CSI_CUP, 15, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, paramsFromInts([]int{1000, 1000}), CSI_CUP, fb1.getNumRows() - 1, fb1.getNumCols() - 1},
		// HVP - horizontal vertical position
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{}), CSI_HVP, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, paramsFromInts([]int{}), CSI_HVP, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{0, 0}}, paramsFromInts([]int{}), CSI_HVP, 0, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, paramsFromInts([]int{0, 16}), CSI_HVP, 0, 15},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, paramsFromInts([]int{16}), CSI_HVP, 15, 0},
		{&Terminal{fb: fb1, cur: cursor{10, 25}}, paramsFromInts([]int{1000, 1000}), CSI_HVP, fb1.getNumRows() - 1, fb1.getNumCols() - 1},
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
		t, _ := NewTerminal()
		t.fb = fb
		t.cur = c
		t.setFlag(PRIV_CSI_DECAWM, wrap)
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

func TestCursorMoveToAnsi(t *testing.T) {
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
	t1, _ := NewTerminal()
	t1.Resize(10, 10)
	t2, _ := NewTerminal()
	t2.Resize(10, 10)
	t3, _ := NewTerminal()
	t3.Resize(20, 15)
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
	t9.setFlag(PRIV_CSI_DECAWM, true)
	t10 := t9.Copy()
	t10.setFlag(PRIV_CSI_DECAWM, false)
	t10.setFlag(PRIV_CSI_LNM, true)
	t11 := t10.Copy()
	t11.vertMargin = newMargin(2, 5)
	t12 := t10.Copy()
	t12.horizMargin = newMargin(3, 7)
	t13 := t10.Copy()
	t13.horizMargin = newMargin(0, 4)
	t13.vertMargin = newMargin(1, 6)
	t14, _ := NewTerminal()
	t15 := t14.Copy()
	t15.curF = format{fg: standardColors[FG_RED], italic: true}
	t16 := t15.Copy()
	t16.curF = format{fg: standardColors[FG_YELLOW], italic: true, brightness: FONT_BOLD}
	t17 := t1.Copy()
	t17.setFlag(PRIV_CSI_BRACKET_PASTE, true)
	t18 := t17.Copy()
	t18.setFlag(PRIV_CSI_BRACKET_PASTE, false)
	t18.setFlag(PRIV_CSI_DECCKM, true)

	cases := []struct {
		src, dest *Terminal
		want      []byte
	}{
		{t1, t2, []byte{}},
		{t2, t3, []byte(fmt.Sprintf("%c%c%c%c%c%s;%d;%d%c%c%c%c%c", ESC, ESC_CSI, CSI_SGR, ESC, ESC_OSC, OSC_SETSIZE, 20, 15, CTRL_BEL, ESC, ESC_CSI, ';', CSI_CUP))},
		{t3, t4, []byte(fmt.Sprintf("%c%c%c%s%c%c%d%c%c%s", ESC, ESC_CSI, CSI_SGR, cursor{5, 7}.getMoveToAnsi(), ESC, ESC_CSI, FG_RED, CSI_SGR, 'a', cursor{}.getMoveToAnsi()))},
		{t4, t5, []byte(fmt.Sprintf("%c%c%s;%s%c", ESC, ESC_OSC, OSC_TITLE, "mytitle", CTRL_BEL))},
		{t4, t6, []byte(fmt.Sprintf("%c%c%s;%s%c", ESC, ESC_OSC, OSC_ICON_TITLE, "mytitle", CTRL_BEL))},
		{t4, t7, []byte(fmt.Sprintf("%c%c%s;%s%c", ESC, ESC_OSC, OSC_ICON, "myicon", CTRL_BEL))},
		{t1, t8, []byte(fmt.Sprintf("%c%c%s;%s%c%c%c%c%c%c%s;%d;%d%c%c%c;%c", ESC, ESC_OSC, OSC_ICON, "myicon", CTRL_BEL, ESC, ESC_CSI, CSI_SGR, ESC, ESC_OSC, OSC_SETSIZE, 10, 6, CTRL_BEL, ESC, ESC_CSI, CSI_CUP))},
		{t8, t9, []byte(fmt.Sprintf("%c%c%d%c", ESC, ESC_CSI, PRIV_CSI_DECAWM, CSI_PRIV_ENABLE))},
		{t9, t10, []byte(fmt.Sprintf("%c%c%d%c%c%c%d%c", ESC, ESC_CSI, PRIV_CSI_DECAWM, CSI_PRIV_DISABLE, ESC, ESC_CSI, PRIV_CSI_LNM, CSI_PRIV_ENABLE))},
		{t10, t11, []byte(fmt.Sprintf("%c%c%d;%d%c", ESC, ESC_CSI, 3, 6, CSI_DECSTBM))},
		{t10, t12, []byte(fmt.Sprintf("%c%c%d;%d%c", ESC, ESC_CSI, 4, 8, CSI_DECSLRM))},
		{t9, t11, []byte(fmt.Sprintf("%c%c%d;%d%c%c%c%d%c%c%c%d%c", ESC, ESC_CSI, 3, 6, CSI_DECSTBM, ESC, ESC_CSI, PRIV_CSI_DECAWM, CSI_PRIV_DISABLE, ESC, ESC_CSI, PRIV_CSI_LNM, CSI_PRIV_ENABLE))},
		{t9, t12, []byte(fmt.Sprintf("%c%c%d;%d%c%c%c%d%c%c%c%d%c", ESC, ESC_CSI, 4, 8, CSI_DECSLRM, ESC, ESC_CSI, PRIV_CSI_DECAWM, CSI_PRIV_DISABLE, ESC, ESC_CSI, PRIV_CSI_LNM, CSI_PRIV_ENABLE))},
		{t9, t13, []byte(fmt.Sprintf("%c%c%d;%d%c%c%c%d;%d%c%c%c%d%c%c%c%d%c", ESC, ESC_CSI, 1, 5, CSI_DECSLRM, ESC, ESC_CSI, 2, 7, CSI_DECSTBM, ESC, ESC_CSI, PRIV_CSI_DECAWM, CSI_PRIV_DISABLE, ESC, ESC_CSI, PRIV_CSI_LNM, CSI_PRIV_ENABLE))},
		{t14, t15, []byte(fmt.Sprintf("%c%c%dm%c%c%dm", ESC, ESC_CSI, FG_RED, ESC, ESC_CSI, ITALIC_ON))},

		{t15, t16, []byte(fmt.Sprintf("%c%c%d%c%c%c%dm", ESC, ESC_CSI, FG_YELLOW, CSI_SGR, ESC, ESC_CSI, FONT_BOLD))},
		{t1, t17, []byte(fmt.Sprintf("%c%c%d%c", ESC, ESC_CSI, PRIV_CSI_BRACKET_PASTE, CSI_PRIV_ENABLE))},
		{t17, t18, []byte(fmt.Sprintf("%c%c%d%c%c%c%d%c", ESC, ESC_CSI, PRIV_CSI_DECCKM, CSI_PRIV_ENABLE, ESC, ESC_CSI, PRIV_CSI_BRACKET_PASTE, CSI_PRIV_DISABLE))},
	}

	for i, c := range cases {
		if got := c.src.Diff(c.dest); !slices.Equal(got, c.want) {
			t.Errorf("%d: Got\n\t%q, wanted\n\t%q", i, string(got), string(c.want))
		}
	}
}

func TestEraseInDisplay(t *testing.T) {
	emptyFb := newFramebuffer(10, 10)
	fb2 := emptyFb.copy()
	fillBuffer(fb2)
	fb2erase1 := fb2.copy()
	copy(fb2erase1.data[4][4:], newRow(6))
	fb2erase1.data[5] = newRow(10)
	fb2erase1.data[6] = newRow(10)
	fb2erase1.data[7] = newRow(10)
	fb2erase1.data[8] = newRow(10)
	fb2erase1.data[9] = newRow(10)
	fb2erase2 := fb2.copy()
	copy(fb2erase2.data[9][4:], newRow(6))
	fb2erase3 := fb2.copy()
	fb2erase3.data[0] = newRow(10)
	fb2erase3.data[1] = newRow(10)
	fb2erase3.data[2] = newRow(10)
	fb2erase3.data[3] = newRow(10)
	copy(fb2erase3.data[4][0:4], newRow(4))

	cases := []struct {
		cur    cursor
		params *parameters
		fb     *framebuffer
		wantFb *framebuffer
	}{
		{
			cursor{0, 0},
			&parameters{1, []int{0}},
			emptyFb.copy(),
			emptyFb.copy(),
		},
		{
			cursor{4, 4},
			&parameters{1, []int{0}},
			fb2.copy(),
			fb2erase1,
		},
		{
			cursor{9, 4},
			&parameters{1, []int{0}},
			fb2.copy(),
			fb2erase2,
		},
		{
			cursor{4, 4},
			&parameters{1, []int{1}},
			fb2.copy(),
			fb2erase3,
		},
		{
			cursor{4, 4},
			&parameters{1, []int{2}},
			fb2.copy(),
			emptyFb,
		},
		{
			cursor{9, 9},
			&parameters{1, []int{2}},
			fb2.copy(),
			emptyFb,
		},
	}

	for i, c := range cases {
		term, _ := NewTerminal()
		term.fb = c.fb
		term.cur = c.cur
		term.eraseInDisplay(c.params)

		if !c.wantFb.equal(term.fb) {
			t.Errorf("%d: Got\n%v wanted\n%v", i, term.fb, c.wantFb)
		}
	}
}

func TestResizeTabs(t *testing.T) {
	cases := []struct {
		tabs []bool
		sz   int
		want []bool
	}{
		// Same size, default tab stops
		{
			[]bool{true, false, false, false, false, false, false, false, false, true},
			10,
			[]bool{true, false, false, false, false, false, false, false, false, true},
		},
		// Same size, preserved modifications
		{
			[]bool{true, false, false, true, false, true, false, false, false, true},
			10,
			[]bool{true, false, false, true, false, true, false, false, false, true},
		},
		// Larger, default tab stops
		{
			[]bool{true, false, false, false, false, false, false, false, false, true},
			17,
			[]bool{true, false, false, false, false, false, false, false, false, true, false, false, false, false, false, false, true},
		},
		// Smaller, default tab stops
		{
			[]bool{true, false, false, false, false, false, false, false, false, true},
			12,
			[]bool{true, false, false, false, false, false, false, false, false, true, false, false},
		},
		// Smaller, preserved modifications
		{
			[]bool{true, false, false, true, false, false, false, false, false, true},
			7,
			[]bool{true, false, false, true, false, false},
		},
		// Larger, preserved modifications
		{
			[]bool{true, false, false, true, false, false, false, true, false, true},
			17,
			[]bool{true, false, false, true, false, false, false, true, false, true, false, false, false, false, false, false, true},
		},
	}

	for i, c := range cases {
		term := &Terminal{tabs: c.tabs}
		term.resizeTabs(c.sz)
		if !slices.Equal(term.tabs, c.want) {
			t.Errorf("%d: Got\n\t%v, wanted\n\t%v", i, term.tabs, c.want)
		}
	}
}

func TestClearTabs(t *testing.T) {
	cases := []struct {
		tabs     []bool
		cur      cursor
		tbc_mode int
		want     []bool
	}{
		{
			[]bool{true, false, false, true, false},
			cursor{5, 2},
			TBC_CUR,
			[]bool{true, false, false, true, false},
		},
		{
			[]bool{true, false, false, true, false},
			cursor{5, 3},
			TBC_CUR,
			[]bool{true, false, false, false, false},
		},
		{
			[]bool{true, false, false, true, false},
			cursor{5, 3},
			TBC_ALL,
			[]bool{false, false, false, false, false},
		},
	}

	for i, c := range cases {
		term := &Terminal{tabs: c.tabs, cur: c.cur}
		params := &parameters{1, []int{c.tbc_mode}}
		term.clearTabs(params)
		if !slices.Equal(term.tabs, c.want) {
			t.Errorf("%d: Got\n\t%v, wanted\n\t%v", i, term.tabs, c.want)
		}
	}
}

func TestStepTabs(t *testing.T) {
	cases := []struct {
		tabs  []bool
		cur   cursor
		steps int
		want  cursor
	}{
		{
			[]bool{true, false, false, false, true, false, false, true, false},
			cursor{5, 3},
			1,
			cursor{5, 4},
		},
		{
			[]bool{true, false, false, false, true, false, false, true, false},
			cursor{5, 8},
			1,
			cursor{5, 8},
		},
		{
			[]bool{true, false, false, false, true, false, false, true, false},
			cursor{5, 3},
			2,
			cursor{5, 7},
		},
		{
			[]bool{true, false, false, false, true, false, false, true, false},
			cursor{5, 3},
			3,
			cursor{5, 8},
		},
		{
			[]bool{true, true, false, false, true, false, false, true, false},
			cursor{5, 3},
			-1,
			cursor{5, 1},
		},
		{
			[]bool{true, false, true, false, true, false, false, true, false},
			cursor{5, 7},
			-2,
			cursor{5, 2},
		},
		{
			[]bool{true, false, true, false, true, false, false, true, false},
			cursor{5, 3},
			-3,
			cursor{5, 0},
		},
		{
			[]bool{true, false, true, false, true, false, false, true, false},
			cursor{5, 3},
			0,
			cursor{5, 3},
		},
		{
			[]bool{true, false, true, false, true, false, false, true, false},
			cursor{5, 0},
			-1,
			cursor{5, 0},
		},
		{
			[]bool{true, false, true, false, true, false, false, true, false},
			cursor{5, 8},
			1,
			cursor{5, 8},
		},
	}

	for i, c := range cases {
		term := &Terminal{tabs: c.tabs, cur: c.cur, fb: newFramebuffer(10, len(c.tabs))}
		term.stepTabs(c.steps)
		if !term.cur.equal(c.want) {
			t.Errorf("%d: Got %s, wanted %s", i, term.cur, c.want)
		}
	}
}
