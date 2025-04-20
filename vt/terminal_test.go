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
	xc := newCell('x', defFmt)
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
		{boxMTerm(), cursor{9, 3}, cursor{9, 3}}, // should scroll
	}

	for i, c := range cases {
		c.t.cur = c.cur
		c.t.lineFeed()
		if c.cur.row == c.wantCur.row {
			gc, _ := c.t.fb.getCell(c.cur.row-1, c.cur.col)
			if !gc.equal(xc) {
				t.Errorf("%d: Invalid linefeed scroll (old line). Got %v, wanted %v", i, gc, xc)
			}
			gc, _ = c.t.fb.getCell(c.cur.row, c.cur.col)
			if !gc.equal(defaultCell()) {
				t.Errorf("%d: Invalid linefeed scroll (new line). Got %v, wanted %v", i, gc, xc)
			}
		}
		if !c.t.cur.equal(c.wantCur) {
			t.Errorf("%d: Got %s, wanted %s", i, c.t.cur, c.wantCur)
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
		if wrap {
			t.setMode(DECAWM, "?", CSI_MODE_SET)
		}
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

func testTerminalCopy(term *Terminal) *Terminal {
	c := term.Copy()
	c.lastChg = time.Now()
	return c
}

func TestTerminalDiff(t *testing.T) {
	t1, _ := NewTerminal()
	t1.Resize(10, 10)
	t2, _ := NewTerminal()
	t2.Resize(10, 10)
	t3, _ := NewTerminal()
	t3.Resize(20, 15)
	t4 := testTerminalCopy(t3)
	t4.fb.setCell(5, 7, newCell('a', format{fg: newColor(FG_RED)}))
	t5 := testTerminalCopy(t4)
	t5.title = "mytitle"
	t6 := testTerminalCopy(t5)
	t6.icon = "mytitle"
	t7 := testTerminalCopy(t4)
	t7.icon = "myicon"
	t8 := testTerminalCopy(t7)
	t8.Resize(10, 6)
	t9 := testTerminalCopy(t8)
	t9.setMode(DECAWM, "?", CSI_MODE_SET)
	t10 := testTerminalCopy(t9)
	t10.setMode(DECAWM, "?", CSI_MODE_RESET)
	t10.setMode(LNM, "?", CSI_MODE_SET)
	t11 := testTerminalCopy(t10)
	t11.vertMargin = newMargin(2, 5)
	t12 := testTerminalCopy(t10)
	t12.horizMargin = newMargin(3, 7)
	t13 := testTerminalCopy(t10)
	t13.horizMargin = newMargin(0, 4)
	t13.vertMargin = newMargin(1, 6)
	t14, _ := NewTerminal()
	t15 := testTerminalCopy(t14)
	t15.curF = format{fg: newColor(FG_RED), attrs: UNDERLINE}
	t16 := testTerminalCopy(t15)
	t16.curF = format{fg: newColor(FG_YELLOW), attrs: BOLD | UNDERLINE}
	t17 := testTerminalCopy(t1)
	t17.setMode(BRACKET_PASTE, "?", CSI_MODE_SET)
	t17.setMode(IRM, "", CSI_MODE_SET)
	t18 := testTerminalCopy(t17)
	t18.setMode(BRACKET_PASTE, "?", CSI_MODE_RESET)
	t18.setMode(DECCKM, "?", CSI_MODE_SET)
	t18.setMode(IRM, "", CSI_MODE_RESET)
	t19, _ := NewTerminal()
	t19.Resize(10, 10)
	t19.fb.setCell(0, 0, newCell('A', defFmt))
	t20 := t19.Copy()
	t20.fb.setCell(0, 1, newCell('*', defFmt))
	t20.lastChg = time.Now()
	t21, _ := NewTerminal()
	t22 := t21.Copy()
	t22.lastChg = time.Now()
	t22.cur = cursor{10, 10}

	cases := []struct {
		src, dest *Terminal
		want      []byte
	}{
		{t1, t1, []byte{}},
		{t1, t2, []byte{}},
		{t2, t3, []byte(fmt.Sprintf("%c%c%c%c%c%s;%d;%d%c%c%c%c", ESC, CSI, CSI_SGR, ESC, OSC, OSC_SETSIZE, 20, 15, BEL, ESC, CSI, CSI_CUP))},
		{t3, t4, []byte(fmt.Sprintf("%c%c%c%s%c%c%d%c%c%s", ESC, CSI, CSI_SGR, cursor{5, 7}.getMoveToAnsi(), ESC, CSI, FG_RED, CSI_SGR, 'a', cursor{}.getMoveToAnsi()))},
		{t4, t5, []byte(fmt.Sprintf("%c%c%s;%s%c", ESC, OSC, OSC_TITLE, "mytitle", BEL))},
		{t4, t6, []byte(fmt.Sprintf("%c%c%s;%s%c", ESC, OSC, OSC_ICON_TITLE, "mytitle", BEL))},
		{t4, t7, []byte(fmt.Sprintf("%c%c%s;%s%c", ESC, OSC, OSC_ICON, "myicon", BEL))},
		{t1, t8, []byte(fmt.Sprintf("%c%c%s;%s%c%c%c%c%c%c%s;%d;%d%c%c%c%c", ESC, OSC, OSC_ICON, "myicon", BEL, ESC, CSI, CSI_SGR, ESC, OSC, OSC_SETSIZE, 10, 6, BEL, ESC, CSI, CSI_CUP))},
		{t8, t9, []byte(fmt.Sprintf("%c%c?%d%c", ESC, CSI, DECAWM, CSI_MODE_SET))},
		{t9, t10, []byte(fmt.Sprintf("%c%c?%d%c%c%c?%d%c", ESC, CSI, DECAWM, CSI_MODE_RESET, ESC, CSI, LNM, CSI_MODE_SET))},
		{t10, t11, []byte(fmt.Sprintf("%c%c%d;%d%c", ESC, CSI, 3, 6, CSI_DECSTBM))},
		{t10, t12, []byte(fmt.Sprintf("%c%c%d;%d%c", ESC, CSI, 4, 8, CSI_DECSLRM))},
		{t9, t11, []byte(fmt.Sprintf("%c%c%d;%d%c%c%c?%d%c%c%c?%d%c", ESC, CSI, 3, 6, CSI_DECSTBM, ESC, CSI, DECAWM, CSI_MODE_RESET, ESC, CSI, LNM, CSI_MODE_SET))},
		{t9, t12, []byte(fmt.Sprintf("%c%c%d;%d%c%c%c?%d%c%c%c?%d%c", ESC, CSI, 4, 8, CSI_DECSLRM, ESC, CSI, DECAWM, CSI_MODE_RESET, ESC, CSI, LNM, CSI_MODE_SET))},
		{t9, t13, []byte(fmt.Sprintf("%c%c%d;%d%c%c%c%d;%d%c%c%c?%d%c%c%c?%d%c", ESC, CSI, 1, 5, CSI_DECSLRM, ESC, CSI, 2, 7, CSI_DECSTBM, ESC, CSI, DECAWM, CSI_MODE_RESET, ESC, CSI, LNM, CSI_MODE_SET))},
		{t14, t15, []byte(fmt.Sprintf("%c%c%dm%c%c%dm", ESC, CSI, FG_RED, ESC, CSI, UNDERLINE_ON))},

		{t15, t16, []byte(fmt.Sprintf("%c%c%d%c%c%c%dm", ESC, CSI, FG_YELLOW, CSI_SGR, ESC, CSI, INTENSITY_BOLD))},
		{t1, t17, []byte(fmt.Sprintf("%c%c?%d%c%c%c%d%c", ESC, CSI, BRACKET_PASTE, CSI_MODE_SET, ESC, CSI, IRM, CSI_MODE_SET))},
		{t17, t18, []byte(fmt.Sprintf("%c%c?%d%c%c%c?%d%c%c%c%d%c", ESC, CSI, BRACKET_PASTE, CSI_MODE_RESET, ESC, CSI, DECCKM, CSI_MODE_SET, ESC, CSI, IRM, CSI_MODE_RESET))},
		{t19, t20, []byte(fmt.Sprintf("%c%c%c%c%c;2%c*%c%c%c", ESC, CSI, CSI_SGR, ESC, CSI, CSI_CUP, ESC, CSI, CSI_CUP))},
		{t21, t22, []byte(fmt.Sprintf("%c%c%d;%d%c", ESC, CSI, 11, 11, CSI_CUP))},
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
	copy(fb2erase3.data[4][0:5], newRow(5))

	cases := []struct {
		cur    cursor
		params *parameters
		fb     *framebuffer
		wantFb *framebuffer
	}{
		{
			cursor{0, 0},
			&parameters{1, []int{0}}, // pos to end of screen
			emptyFb.copy(),
			emptyFb.copy(),
		},
		{
			cursor{4, 4},
			&parameters{1, []int{0}}, // pos to end of screen
			fb2.copy(),
			fb2erase1,
		},
		{
			cursor{9, 4},
			&parameters{1, []int{0}}, // pos to end of screen
			fb2.copy(),
			fb2erase2,
		},
		{
			cursor{4, 4},
			&parameters{1, []int{1}}, // pos to beginning of screen
			fb2.copy(),
			fb2erase3,
		},
		{
			cursor{4, 4},
			&parameters{1, []int{2}}, // whole screen
			fb2.copy(),
			emptyFb,
		},
		{
			cursor{9, 9},
			&parameters{1, []int{2}}, // whole screen
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

func TestIRM(t *testing.T) {
	t1, _ := NewTerminal()
	want1 := t1.Copy()
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

	t2, _ := NewTerminal()
	want2 := t2.Copy()
	want2.lastChg = time.Now()
	t2.cursorMoveAbs(0, 79)
	t2.print('b')
	t2.cursorMoveAbs(0, 79)
	t2.setMode(IRM, "", CSI_MODE_SET)
	t2.print('*')
	t2.setMode(IRM, "", CSI_MODE_RESET)
	want2.cursorMoveAbs(0, 79)
	want2.print('*')

	t3, _ := NewTerminal()
	want3 := t3.Copy()
	want3.lastChg = time.Now()
	t3.cursorMoveAbs(0, 79)
	t3.print('x')
	t3.cursorMoveAbs(0, 79)
	t3.setMode(IRM, "", CSI_MODE_SET)
	t3.print('世')
	t3.setMode(IRM, "", CSI_MODE_RESET)
	want3.cursorMoveAbs(0, 79)
	want3.print('世')

	t4, _ := NewTerminal()
	want4 := t4.Copy()
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

func TestDeleteChars(t *testing.T) {
	t1, _ := NewTerminal()
	want1 := t1.Copy()
	t1.print('a')
	t1.print('b')
	t1.print('c')
	t1.cursorMoveAbs(0, 1) // on top of the b
	want1.print('a')
	want1.print('c')
	want1.lastChg = time.Now()
	p1 := &parameters{num: 1, items: []int{1}}

	cases := []struct {
		t, cmp *Terminal
		n      int // number of chars to delete
		want   string
	}{
		{t1, want1, 0, "\x1b[;3H"}, // the want terminal moves the cursor as it prints
	}

	for i, c := range cases {
		c.t.deleteChars(p1, []rune{})
		if d := string(c.t.Diff(c.cmp)); d != c.want {
			t.Errorf("%d: Got %q, wanted %q.", i, d, c.want)
		}
	}
}
