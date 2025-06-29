// Copyright (c) 2025, Ben Walton
// All rights reserved.
package vt

import (
	"fmt"
	"slices"
	"testing"
	"time"
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

func TestLineFeed(t *testing.T) {
	xc := newCell('x', defFmt.copy(), defOSC8.copy())
	defTerm := func() *Terminal {
		t := &Terminal{fb: newFramebuffer(10, 10)}
		t.fb.setCell(9, 5, xc)
		return t
	}

	vertMTerm := func() *Terminal {
		t := &Terminal{
			fb:         newFramebuffer(10, 10),
			vertMargin: newMargin(2, 5),
		}
		t.fb.setCell(5, 5, xc)
		t.fb.setCell(9, 5, xc)
		return t
	}

	horizMTerm := func() *Terminal {
		t := &Terminal{
			fb:          newFramebuffer(10, 10),
			horizMargin: newMargin(2, 5),
		}
		t.fb.setCell(5, 5, xc)
		t.fb.setCell(9, 5, xc)
		return t
	}

	boxMTerm := func() *Terminal {
		t := &Terminal{
			fb:          newFramebuffer(10, 10),
			vertMargin:  newMargin(2, 5),
			horizMargin: newMargin(2, 5),
		}
		t.fb.setCell(5, 3, xc)
		t.fb.setCell(9, 3, xc)
		return t
	}

	cases := []struct {
		t            *Terminal
		cur, wantCur cursor
	}{
		{defTerm(), cursor{0, 0}, cursor{1, 0}},
		{defTerm(), cursor{1, 1}, cursor{2, 1}},
		{defTerm(), cursor{9, 5}, cursor{9, 5}}, // should scroll
		{vertMTerm(), cursor{2, 5}, cursor{3, 5}},
		{vertMTerm(), cursor{5, 5}, cursor{5, 5}}, // should scroll
		{horizMTerm(), cursor{5, 5}, cursor{6, 5}},
		{horizMTerm(), cursor{5, 5}, cursor{6, 5}},
		{horizMTerm(), cursor{9, 5}, cursor{9, 5}}, // should scroll region
		{boxMTerm(), cursor{0, 0}, cursor{1, 0}},
		{boxMTerm(), cursor{5, 3}, cursor{5, 3}}, // should scroll region
		{boxMTerm(), cursor{9, 3}, cursor{9, 3}}, // no scroll (out marg, last row)
	}

	for i, c := range cases {
		c.t.cur = c.cur
		c.t.lineFeed()
		if c.cur.row == c.wantCur.row && c.cur.row != c.t.Rows()-1 { // we hit bottom so scrolled
			gc, _ := c.t.fb.cell(c.cur.row-1, c.cur.col)
			if !gc.equal(xc) {
				t.Errorf("%d: Invalid linefeed scroll (old line). Got %v, wanted %v", i, gc, xc)
			}
			gc, _ = c.t.fb.cell(c.cur.row, c.cur.col)
			if !gc.equal(defaultCell()) {
				t.Errorf("%d: Invalid linefeed scroll (new line). Got %v, wanted %v", i, gc, xc)
			}
		}
		if !c.t.cur.equal(c.wantCur) {
			t.Errorf("%d: Got %s, wanted %s", i, c.t.cur, c.wantCur)
		}
	}
}

func TestPrintCharsets(t *testing.T) {
	cs1 := &charset{}
	csg1 := &charset{set: 1}
	csg0s := &charset{g: [2]bool{true, false}}
	csg1s := &charset{set: 1, g: [2]bool{false, true}}

	fb1w := newFramebuffer(10, 10)
	fb1w.setCell(0, 0, newCell('a', defFmt.copy(), defOSC8.copy()))

	fb2w := newFramebuffer(10, 10)
	fb2w.setCell(0, 0, newCell('a', defFmt.copy(), defOSC8.copy()))

	fb3w := newFramebuffer(10, 10)
	fb3w.setCell(0, 0, newCell('£', defFmt.copy(), defOSC8.copy()))

	fb4w := newFramebuffer(10, 10)
	fb4w.setCell(0, 0, newCell('┼', defFmt.copy(), defOSC8.copy()))

	cases := []struct {
		cs   *charset
		r    rune
		want *framebuffer
	}{
		{cs1, 'a', fb1w},
		{csg1, 'a', fb2w},
		{csg0s, '}', fb3w},
		{csg1s, 'n', fb4w},
	}

	for i, c := range cases {
		term, _ := NewTerminal(10, 10)
		term.cs = c.cs
		term.curF = defFmt
		term.print(c.r)
		if !term.fb.equal(c.want) {
			t.Errorf("%d: Got\n%v\nWanted:\n%v\n", i, term.fb.String(), c.want.String())
		}
	}
}

func TestPrint(t *testing.T) {
	dfb := func() *framebuffer {
		return newFramebuffer(10, 10)
	}

	wfb1 := newFramebuffer(10, 10)
	wfb1.setCell(0, 0, newCell('a', defFmt.copy(), defOSC8.copy()))
	wfb1_irm := newFramebuffer(10, 10)
	wfb1_irm.setCell(0, 0, newCell('b', defFmt.copy(), defOSC8.copy()))
	wfb1_irm.setCell(0, 1, newCell('a', defFmt.copy(), defOSC8.copy()))

	wfb2 := newFramebuffer(10, 10)
	wfb2.setCell(0, 0, newCell('a', defFmt.copy(), defOSC8.copy()))
	wfb2.setCell(0, 1, newCell('b', defFmt.copy(), defOSC8.copy()))

	wfb3 := newFramebuffer(10, 10)
	wfb3.setCell(0, 0, newCell('ü', defFmt.copy(), defOSC8.copy()))
	wfb3_irm := newFramebuffer(10, 10)
	wfb3_irm.setCell(0, 0, newCell('x', defFmt.copy(), defOSC8.copy()))
	wfb3_irm.setCell(0, 1, newCell('ü', defFmt.copy(), defOSC8.copy()))

	wfb4 := newFramebuffer(10, 10)
	wfb4.setCell(0, 9, newCell('ü', defFmt.copy(), defOSC8.copy()))

	wfb5 := newFramebuffer(10, 10)
	wfb5.setCell(0, 9, newCell('z', defFmt.copy(), defOSC8.copy()))

	wfb6 := newFramebuffer(10, 10)
	wfb6.setCell(1, 0, newCell('z', defFmt.copy(), defOSC8.copy()))

	wfb7 := newFramebuffer(10, 10)
	wfb7.setCell(0, 8, fragCell('世', defFmt.copy(), defOSC8.copy(), FRAG_PRIMARY))
	wfb7.setCell(0, 9, fragCell(0, defFmt.copy(), defOSC8.copy(), FRAG_SECONDARY))

	wfb8 := newFramebuffer(10, 10)
	wfb8.setCell(1, 0, fragCell('世', defFmt.copy(), defOSC8.copy(), FRAG_PRIMARY))
	wfb8.setCell(1, 1, fragCell(0, defFmt.copy(), defOSC8.copy(), FRAG_SECONDARY))

	wfb9 := newFramebuffer(10, 10)
	wfb9.setCell(0, 5, fragCell('世', defFmt.copy(), defOSC8.copy(), FRAG_PRIMARY))
	wfb9.setCell(0, 6, fragCell(0, defFmt.copy(), defOSC8.copy(), FRAG_SECONDARY))

	ffb := newFramebuffer(10, 10)
	ffb.setCell(5, 5, fragCell('世', defFmt.copy(), defOSC8.copy(), FRAG_PRIMARY))
	ffb.setCell(5, 6, fragCell(0, defFmt.copy(), defOSC8.copy(), FRAG_SECONDARY))

	// We'll write a combining character at 5,6, which is the
	// fragmented second half of the wide cell in 5,5 (from
	// ffb). That should demonstrate overwritting a frag cell with
	// a complex case.
	wffb := newFramebuffer(10, 10)
	wffb.setCell(5, 5, defaultCell())
	wffb.setCell(5, 6, newCell('ü', defFmt.copy(), defOSC8.copy()))

	ffb2 := newFramebuffer(10, 10)
	ffb2.setCell(5, 5, fragCell('世', defFmt.copy(), defOSC8.copy(), FRAG_PRIMARY))
	ffb2.setCell(5, 6, fragCell(0, defFmt.copy(), defOSC8.copy(), FRAG_SECONDARY))

	// We'll write a combining character at 5,5, which is the
	// fragmented second half of the wide cell in 5,5 (from
	// ffb2). That should demonstrate overwritting a frag cell with
	// a complex case.
	wffb2 := newFramebuffer(10, 10)
	wffb2.setCell(5, 6, newCell('ü', defFmt.copy(), defOSC8.copy()))

	sfb := newFramebuffer(10, 10)
	sfb.setCell(8, 9, newCell('b', defFmt.copy(), defOSC8.copy()))
	sfb.setCell(9, 0, newCell('a', defFmt.copy(), defOSC8.copy()))

	wsfb := newFramebuffer(10, 10)
	wsfb.setCell(7, 9, newCell('b', defFmt.copy(), defOSC8.copy()))
	wsfb.setCell(8, 0, newCell('a', defFmt.copy(), defOSC8.copy()))
	wsfb.setCell(9, 0, newCell('ü', defFmt.copy(), defOSC8.copy()))

	wfb10 := newFramebuffer(10, 10)
	wfb10.setCell(9, 9, newCell('ü', defFmt.copy(), defOSC8.copy()))

	fb13 := dfb()
	fb13.setCell(5, 5, fragCell('世', defFmt.copy(), defOSC8.copy(), FRAG_PRIMARY))
	fb13.setCell(5, 6, fragCell(0, defFmt.copy(), defOSC8.copy(), FRAG_SECONDARY))
	wfb13 := dfb()
	wfb13.setCell(5, 5, newCell('a', defFmt.copy(), defOSC8.copy()))

	combFb := dfb()
	combFb.setCell(1, 0, newCell('a', defFmt.copy(), defOSC8.copy()))
	combFb.setCell(1, 1, newCell('b', defFmt.copy(), defOSC8.copy()))
	combFb.setCell(1, 2, newCell('u', defFmt.copy(), defOSC8.copy()))
	wfb_comb := combFb.copy()
	wfb_comb.setCell(1, 2, newCell('ü', defFmt.copy(), defOSC8.copy()))

	// wrap == CSI_MODE_{RE,}SET
	dterm := func(c cursor, fb *framebuffer, wrap, irm rune) *Terminal {
		t, _ := NewTerminal(DEF_ROWS, DEF_COLS)
		t.fb = fb
		t.cur = c
		t.setMode(DECAWM, "?", wrap)
		t.setMode(IRM, "", irm)
		return t
	}

	cases := []struct {
		t       *Terminal
		input   string
		wantCur cursor
		wantFb  *framebuffer
	}{
		{dterm(cursor{0, 0}, dfb(), CSI_MODE_RESET, CSI_MODE_RESET), "a", cursor{0, 1}, wfb1},
		{dterm(cursor{0, 0}, dfb(), CSI_MODE_RESET, CSI_MODE_SET), "ab", cursor{0, 0}, wfb1_irm},
		{dterm(cursor{0, 0}, dfb(), CSI_MODE_RESET, CSI_MODE_RESET), "ab", cursor{0, 2}, wfb2},
		{dterm(cursor{0, 0}, dfb(), CSI_MODE_RESET, CSI_MODE_RESET), "u\u0308", cursor{0, 1}, wfb3},
		{dterm(cursor{0, 0}, dfb(), CSI_MODE_RESET, CSI_MODE_SET), "u\u0308x", cursor{0, 0}, wfb3_irm},

		{dterm(cursor{0, 9}, dfb(), CSI_MODE_RESET, CSI_MODE_RESET), "u\u0308", cursor{0, 10}, wfb4},
		{dterm(cursor{0, 9}, dfb(), CSI_MODE_SET, CSI_MODE_RESET), "u\u0308", cursor{0, 10}, wfb4},
		{dterm(cursor{0, 10}, dfb(), CSI_MODE_RESET, CSI_MODE_RESET), "z", cursor{0, 10}, wfb5},
		{dterm(cursor{0, 10}, dfb(), CSI_MODE_SET, CSI_MODE_RESET), "z", cursor{1, 1}, wfb6},
		{dterm(cursor{0, 10}, dfb(), CSI_MODE_RESET, CSI_MODE_RESET), "世", cursor{0, 10}, wfb7},
		{dterm(cursor{0, 10}, dfb(), CSI_MODE_SET, CSI_MODE_RESET), "世", cursor{1, 2}, wfb8},
		{dterm(cursor{0, 5}, dfb(), CSI_MODE_SET, CSI_MODE_RESET), "世", cursor{0, 7}, wfb9},
		{dterm(cursor{5, 6}, ffb, CSI_MODE_RESET, CSI_MODE_RESET), "u\u0308", cursor{5, 7}, wffb},
		{dterm(cursor{5, 6}, ffb2, CSI_MODE_RESET, CSI_MODE_RESET), "u\u0308", cursor{5, 7}, wffb2},
		{dterm(cursor{9, 10}, sfb, CSI_MODE_SET, CSI_MODE_RESET), "u\u0308", cursor{9, 1}, wsfb},
		{dterm(cursor{9, 10}, dfb(), CSI_MODE_RESET, CSI_MODE_RESET), "u\u0308", cursor{9, 10}, wfb10},
		{dterm(cursor{5, 5}, fb13, CSI_MODE_RESET, CSI_MODE_RESET), "a", cursor{5, 6}, wfb13},
		{dterm(cursor{2, 0}, combFb, CSI_MODE_SET, CSI_MODE_RESET), "\u0308", cursor{2, 0}, wfb_comb},
	}

	for i, c := range cases {
		for _, r := range c.input {
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

func testTerminalCopy(term *Terminal) *Terminal {
	c := term.copy()
	c.lastChg = time.Now()
	return c
}

func TestTerminalDiff(t *testing.T) {
	t1, _ := NewTerminal(DEF_ROWS, DEF_COLS)
	t1.Resize(10, 10)
	t2, _ := NewTerminal(DEF_ROWS, DEF_COLS)
	t2.Resize(10, 10)
	t3, _ := NewTerminal(DEF_ROWS, DEF_COLS)
	t3.Resize(20, 15)
	t4 := testTerminalCopy(t3)
	t4.fb.setCell(5, 7, newCell('a', &format{fg: newColor(FG_RED)}, defOSC8.copy()))
	t5 := testTerminalCopy(t4)
	t5.title = "mytitle"
	t6 := testTerminalCopy(t5)
	t6.icon = "mytitle"
	t7 := testTerminalCopy(t4)
	t7.icon = "myicon"
	t8 := testTerminalCopy(t7)
	t8.Resize(10, 6)
	t9 := testTerminalCopy(t8)
	t9.setMode(REV_VIDEO, "?", CSI_MODE_SET)
	t10 := testTerminalCopy(t9)
	t10.setMode(REV_VIDEO, "?", CSI_MODE_RESET)
	t10.setMode(SHOW_CURSOR, "?", CSI_MODE_RESET)
	t11 := testTerminalCopy(t10)
	t11.vertMargin = newMargin(2, 5)
	t12 := testTerminalCopy(t10)
	t12.horizMargin = newMargin(3, 7)
	t13 := testTerminalCopy(t10)
	t13.horizMargin = newMargin(0, 4)
	t13.vertMargin = newMargin(1, 6)
	t14, _ := NewTerminal(DEF_ROWS, DEF_COLS)
	t15 := testTerminalCopy(t14)
	t15.curF = &format{fg: newColor(FG_RED), attrs: UNDERLINE}
	t16 := testTerminalCopy(t15)
	t16.curF = &format{fg: newColor(FG_YELLOW), attrs: BOLD | UNDERLINE}
	t17 := testTerminalCopy(t1)
	t17.setMode(DECOM, "?", CSI_MODE_SET) // no transport, no diff
	t17.setMode(IRM, "", CSI_MODE_SET)    // no transport, no diff
	t19, _ := NewTerminal(DEF_ROWS, DEF_COLS)
	t19.Resize(10, 10)
	t19.fb.setCell(0, 0, newCell('A', defFmt.copy(), defOSC8.copy()))
	t20 := t19.copy()
	t20.fb.setCell(0, 1, newCell('*', defFmt.copy(), defOSC8.copy()))
	t20.lastChg = time.Now()
	t21, _ := NewTerminal(DEF_ROWS, DEF_COLS)
	t22 := t21.copy()
	t22.lastChg = time.Now()
	t22.cur = cursor{10, 10}
	t23 := testTerminalCopy(t22)
	t23.keypad = PAM

	cases := []struct {
		src, dest *Terminal
		want      string
	}{
		{t1, t1, ""},
		{t1, t2, ""},
		{t2, t3, fmt.Sprintf("%c%c%c%c%c%s;%d;%d%c%c%c%s%c%c%c%c%c", ESC, CSI, CSI_SGR, ESC, OSC, OSC_SETSIZE, 20, 15, BEL, ESC, OSC, cancelHyperlink, ESC, ST, ESC, CSI, CSI_CUP)},
		{t3, t4, fmt.Sprintf("%c%c%c%s%c%c%d%c%c%c%c%s%c%c%s", ESC, CSI, CSI_SGR, cursor{5, 7}.ansiString(), ESC, CSI, FG_RED, CSI_SGR, 'a', ESC, OSC, cancelHyperlink, ESC, ST, cursor{}.ansiString())},
		{t4, t5, fmt.Sprintf("%c%c%s;%s%c", ESC, OSC, OSC_TITLE, "mytitle", BEL)},
		{t4, t6, fmt.Sprintf("%c%c%s;%s%c", ESC, OSC, OSC_ICON_TITLE, "mytitle", BEL)},
		{t4, t7, fmt.Sprintf("%c%c%s;%s%c", ESC, OSC, OSC_ICON, "myicon", BEL)},
		{t1, t8, fmt.Sprintf("%c%c%s;%s%c%c%c%c%c%c%s;%d;%d%c%c%c%s%c%c%c%c%c", ESC, OSC, OSC_ICON, "myicon", BEL, ESC, CSI, CSI_SGR, ESC, OSC, OSC_SETSIZE, 10, 6, BEL, ESC, OSC, cancelHyperlink, ESC, ST, ESC, CSI, CSI_CUP)},
		{t8, t9, fmt.Sprintf("%c%c?%d%c", ESC, CSI, REV_VIDEO, CSI_MODE_SET)},
		{t9, t10, fmt.Sprintf("%c%c?%d%c%c%c?%d%c", ESC, CSI, REV_VIDEO, CSI_MODE_RESET, ESC, CSI, SHOW_CURSOR, CSI_MODE_RESET)},
		{t10, t11, ""}, // No diff as we don't ship margins
		{t10, t12, ""}, // No diff as we don't ship margins
		{t9, t11, fmt.Sprintf("%c%c?%d%c%c%c?%d%c", ESC, CSI, REV_VIDEO, CSI_MODE_RESET, ESC, CSI, SHOW_CURSOR, CSI_MODE_RESET)},
		{t9, t12, fmt.Sprintf("%c%c?%d%c%c%c?%d%c", ESC, CSI, REV_VIDEO, CSI_MODE_RESET, ESC, CSI, SHOW_CURSOR, CSI_MODE_RESET)},
		{t9, t13, fmt.Sprintf("%c%c?%d%c%c%c?%d%c", ESC, CSI, REV_VIDEO, CSI_MODE_RESET, ESC, CSI, SHOW_CURSOR, CSI_MODE_RESET)},
		{t14, t15, fmt.Sprintf("%c%c%dm%c%c%dm", ESC, CSI, FG_RED, ESC, CSI, UNDERLINE_ON)},
		{t15, t16, fmt.Sprintf("%c%c%d%c%c%c%dm", ESC, CSI, FG_YELLOW, CSI_SGR, ESC, CSI, INTENSITY_BOLD)},
		{t1, t17, ""}, // should tranpsort is false for DECOM
		{t19, t20, fmt.Sprintf("%c%c%c%c%c;2%c*%c%c%s%c%c%c%c%c", ESC, CSI, CSI_SGR, ESC, CSI, CSI_CUP, ESC, OSC, cancelHyperlink, ESC, ST, ESC, CSI, CSI_CUP)},
		{t21, t22, fmt.Sprintf("%c%c%d;%d%c", ESC, CSI, 11, 11, CSI_CUP)},
		{t22, t23, fmt.Sprintf("%c%c", ESC, PAM)},
	}

	for i, c := range cases {
		if got := string(c.src.Diff(c.dest)); got != c.want {
			t.Errorf("%d: Got\n\t%q, wanted\n\t%q", i, got, c.want)
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
	copy(fb2erase3.data[4][0:5], newRow(5))

	cases := []struct {
		cur    cursor
		ep     int // erase param 0, 1 or 2
		fb     *framebuffer
		wantFb *framebuffer
	}{
		{
			cursor{0, 0},
			ERASE_FROM_CUR,
			emptyFb.copy(),
			emptyFb.copy(),
		},
		{
			cursor{4, 4},
			ERASE_FROM_CUR,
			fb2.copy(),
			fb2erase1,
		},
		{
			cursor{9, 4},
			ERASE_FROM_CUR,
			fb2.copy(),
			fb2erase2,
		},
		{
			cursor{4, 4},
			ERASE_TO_CUR,
			fb2.copy(),
			fb2erase3,
		},
		{
			cursor{4, 4},
			ERASE_ALL,
			fb2.copy(),
			emptyFb,
		},
		{
			cursor{9, 9},
			ERASE_ALL,
			fb2.copy(),
			emptyFb,
		},
	}

	for i, c := range cases {
		term, _ := NewTerminal(DEF_ROWS, DEF_COLS)
		term.fb = c.fb
		term.cur = c.cur
		term.eraseInDisplay(c.ep)

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

func TestIRM(t *testing.T) {
	t1, _ := NewTerminal(DEF_ROWS, DEF_COLS)
	want1 := t1.copy()
	t1.print('a')
	t1.print('b')
	t1.setMode(IRM, "", CSI_MODE_SET)
	t1.cursorMoveAbs(0, 1) // on top of the b
	t1.print('*')
	t1.setMode(IRM, "", CSI_MODE_RESET)
	want1.print('a')
	want1.print('*')
	want1.print('b')
	want1.lastChg = time.Now()

	t2, _ := NewTerminal(DEF_ROWS, DEF_COLS)
	want2 := t2.copy()
	want2.lastChg = time.Now()
	t2.cursorMoveAbs(0, 79)
	t2.print('b')
	t2.cursorMoveAbs(0, 79)
	t2.setMode(IRM, "", CSI_MODE_SET)
	t2.print('*')
	t2.setMode(IRM, "", CSI_MODE_RESET)
	want2.cursorMoveAbs(0, 79)
	want2.print('*')

	t3, _ := NewTerminal(DEF_ROWS, DEF_COLS)
	t3.setMode(DECAWM, "?", CSI_MODE_RESET)
	want3 := t3.copy()
	want3.lastChg = time.Now()
	t3.setMode(IRM, "", CSI_MODE_SET)
	t3.cursorMoveAbs(0, 78)
	t3.print('x')
	t3.print('世')
	t3.setMode(IRM, "", CSI_MODE_RESET)
	want3.cursorMoveAbs(0, 79)
	want3.print('世')
	want3.cursorMoveAbs(0, 78) // because t3 has insert on

	t4, _ := NewTerminal(DEF_ROWS, DEF_COLS)
	want4 := t4.copy()
	want4.lastChg = time.Now()
	t4.print('x')
	t4.cursorMoveAbs(0, 0)
	t4.setMode(IRM, "", CSI_MODE_SET)
	t4.print('世')
	t4.setMode(IRM, "", CSI_MODE_RESET)
	want4.print('世')
	want4.print('x')

	cases := []struct {
		t, cmp *Terminal
		want   string
	}{
		{t1, want1, "\x1b[;4H"}, // the want terminal moves the cursor as it prints
		{t2, want2, "\x1b[;81H"},
		{t3, want3, ""},         // we moved the cursor back to become equivalent
		{t4, want4, "\x1b[;4H"}, // we didn't move the cursor back
	}

	for i, c := range cases {
		if d := c.t.Diff(c.cmp); string(d) != c.want {
			t.Errorf("%d: Got %q, wanted %q.", i, d, c.want)
		}
	}
}

type prtCmd struct {
	row, col int
	chars    string
}

func mt(cmds []prtCmd, m *margin) *Terminal {
	nt, _ := NewTerminal(DEF_ROWS, DEF_COLS)
	nt.lastChg = time.Now()
	for _, c := range cmds {
		nt.cursorMoveAbs(c.row, c.col)
		for _, r := range c.chars {
			nt.print(r)
		}
	}
	if m != nil {
		nt.horizMargin = *m
	}

	return nt
}

func TestDeleteChars(t *testing.T) {
	t1 := mt([]prtCmd{
		prtCmd{0, 0, "abc"},
		prtCmd{0, 1, ""}, // on top of the b
	}, nil)

	want1 := mt([]prtCmd{
		prtCmd{0, 0, "ac"},
		prtCmd{0, 1, ""},
	}, nil)

	t4 := t1.copy()
	want4 := mt([]prtCmd{
		prtCmd{0, 0, "a"},
	}, nil)

	t5 := mt([]prtCmd{
		prtCmd{0, 0, "axy世"},
		prtCmd{0, 79, "z"},
		prtCmd{0, 1, ""}, // one top of the x
	}, nil)
	want5 := mt([]prtCmd{
		prtCmd{0, 0, "a"},
		prtCmd{0, 75, "z"},
		prtCmd{0, 1, ""}, // on top of the empty cell beside a
	}, nil)

	t6 := mt([]prtCmd{
		prtCmd{0, 0, "x世y"},
		prtCmd{0, 79, "z"},
		prtCmd{0, 1, ""}, // one top of the 世
	}, nil)
	want6 := mt([]prtCmd{
		prtCmd{0, 0, "xy"},
		prtCmd{0, 77, "z"}, // Deleting the 世 should delete
		// the frag and pull this back by
		// 2 columns, so not 78
		prtCmd{0, 1, ""}, // on top of the y
	}, nil)

	t7 := mt([]prtCmd{
		prtCmd{0, 0, "x世y世z"},
		prtCmd{0, 79, "z"},
		prtCmd{0, 1, ""}, // one top of the 世
	}, nil)
	want7 := mt([]prtCmd{
		prtCmd{0, 0, "xy世z"},
		prtCmd{0, 77, "z"}, // Deleting the 世 should delete
		// the frag and pull this back by
		// 2 columns, so not 78
		prtCmd{0, 1, ""}, // on top of the y
	}, nil)

	t8 := mt([]prtCmd{
		prtCmd{0, 0, "x世y"},
		prtCmd{0, 50, "世"},
		prtCmd{0, 79, "z"},
		prtCmd{0, 1, ""}, // one top of the 世
	}, &margin{0, 51, true})
	want8 := mt([]prtCmd{
		prtCmd{0, 0, "xy"},
		prtCmd{0, 48, "世"},
		prtCmd{0, 79, "z"},
		prtCmd{0, 1, ""}, // on top of the y
	}, &margin{0, 51, true})

	cases := []struct {
		t, cmp *Terminal
		n      int // how many characters to delete
		want   string
	}{
		{t1, want1, 1, ""}, // the want terminal moves the cursor as it prints
		{t4, want4, 2, ""},
		{t5, want5, 3, ""},
		{t6, want6, 1, ""},
		{t7, want7, 1, ""},
		{t8, want8, 1, ""},
	}

	for i, c := range cases {
		c.t.deleteChars(c.n)
		if d := string(c.t.Diff(c.cmp)); d != c.want {
			t.Errorf("%d: Got %q, wanted %q.", i, d, c.want)
		}
	}
}

func TestOSCResize(t *testing.T) {
	nt := func() *Terminal {
		x, _ := NewTerminal(DEF_ROWS, DEF_COLS)
		return x
	}
	cases := []struct {
		term        *Terminal
		rows, cols  int
		wantSuccess bool
	}{
		{nt(), 60, 201, true},
		{nt(), 24, 81, true},
		{nt(), 25, 80, true},
		{nt(), 10, 10, true},
		{nt(), -1, 201, false},
		{nt(), 60, -1, false},
		{nt(), MAX_ROWS + 1, 80, false},
		{nt(), MAX_ROWS + 1, MAX_COLS + 1, false},
	}

	for i, c := range cases {
		c.term.oscTemp = []rune(fmt.Sprintf("%s;%d;%d", OSC_SETSIZE, c.rows, c.cols))
		c.term.handleOSC(ACTION_OSC_END, BEL)
		switch c.wantSuccess {
		case true:
			if rows, cols := c.term.Rows(), c.term.Cols(); rows != c.rows || cols != c.cols {
				t.Errorf("%d: Wanted r:%d, c: %d; got r:%d, c:%d", i, c.rows, c.cols, rows, cols)
			}
		case false:
			if rows, cols := c.term.Rows(), c.term.Cols(); rows != 24 || cols != 80 {
				t.Errorf("%d: Wanted r:%d, c: %d; got r:%d, c:%d", i, c.rows, c.cols, rows, cols)
			}
		}
	}
}

func TestMakeOverlay(t *testing.T) {
	nt := func(rows, cols int) *Terminal {
		x, _ := NewTerminal(DEF_ROWS, DEF_COLS)
		x.fb.resize(rows, cols)
		return x
	}

	prefix := "\x1b7\x1b[H\x1b[30;41;1m\x1b[2K"
	suffix := "\x1b8"

	cases := []struct {
		term *Terminal
		text string
		want string
	}{
		{nt(10, 10), "foo", fmt.Sprintf("%s%c%c;%d%c%s%s", prefix, ESC, CSI, 4, CSI_CUP, "foo", suffix)},
		{nt(10, 12), "foo", fmt.Sprintf("%s%c%c;%d%c%s%s", prefix, ESC, CSI, 5, CSI_CUP, "foo", suffix)},
		{nt(24, 80), "testing", fmt.Sprintf("%s%c%c;%d%c%s%s", prefix, ESC, CSI, 37, CSI_CUP, "testing", suffix)},
	}

	for i, c := range cases {
		if got := string(c.term.MakeOverlay(c.text)); got != c.want {
			t.Errorf("%d: Got %q, wanted %q", i, got, c.want)
		}
	}
}
