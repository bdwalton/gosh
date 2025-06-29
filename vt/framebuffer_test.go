// Copyright (c) 2025, Ben Walton
// All rights reserved.
package vt

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"
)

var nonDefFmt = &format{
	fg:    newColor(FG_YELLOW),
	bg:    newColor(BG_BLUE),
	attrs: UNDERLINE,
}

func fillBuffer(fb *framebuffer) *framebuffer {
	for row := 0; row < fb.rows(); row++ {
		for col := 0; col < fb.cols(); col++ {
			fb.setCell(row, col, newCell('a'+rune(rand.Intn(26)), nonDefFmt, defOSC8))
		}
	}

	return fb
}

func TestCellDiff(t *testing.T) {
	cases := []struct {
		src, dest *cell
		want      string
	}{
		{
			defaultCell(),
			defaultCell(),
			"",
		},
		{
			defaultCell(),
			fragCell('世', defFmt.copy(), defOSC8.copy(), FRAG_SECONDARY),
			"",
		},
		{
			newCell('a', defFmt.copy(), defOSC8.copy()),
			newCell(' ', defFmt.copy(), defOSC8.copy()),
			" ",
		},
		{
			newCell('b', &format{fg: newColor(FG_BLUE)}, defOSC8.copy()),
			newCell(' ', defFmt.copy(), defOSC8.copy()),
			"\x1b[m ",
		},
		{
			newCell('b', &format{attrs: UNDERLINE}, defOSC8.copy()),
			newCell('b', defFmt.copy(), defOSC8.copy()),
			"\x1b[mb",
		},
		{
			newCell('b', defFmt.copy(), defOSC8.copy()),
			newCell('b', &format{attrs: UNDERLINE}, defOSC8.copy()),
			fmt.Sprintf("%c%c%d%c%c", ESC, CSI, UNDERLINE_ON, CSI_SGR, 'b'),
		},
		{
			newCell('a', defFmt.copy(), defOSC8.copy()),
			newCell('a', defFmt.copy(), newHyperlink("8;id=0;http://foo.com")),
			fmt.Sprintf("%c%c%s%c%c%c", ESC, OSC, "8;id=0;http://foo.com", ESC, ST, 'a'),
		},
		{
			newCell('a', defFmt.copy(), newHyperlink("8;id=0;http://foo.com")),
			newCell('a', defFmt.copy(), defOSC8.copy()),
			fmt.Sprintf("%c%c%s%c%c%c", ESC, OSC, "8;;", ESC, ST, 'a'),
		},
	}

	for i, c := range cases {
		if got := string(c.src.diff(c.dest)); got != c.want {
			t.Errorf("\n%d: Got: %q\nWanted: %q", i, got, c.want)
		}
	}
}

func TestCellEfficientDiff(t *testing.T) {
	cases := []struct {
		src, dest *cell
		f         *format
		hl        *osc8
		want      string
	}{
		{
			defaultCell(),
			defaultCell(),
			defFmt.copy(),
			defOSC8.copy(),
			"",
		},
		{
			newCell('a', &format{fg: newColor(FG_RED)}, defOSC8.copy()),
			newCell('a', &format{fg: newColor(FG_RED)}, defOSC8.copy()),
			&format{fg: newColor(FG_RED)},
			defOSC8.copy(),
			"",
		},
		{
			newCell('a', &format{fg: newColor(FG_RED)}, defOSC8.copy()),
			newCell('a', &format{fg: newColor(FG_RED)}, defOSC8.copy()),
			defFmt.copy(),
			defOSC8.copy(),
			fmt.Sprintf("%c%c%d%c%c", ESC, CSI, FG_RED, CSI_SGR, 'a'),
		},
		{
			newCell('a', &format{fg: newColor(FG_RED)}, defOSC8.copy()),
			newCell('a', &format{bg: newColor(BG_RED)}, defOSC8.copy()),
			defFmt.copy(),
			defOSC8.copy(),
			fmt.Sprintf("%c%c%d%c%c", ESC, CSI, BG_RED, CSI_SGR, 'a'),
		},
		{
			newCell('a', defFmt.copy(), newHyperlink("8;;file:///foo/bar")),
			newCell('a', defFmt.copy(), newHyperlink("8;;file:///foo/bar")),
			defFmt.copy(),
			defOSC8.copy(),
			fmt.Sprintf("%c%c%s%c%c%c", ESC, OSC, "8;;file:///foo/bar", ESC, ST, 'a'),
		},
		{
			newCell('a', defFmt.copy(), defOSC8.copy()),
			newCell('a', defFmt.copy(), defOSC8.copy()),
			defFmt.copy(),
			newHyperlink("8;;file:///foo/bar"),

			fmt.Sprintf("%c%c%s%c%c%c", ESC, OSC, "8;;", ESC, ST, 'a'),
		},
	}

	for i, c := range cases {
		if got := string(c.src.efficientDiff(c.dest, c.f, c.hl)); got != c.want {
			t.Errorf("\n%d: Got: %v %q\nWanted: %v %q", i, got, string(got), c.want, string(c.want))
		}
	}
}

func TestCellEquality(t *testing.T) {
	cases := []struct {
		c1, c2 *cell
		want   bool
	}{
		{defaultCell(), defaultCell(), true},
		{fragCell('r', defFmt.copy(), defOSC8.copy(), 1), defaultCell(), false},
		{fragCell('r', defFmt.copy(), defOSC8.copy(), 1), fragCell('r', defFmt.copy(), defOSC8.copy(), 2), false},
		{newCell('r', defFmt.copy(), defOSC8.copy()), newCell('r', defFmt.copy(), defOSC8.copy()), true},
		{defaultCell(), fragCell('r', defFmt.copy(), defOSC8.copy(), 2), false},
		{newCell('a', defFmt.copy(), defOSC8.copy()), newCell('a', defFmt.copy(), defOSC8.copy()), true},
		{newCell('a', &format{attrs: UNDERLINE}, defOSC8.copy()), newCell('a', &format{attrs: UNDERLINE}, defOSC8.copy()), true},
		{defaultCell(), newCell('a', defFmt.copy(), defOSC8.copy()), false},
		{fragCell('a', defFmt.copy(), defOSC8.copy(), 1), newCell('a', defFmt.copy(), defOSC8.copy()), false},
		{newCell('a', &format{attrs: UNDERLINE}, defOSC8.copy()), newCell('a', defFmt.copy(), defOSC8.copy()), false},
		{newCell('a', defFmt.copy(), newHyperlink("8;;http://foo.com")), newCell('a', defFmt.copy(), newHyperlink("8;;http://foo.com")), true},
		{newCell('a', defFmt.copy(), newHyperlink("8;;http://foo.com")), newCell('a', defFmt.copy(), newHyperlink("8;;http://bar.com")), false},
		{newCell('a', defFmt.copy(), newHyperlink("8;;")), newCell('a', defFmt.copy(), newHyperlink("8;;http://bar.com")), false},
		{newCell('a', defFmt.copy(), newHyperlink("8;;")), newCell('a', defFmt.copy(), defOSC8.copy()), true},
	}

	for i, c := range cases {
		if got := c.c1.equal(c.c2); got != c.want {
			t.Errorf("%d: Got %t, wanted %t when comparing %v.equal(%v)", i, got, c.want, c.c1, c.c2)
		}
	}
}

func TestSetCells(t *testing.T) {
	cases := []struct {
		fb         *framebuffer
		t, b, l, r int
		fm         *format
	}{
		{fillBuffer(newFramebuffer(10, 10)), 0, 0, 0, 5, defFmt.copy()},
		{fillBuffer(newFramebuffer(10, 10)), 0, 0, 5, 9, defFmt.copy()},
		{fillBuffer(newFramebuffer(10, 10)), -1, -1, 5, 9, defFmt.copy()},
		{fillBuffer(newFramebuffer(10, 10)), 10, 10, 5, 9, defFmt.copy()},
		{fillBuffer(newFramebuffer(10, 10)), 5, 5, 9, 5, defFmt.copy()},
		{fillBuffer(newFramebuffer(10, 10)), 5, 5, 9, 9, defFmt.copy()},
		{fillBuffer(newFramebuffer(10, 10)), 0, 0, 0, 5, &format{bg: newColor(BG_BLUE)}},
		{fillBuffer(newFramebuffer(10, 10)), 5, 5, 9, 9, &format{bg: newColor(BG_RED)}},
	}

	for i, c := range cases {
		empty := defaultCell()
		empty.f = c.fm

		c.fb.setCells(c.t, c.b, c.l, c.r, empty)

		for row := range c.fb.data {
			for col := range row {
				got, _ := c.fb.cell(row, col)
				if row >= c.t && row <= c.b && col >= c.l && col <= c.r {
					if !got.equal(empty) {
						t.Errorf("%d: (row:%d, col:%d) Got\n\t%v, wanted\n\t%v", i, row, col, got, empty)
						break
					}
				} else {
					if got.equal(empty) {
						t.Errorf("%d: Expected empty, got %v", i, got)
						break
					}
				}
			}
		}
	}
}

func TestResetRows(t *testing.T) {
	cases := []struct {
		fb         *framebuffer
		start, end int
		want       bool
	}{
		{fillBuffer(newFramebuffer(2, 2)), 0, 0, true},
		{fillBuffer(newFramebuffer(2, 2)), 0, -1, false},
		{fillBuffer(newFramebuffer(2, 2)), -1, 0, false},
		{fillBuffer(newFramebuffer(2, 2)), 2, 2, false},
		{fillBuffer(newFramebuffer(2, 2)), 2, 1, false},
		{fillBuffer(newFramebuffer(24, 80)), 15, 18, true},
	}

	empty := defaultCell()

	for i, c := range cases {
		resetWorked := c.fb.resetRows(c.start, c.end)
		if resetWorked != c.want {
			t.Errorf("%d: Got %t, wanted %t", i, resetWorked, c.want)
		} else {
			if resetWorked {
				nr := c.fb.rows()
				nc := c.fb.cols()
				for row := 0; row < nr; row++ {
					for col := 0; col < nc; col++ {
						got, _ := c.fb.cell(row, col)
						if row < c.start || row > c.end {
							if got.equal(empty) {
								t.Errorf("%d: (row:%d, col:%d) Got %v, wanted non-default", i, row, col, got)
							}
						} else {
							if !got.equal(empty) {
								t.Errorf("%d: (row:%d, col:%d) Got\n\t%v, wanted\n\t%v", i, row, col, got, empty)
							}
						}
					}
				}
			}
		}

	}
}

func TestSetAndGetCell(t *testing.T) {
	cases := []struct {
		row, col int
		c        *cell
		wantErr  error
	}{
		{5, 5, defaultCell(), nil},
		{1, 2, newCell('a', &format{fg: newColor(FG_BRIGHT_BLACK), attrs: UNDERLINE}, defOSC8.copy()), nil},
		{1, 2, newCell('b', &format{fg: newColor(FG_RED), attrs: STRIKEOUT}, defOSC8.copy()), nil},
		{8, 3, newCell('b', &format{bg: newColor(BG_BLUE), attrs: REVERSED}, defOSC8.copy()), nil},
		{10, 01, newCell('b', &format{fg: newColor(FG_BRIGHT_BLACK), attrs: UNDERLINE}, defOSC8.copy()), fbInvalidCell},
		{-1, 100, newCell('b', &format{fg: newColor(FG_BRIGHT_BLACK), attrs: UNDERLINE}, defOSC8.copy()), fbInvalidCell},
		{-1, 1, newCell('b', &format{fg: newColor(FG_BRIGHT_BLACK), attrs: UNDERLINE}, defOSC8.copy()), fbInvalidCell},
		{1, -1, newCell('b', &format{fg: newColor(FG_BRIGHT_BLACK), attrs: UNDERLINE}, defOSC8.copy()), fbInvalidCell},
	}

	fb := newFramebuffer(10, 10)
	for i, c := range cases {
		fb.setCell(c.row, c.col, c.c)
		got, err := fb.cell(c.row, c.col)

		if err == nil && !got.equal(c.c) {
			t.Errorf("%d: Got %v (%v), wanted %v (%v)", i, got, c.c, err, c.wantErr)
		} else {
			if !errors.Is(err, c.wantErr) {
				t.Errorf("%d: Got error %v, wanted error %v", i, err, c.wantErr)
			}
		}
	}
}

func TestResize(t *testing.T) {
	cases := []struct {
		fb           *framebuffer
		nrows, ncols int // updated size params
		want         bool
	}{
		{newFramebuffer(2, 2), 4, 4, true},
		{newFramebuffer(4, 4), 2, 2, true},
		{newFramebuffer(10, 10), MIN_ROWS, MIN_COLS, true},
		{newFramebuffer(10, 10), MAX_ROWS, MAX_COLS, true},
		{newFramebuffer(10, 10), 20, MIN_COLS - 1, false},
		{newFramebuffer(10, 10), MIN_ROWS - 1, 5, false},
		{newFramebuffer(10, 10), MAX_ROWS + 1, 20, false},
		{newFramebuffer(10, 10), 20, MAX_COLS + 1, false},
	}

	for i, c := range cases {
		got := c.fb.resize(c.nrows, c.ncols)
		if got != c.want {
			t.Errorf("%d: Expected %t resize, but got %t", i, c.want, got)
		} else {
			if got && (c.fb.rows() != c.nrows || c.fb.cols() != c.ncols) {
				t.Errorf("%d: Expected (%d, %d), got (%d, %d)", i, c.nrows, c.ncols, c.fb.rows(), c.fb.cols())
			}
		}
	}

	// Separate test to ensure we never leave a split "fragmented" cell behind.
	fb := newFramebuffer(10, 10)
	fb.setCell(0, 8, fragCell('世', defFmt.copy(), defOSC8.copy(), FRAG_PRIMARY))
	fb.setCell(0, 9, fragCell(0, defFmt.copy(), defOSC8.copy(), FRAG_SECONDARY))

	// this should split the width 2 fragment we just added and
	// force (0,8) to be cleared
	fb.resize(10, 9)

	tfb := newFramebuffer(10, 9)
	if !tfb.equal(fb) {
		t.Errorf("Chopping a wide character fragment in half failed. Wanted:\n%s\nGot:\n%s", tfb, fb)
	}
}

func TestScrollRows(t *testing.T) {
	cases := []struct {
		fb     *framebuffer
		scroll int
		want   *framebuffer
	}{
		{numberedFBForTest(0, 8, 10, 0, 0), 0, numberedFBForTest(0, 8, 10, 0, 0)},
		{numberedFBForTest(0, 8, 10, 0, 0), 2, numberedFBForTest(2, 8, 10, 0, 2)},
		{numberedFBForTest(0, 8, 10, 0, 0), 8, newFramebuffer(8, 10)},
		{numberedFBForTest(0, 8, 10, 0, 0), -1, numberedFBForTest(0, 8, 10, 1, 0)},
	}

	for i, c := range cases {
		c.fb.scrollRows(c.scroll)
		if !c.fb.equal(c.want) {
			t.Errorf("%d: Got\n%v, wanted\n%v", i, c.fb, c.want)
		}
	}
}

func TestComplexScrollRows(t *testing.T) {
	// Emulate a sequence we found in emacs that broke us
	// 1;51r cur="(44, 0)"
	// 23;1H cur="(0, 0)"
	// 8L cur="(22, 0)"
	// 1;28r cur="(22, 0)"
	// 1;1H cur="(0, 0)"
	// 8L cur="(0, 0)"
	// 1;60r cur="(0, 0)"
	//
	// IOW, set a margin (0,50), move back to the 23rd line,
	// scroll 8 lines down in the region, set the margin again
	// (0,27), move the cursor to the home row, scroll 8 lines
	// down in the new region, reset margin to the full height of
	// the terminal.
	input := []byte("\x1b[1;51r\x1b[23;1H\x1b[8L\x1b[1;28r\x1b[1;1H\x1b[8L\x1b[1;60r\x1b[60;1H")

	fb := newFramebuffer(60, 10)
	for i, n := range []int{0, 7, 22, 30, 50, 51, 59} {
		fbr, err := fb.subRegion(n, n, 0, 9)
		if err != nil {
			t.Errorf("couldn't pull subregion (%d): %v", n, err)
			break
		}
		fbr.fill(newCell(rune('0'+i), defFmt.copy(), defOSC8.copy()))
	}

	fbw := newFramebuffer(60, 10)
	nums := []int{0, 1, 2, 3, 5, 6}
	for i, n := range []int{8, 15, 30, 38, 51, 59} {
		fbr, err := fbw.subRegion(n, n, 0, 9)
		if err != nil {
			t.Errorf("couldn't pull subregion(%d): %v", n, err)
			break
		}
		fbr.fill(newCell(rune('0'+nums[i]), defFmt.copy(), defOSC8.copy()))
	}

	t1, _ := NewTerminal(60, 10)
	t1.fb = fb
	t1.cur = cursor{44, 0}

	t1.Write(input)

	if !t1.fb.equal(fbw) {
		t.Errorf("unexpected differences; Got:\n%v\nWanted:\n%v\n\n", t1.fb, fbw)
	}

}

func TestFBEquality(t *testing.T) {
	dfb := newFramebuffer(10, 10)
	ofb := newFramebuffer(10, 10)
	ofb.setCell(5, 5, newCell('z', &format{attrs: UNDERLINE}, defOSC8.copy()))

	cases := []struct {
		fb   *framebuffer
		ofb  *framebuffer
		want bool
	}{
		{newFramebuffer(5, 5), newFramebuffer(5, 5), true},
		{newFramebuffer(5, 10), newFramebuffer(5, 10), true},
		{newFramebuffer(10, 5), newFramebuffer(10, 5), true},
		{newFramebuffer(10, 10), newFramebuffer(5, 5), false},
		{newFramebuffer(5, 10), newFramebuffer(2, 10), false},
		{dfb, ofb, false},
	}

	for i, c := range cases {
		if got := c.fb.equal(c.ofb); got != c.want {
			t.Errorf("%d: Got %t, wanted %t, comparing:\n%s and \n\n%s", i, got, c.want, c.fb, c.ofb)
		}
	}
}

func TestCopy(t *testing.T) {
	cases := []struct {
		fb         *framebuffer
		l, r, t, b int
	}{
		{fillBuffer(newFramebuffer(10, 10)), 2, 1, 3, 4},
		{fillBuffer(newFramebuffer(20, 15)), 3, 2, 1, 9},
	}

	for i, c := range cases {
		cfb := c.fb.copy()
		if !cfb.equal(c.fb) {
			t.Errorf("%d: %v != %v", i, cfb, c.fb)
		}
	}
}

func TestAnsiOSCSize(t *testing.T) {
	cases := []struct {
		fb   *framebuffer
		want string
	}{
		{newFramebuffer(10, 10), fmt.Sprintf("%c%cX;%d;%d%c", ESC, OSC, 10, 10, BEL)},
		{newFramebuffer(10, 5), fmt.Sprintf("%c%cX;%d;%d%c", ESC, OSC, 10, 5, BEL)},
		{newFramebuffer(15, 22), fmt.Sprintf("%c%cX;%d;%d%c", ESC, OSC, 15, 22, BEL)},
	}

	for i, c := range cases {
		if got := string(c.fb.ansiOSCSize()); got != c.want {
			t.Errorf("%d: Got\n\t%v, wanted\n\t%v", i, got, c.want)
		}
	}
}

func TestFrameBufferDiff(t *testing.T) {
	fb1 := newFramebuffer(10, 10)
	fb2 := newFramebuffer(10, 10)
	fb3 := newFramebuffer(10, 10)
	fb3.resize(10, 20)
	fb3.setCell(5, 11, newCell('a', defFmt.copy(), defOSC8.copy()))
	fb4 := fb3.copy()
	fb4.setCell(5, 12, newCell('b', defFmt.copy(), defOSC8.copy()))
	fb5 := fb4.copy()
	fb5.setCell(5, 12, newCell('b', &format{fg: newColor(FG_GREEN)}, defOSC8.copy()))
	fb5.setCell(5, 13, newCell('c', &format{fg: newColor(FG_GREEN)}, defOSC8.copy()))

	fb6 := fb5.copy()
	fb6.setCell(1, 0, newCell('X', &format{fg: newColor(FG_BLUE), bg: newColor(BG_RED)}, defOSC8.copy()))
	fb6.setCell(5, 12, newCell('Y', &format{fg: newColor(FG_BLUE), bg: newColor(BG_RED)}, defOSC8.copy()))
	fb6.setCell(5, 13, newCell('Z', &format{fg: newColor(FG_YELLOW), bg: newColor(BG_GREEN)}, defOSC8.copy()))
	fb6.resize(10, 13)

	fb7 := newFramebuffer(24, 80)
	fb8 := fb7.copy()

	fb8.setCell(0, 0, newCell(' ', defFmt.copy(), defOSC8.copy()))
	fb8.setCell(0, 1, newCell('a', &format{bg: newColor(BG_BLACK)}, defOSC8.copy()))
	fb8.setCell(0, 2, newCell('b', &format{bg: newColor(BG_BLACK)}, defOSC8.copy()))
	fb8.setCell(0, 3, newCell('c', &format{bg: newColor(BG_BLACK)}, defOSC8.copy()))
	fb8.setCell(0, 4, newCell(' ', &format{bg: newColor(BG_BLACK)}, defOSC8.copy()))
	fb8.setCell(0, 5, newCell('\ue0b0', &format{fg: newColor(FG_BLACK), bg: newColor(BG_BLUE)}, defOSC8.copy()))
	fb8.setCell(0, 6, newCell(' ', &format{fg: newColor(FG_BLACK), bg: newColor(BG_BLUE)}, defOSC8.copy()))
	fb8.setCell(0, 7, newCell('~', &format{fg: newColor(FG_BLACK), bg: newColor(BG_BLUE)}, defOSC8.copy()))
	fb8.setCell(0, 8, newCell(' ', &format{fg: newColor(FG_BLACK), bg: newColor(BG_BLUE)}, defOSC8.copy()))
	fb8.setCell(0, 9, newCell('\ue0b0', &format{fg: newColor(FG_BLUE), bg: newColor(BG_DEF)}, defOSC8.copy()))
	fb8.setCell(0, 10, newCell(' ', defFmt.copy(), defOSC8.copy()))
	fb9 := newFramebuffer(10, 10)
	fb9.setCell(0, 0, newCell('A', defFmt.copy(), defOSC8.copy()))
	fb10 := fb9.copy()
	fb10.setCell(0, 1, newCell('*', defFmt.copy(), defOSC8.copy()))
	fb11 := fb9.copy()
	fb11.setCell(0, 0, newCell('B', defFmt.copy(), defOSC8.copy()))

	fb12 := newFramebuffer(24, 80)
	fb12.setCell(0, 78, newCell('y', defFmt.copy(), defOSC8.copy()))
	fb12.setCell(0, 79, newCell('y', defFmt.copy(), defOSC8.copy()))
	fb13 := newFramebuffer(24, 80)
	fb13.setCell(0, 78, newCell('y', defFmt.copy(), defOSC8.copy()))

	cases := []struct {
		srcFB, destFB *framebuffer
		want          string
	}{
		// no diff
		{fb1, fb2, ""},
		// set size, move cursor, write rune
		{fb2, fb3, "\x1b]X;10;20\a\x1b[6;12Ha"},
		// move cursor, write rune
		{fb3, fb4, "\x1b[6;13Hb"},
		// move cursor, set pen, write runes
		{fb4, fb5, "\x1b[6;13H\x1b[32mbc"},
		// cursor, set pen, write 2 runes set size, move
		// cursor, set pen, write rune, move cursor, write
		// rune (only Y, no Z because of resize)
		{fb5, fb6, "\x1b]X;10;13\a\x1b[2H\x1b[34m\x1b[41mX\x1b[6;13HY"},
		{fb7, fb8, "\x1b[H \x1b[40mabc \x1b[30m\x1b[44m\ue0b0 ~ \x1b[34m\x1b[49m\ue0b0\x1b[m "},
		{fb9, fb10, "\x1b[;2H*"},
		{fb9, fb11, "\x1b[HB"},
		{fb12, fb13, "\x1b[;80H "},
	}

	for i, c := range cases {
		// shadows, but ok
		srcFB := c.srcFB.copy()
		destFB := c.destFB.copy()
		if got := string(srcFB.diff(destFB)); got != c.want {
			t.Errorf("%d: Got\n\t%q, wanted\n\t%q", i, got, c.want)
		}
	}
}

// Assumes 0-7 for rows so we can a) make cell content to a rune
// representing original row and b) index into the standard foreground
// colors; start indicates the numeric rune we start from when
// creating the framebuffer and defaultsStart and defaultsEnd
// indicates how many empty rows to add at the beginning and end.
func numberedFBForTest(start, rows, cols, defaultsStart, defaultsEnd int) *framebuffer {
	fb := newFramebuffer(rows, cols)
	for i := 0; i < defaultsStart; i++ {
		fb.data[i] = newRow(cols)
	}

	for r := defaultsStart; r < rows-defaultsEnd; r++ {
		row := fb.data[r]
		for c := range row {
			fb.setCell(r, c, newCell(rune(r+-defaultsStart+start+'0'), &format{fg: newColor(30 + start - defaultsStart + r), bg: newDefaultColor()}, defOSC8.copy()))
		}
	}

	for r := rows - defaultsEnd; r < rows; r++ {
		fb.data[r] = newRow(cols)
	}
	return fb
}

func TestSubRegion(t *testing.T) {
	dfb := numberedFBForTest(0, 8, 10, 0, 0)
	cases := []struct {
		fb         *framebuffer
		t, b, l, r int
		want       *framebuffer
		wantErr    error
	}{
		{dfb, 0, 10, 0, 9, nil, invalidRegion},
		{dfb, 0, 7, 0, 10, nil, invalidRegion},
		{dfb, 0, 7, 0, 9, numberedFBForTest(0, 8, 10, 0, 0), nil},
		{dfb, 1, 7, 0, 9, numberedFBForTest(1, 7, 10, 0, 0), nil},
		{dfb, 1, 6, 1, 9, numberedFBForTest(1, 6, 9, 0, 0), nil},
		{dfb, 1, 6, 1, 9, numberedFBForTest(1, 6, 9, 0, 0), nil},
		{dfb, 2, 2, 1, 9, numberedFBForTest(2, 1, 9, 0, 0), nil},
	}

	for i, c := range cases {
		if got, err := c.fb.subRegion(c.t, c.b, c.l, c.r); !errors.Is(err, c.wantErr) || (got != nil && !got.equal(c.want)) {
			t.Errorf("%d: Got\n%v (%v) wanted:\n%v (%v)", i, got, err, c.want, c.wantErr)
		}
	}
}
