package vt

import (
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"testing"
)

var nonDefFmt = format{
	fg:        standardColors[FG_YELLOW],
	bg:        standardColors[BG_BLUE],
	underline: UNDERLINE_DOUBLE,
	italic:    true,
}

func fillBuffer(fb *framebuffer) *framebuffer {
	for row := 0; row < fb.getNumRows(); row++ {
		for col := 0; col < fb.getNumCols(); col++ {
			fb.setCell(row, col, newCell('a'+rune(rand.Intn(26)), nonDefFmt))
		}
	}

	return fb
}

func TestCellDiff(t *testing.T) {
	cases := []struct {
		src, dest cell
		want      []byte
	}{
		{
			defaultCell(),
			defaultCell(),
			[]byte{},
		},
		{
			defaultCell(),
			fragCell('世', defFmt, FRAG_SECONDARY),
			[]byte{},
		},
		{
			newCell('a', defFmt),
			newCell(' ', defFmt),
			[]byte{' '},
		},
		{
			newCell('b', format{fg: standardColors[FG_BLUE]}),
			newCell(' ', defFmt),
			[]byte{ESC, ESC_CSI, CSI_SGR, ' '},
		},
		{
			newCell('b', format{italic: true}),
			newCell('b', defFmt),
			[]byte{ESC, ESC_CSI, CSI_SGR, 'b'},
		},
		{
			newCell('b', defFmt),
			newCell('b', format{italic: true}),
			[]byte(fmt.Sprintf("%c%c%d%c%c", ESC, ESC_CSI, ITALIC_ON, CSI_SGR, 'b')),
		},
	}

	for i, c := range cases {
		if got := c.src.diff(c.dest); !slices.Equal(got, c.want) {
			t.Errorf("\n%d: Got: %v\nWanted: %v", i, got, c.want)
		}
	}
}

func TestCellEfficientDiff(t *testing.T) {
	cases := []struct {
		src, dest cell
		f         format
		want      []byte
	}{
		{
			defaultCell(),
			defaultCell(),
			defFmt,
			[]byte{},
		},
		{
			newCell('a', format{fg: standardColors[FG_RED]}),
			newCell('a', format{fg: standardColors[FG_RED]}),
			format{fg: standardColors[FG_RED]},
			[]byte{},
		},
		{
			newCell('a', format{fg: standardColors[FG_RED]}),
			newCell('a', format{fg: standardColors[FG_RED]}),
			defFmt,
			[]byte(fmt.Sprintf("%c%c%d%c%c", ESC, ESC_CSI, FG_RED, CSI_SGR, 'a')),
		},
		{
			newCell('a', format{fg: standardColors[FG_RED]}),
			newCell('a', format{bg: standardColors[BG_RED]}),
			defFmt,
			[]byte(fmt.Sprintf("%c%c%d%c%c", ESC, ESC_CSI, BG_RED, CSI_SGR, 'a')),
		},
	}

	for i, c := range cases {
		if got := c.src.efficientDiff(c.dest, c.f); !slices.Equal(got, c.want) {
			t.Errorf("\n%d: Got: %v %q\nWanted: %v %q", i, got, string(got), c.want, string(c.want))
		}
	}
}

func TestCellEquality(t *testing.T) {
	cases := []struct {
		c1, c2 cell
		want   bool
	}{
		{cell{}, cell{}, true},
		{cell{frag: 1}, cell{}, false},
		{cell{f: defFmt}, cell{f: defFmt}, true},
		{cell{f: defFmt}, cell{f: defFmt, frag: 2}, false},
		{cell{r: 'a', f: defFmt}, cell{r: 'a', f: defFmt}, true},
		{cell{r: 'a', f: format{italic: true}}, cell{r: 'a', f: format{italic: true}}, true},
		{cell{f: defFmt}, cell{r: 'a', f: defFmt}, false},
		{cell{r: 'a'}, cell{r: 'a', f: defFmt}, true},
		{cell{r: 'a', frag: 1}, cell{r: 'a', f: defFmt}, false},
		{cell{r: 'a'}, cell{r: 'b'}, false},
		{cell{r: 'a', f: defFmt}, cell{r: 'a'}, true},
		{cell{r: 'a', f: format{italic: true}}, cell{r: 'a'}, false},
	}

	for i, c := range cases {
		if got := c.c1.equal(c.c2); got != c.want {
			t.Errorf("%d: Got %t, wanted %t when comparing %v.equal(%v)", i, got, c.want, c.c1, c.c2)
		}
	}
}

func TestResetCells(t *testing.T) {
	cases := []struct {
		fb              *framebuffer
		row, start, end int
		fm              format
		want            bool
	}{
		{fillBuffer(newFramebuffer(10, 10)), 0, 0, 5, defFmt, true},
		{fillBuffer(newFramebuffer(10, 10)), 0, 5, 9, defFmt, true},
		{fillBuffer(newFramebuffer(10, 10)), -1, 5, 9, defFmt, false},
		{fillBuffer(newFramebuffer(10, 10)), 10, 5, 9, defFmt, false},
		{fillBuffer(newFramebuffer(10, 10)), 5, 9, 5, defFmt, false},
		{fillBuffer(newFramebuffer(10, 10)), 5, 9, 9, defFmt, true},
		{fillBuffer(newFramebuffer(10, 10)), 0, 0, 5, format{bg: standardColors[BG_BLUE]}, true},
		{fillBuffer(newFramebuffer(10, 10)), 5, 9, 9, format{bg: standardColors[BG_RED]}, true},
	}

	for i, c := range cases {
		empty := defaultCell()
		empty.f = c.fm

		resetWorked := c.fb.resetCells(c.row, c.start, c.end, c.fm)
		if resetWorked != c.want {
			t.Errorf("%d: Got %t, wanted %t", i, resetWorked, c.want)
		} else {
			if resetWorked {
				nr := c.fb.getNumRows()
				nc := c.fb.getNumCols()
				for row := 0; row < nr; row++ {
					for col := 0; col < nc; col++ {
						got, _ := c.fb.getCell(row, col)
						if row == c.row {
							if col < c.start || col >= c.end {
								if got.equal(empty) {
									t.Errorf("%d: (row:%d, col:%d) Got\n\t%v, wanted\n\t%v", i, row, col, got, empty)
								}
							} else {
								if !got.equal(empty) {
									t.Errorf("%d: Got %t, wanted %t, expected empty, got %v", i, resetWorked, c.want, got)
								}
							}
						} else {
							if got.equal(empty) {
								t.Errorf("%d: (row:%d, col:%d) Got %v, wanted non-default", i, row, col, got)
							}
						}
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
				nr := c.fb.getNumRows()
				nc := c.fb.getNumCols()
				for row := 0; row < nr; row++ {
					for col := 0; col < nc; col++ {
						got, _ := c.fb.getCell(row, col)
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
		c        cell
		wantErr  error
	}{
		{5, 5, defaultCell(), nil},
		{1, 2, newCell('a', format{fg: standardColors[FG_BRIGHT_BLACK], italic: true}), nil},
		{1, 2, newCell('b', format{fg: standardColors[FG_RED], strikeout: true}), nil},
		{8, 3, newCell('b', format{bg: standardColors[BG_BLUE], reversed: true}), nil},
		{10, 01, newCell('b', format{fg: standardColors[FG_BRIGHT_BLACK], italic: true}), fbInvalidCell},
		{-1, 100, newCell('b', format{fg: standardColors[FG_BRIGHT_BLACK], italic: true}), fbInvalidCell},
		{-1, 1, newCell('b', format{fg: standardColors[FG_BRIGHT_BLACK], italic: true}), fbInvalidCell},
		{1, -1, newCell('b', format{fg: standardColors[FG_BRIGHT_BLACK], italic: true}), fbInvalidCell},
	}

	fb := newFramebuffer(10, 10)
	for i, c := range cases {
		fb.setCell(c.row, c.col, c.c)
		got, err := fb.getCell(c.row, c.col)

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
			if got && (c.fb.getNumRows() != c.nrows || c.fb.getNumCols() != c.ncols) {
				t.Errorf("%d: Expected (%d, %d), got (%d, %d)", i, c.nrows, c.ncols, c.fb.getNumRows(), c.fb.getNumCols())
			}
		}
	}

	// Separate test to ensure we never leave a split "fragmented" cell behind.
	fb := newFramebuffer(10, 10)
	fb.setCell(0, 8, fragCell('世', defFmt, FRAG_PRIMARY))
	fb.setCell(0, 9, fragCell(0, defFmt, FRAG_SECONDARY))

	// this should split the width 2 fragment we just added and
	// force (0,8) to be cleared
	fb.resize(10, 9)

	tfb := newFramebuffer(10, 9)
	if !tfb.equal(fb) {
		t.Errorf("Chopping a wide character fragment in half failed. Wanted:\n%s\nGot:\n%s", tfb, fb)
	}
}

func TestScrollRows(t *testing.T) {
	fb1 := numberedFBForTest(2, 6, 10, 2)

	cases := []struct {
		fb     *framebuffer
		scroll int
		want   *framebuffer
	}{
		{numberedFBForTest(0, 8, 10, 0), 0, numberedFBForTest(0, 8, 10, 0)},
		{numberedFBForTest(0, 8, 10, 0), 2, fb1},
		{numberedFBForTest(0, 8, 10, 0), 8, newFramebuffer(8, 10)},
	}

	for i, c := range cases {
		c.fb.scrollRows(c.scroll)
		if !c.fb.equal(c.want) {
			t.Errorf("%d: Got\n%v, wanted\n%v", i, c.fb, c.want)
		}
	}
}

func TestFBEquality(t *testing.T) {
	dfb := newFramebuffer(10, 10)
	ofb := newFramebuffer(10, 10)
	ofb.setCell(5, 5, newCell('z', format{italic: true}))

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
		want []byte
	}{
		{newFramebuffer(10, 10), []byte(fmt.Sprintf("%c%cX;%d;%d%c", ESC, ESC_OSC, 10, 10, CTRL_BEL))},
		{newFramebuffer(10, 5), []byte(fmt.Sprintf("%c%cX;%d;%d%c", ESC, ESC_OSC, 10, 5, CTRL_BEL))},
		{newFramebuffer(15, 22), []byte(fmt.Sprintf("%c%cX;%d;%d%c", ESC, ESC_OSC, 15, 22, CTRL_BEL))},
	}

	for i, c := range cases {
		if got := c.fb.ansiOSCSize(); !slices.Equal(got, c.want) {
			t.Errorf("%d: Got\n\t%v, wanted\n\t%v", i, got, c.want)
		}
	}
}

func TestFrameBufferDiff(t *testing.T) {
	fb1 := newFramebuffer(10, 10)
	fb2 := fb1.copy()
	fb3 := fb2.copy()
	fb3.resize(10, 20)
	fb3.setCell(5, 11, newCell('a', defFmt))
	fb4 := fb3.copy()
	fb4.setCell(5, 12, newCell('b', defFmt))
	fb5 := fb4.copy()
	fb5.setCell(5, 12, newCell('b', format{fg: standardColors[FG_GREEN]}))
	fb5.setCell(5, 13, newCell('c', format{fg: standardColors[FG_GREEN]}))

	fb6 := fb5.copy()
	fb6.setCell(1, 0, newCell('X', format{fg: standardColors[FG_BLUE], bg: standardColors[BG_RED], italic: true}))
	fb6.setCell(5, 12, newCell('Y', format{fg: standardColors[FG_BLUE], bg: standardColors[BG_RED], italic: true}))
	fb6.setCell(5, 13, newCell('Z', format{fg: standardColors[FG_YELLOW], bg: standardColors[BG_GREEN]}))
	fb6.resize(10, 13)

	fb7 := newFramebuffer(24, 80)
	fb8 := fb7.copy()

	fb8.setCell(0, 0, newCell(' ', defFmt))
	fb8.setCell(0, 1, newCell('a', format{bg: standardColors[BG_BLACK]}))
	fb8.setCell(0, 2, newCell('b', format{bg: standardColors[BG_BLACK]}))
	fb8.setCell(0, 3, newCell('c', format{bg: standardColors[BG_BLACK]}))
	fb8.setCell(0, 4, newCell(' ', format{bg: standardColors[BG_BLACK]}))
	fb8.setCell(0, 5, newCell('\ue0b0', format{fg: standardColors[FG_BLACK], bg: standardColors[BG_BLUE]}))
	fb8.setCell(0, 6, newCell(' ', format{fg: standardColors[FG_BLACK], bg: standardColors[BG_BLUE]}))
	fb8.setCell(0, 7, newCell('~', format{fg: standardColors[FG_BLACK], bg: standardColors[BG_BLUE]}))
	fb8.setCell(0, 8, newCell(' ', format{fg: standardColors[FG_BLACK], bg: standardColors[BG_BLUE]}))
	fb8.setCell(0, 9, newCell('\ue0b0', format{fg: standardColors[FG_BLUE], bg: standardColors[BG_DEF]}))
	fb8.setCell(0, 10, newCell(' ', defFmt))

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
		{fb5, fb6, "\x1b]X;10;13\a\x1b[2;H\x1b[34;41m\x1b[3mX\x1b[6;13HY"},
		{fb7, fb8, "\x1b[;H \x1b[40mabc \x1b[30;44m\ue0b0 ~ \x1b[34;49m\ue0b0\x1b[m "},
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

func TestGetRowRegion(t *testing.T) {
	ac := newCell('a', format{fg: standardColors[FG_RED]})
	bc := newCell('b', format{fg: standardColors[FG_BLUE]})
	fb := newFramebuffer(10, 10)
	fb.setCell(3, 5, ac)
	fb.setCell(4, 3, bc)
	fb.setCell(4, 4, ac)

	cases := []struct {
		fb               *framebuffer
		row, left, right int
		want             []cell
		wantErr          error
	}{
		{fb, 2, 5, 7, []cell{defaultCell(), defaultCell()}, nil},
		{fb, 3, 5, 7, []cell{ac, defaultCell()}, nil},
		{fb, 3, 5, 7, []cell{ac, defaultCell()}, nil},
		{fb, 4, 3, 6, []cell{bc, ac, defaultCell()}, nil},
		{fb, 4, 3, 11, nil, invalidRegion},
	}

	for i, c := range cases {
		if got, err := c.fb.getRowRegion(c.row, c.left, c.right); !errors.Is(err, c.wantErr) || !slices.Equal(got, c.want) {
			t.Errorf("%d: Got\n\t%v (%v), wanted\n\t%v (%v)", i, got, err, c.want, c.wantErr)
		}
	}
}

func TestSetRowRegion(t *testing.T) {
	ac := newCell('a', format{fg: standardColors[FG_RED]})
	bc := newCell('b', format{fg: standardColors[FG_BLUE]})
	fb := newFramebuffer(10, 10)
	fb.setCell(3, 5, ac)
	fb.setCell(4, 3, bc)
	fb.setCell(4, 4, ac)

	cases := []struct {
		fb               *framebuffer
		row, left, right int
		new              []cell
		want             []cell
		wantErr          error
	}{
		{fb, 2, 5, 7, []cell{defaultCell(), ac}, []cell{defaultCell(), ac}, nil},
		{fb, 3, 5, 7, []cell{ac, ac}, []cell{ac, ac}, nil},
		{fb, 3, 5, 7, []cell{bc, ac}, []cell{bc, ac}, nil},
		{fb, 4, 3, 6, []cell{ac, bc, bc}, []cell{ac, bc, bc}, nil},
		{fb, 5, 3, 6, []cell{ac, bc, bc, defaultCell()}, []cell{defaultCell(), defaultCell(), defaultCell()}, setRowRegionErr},
	}

	for i, c := range cases {
		err := c.fb.setRowRegion(c.row, c.left, c.right, c.new)
		if !errors.Is(err, c.wantErr) {
			t.Errorf("%d: Wanted err %v, got %v", i, c.wantErr, err)
		}
		got, _ := c.fb.getRowRegion(c.row, c.left, c.right)
		if !slices.Equal(got, c.want) {
			t.Errorf("%d: Got\n\t%v (err=%v), wanted\n\t%v (err=%v)", i, got, err, c.want, c.wantErr)
		}
	}
}

// Assumes 0-7 for rows so we can a) make cell content to a rune
// representing original row and b) index into the standard foreground
// colors; start indicates the numeric rune we start from when
// creating the framebuffer and defaults indicates how many empty rows
// to add at the end.
func numberedFBForTest(start, rows, cols, defaults int) *framebuffer {
	fb := newFramebuffer(rows, cols)
	for r, row := range fb.data {
		for c := range row {
			fb.setCell(r, c, newCell(rune(r+start+'0'), format{fg: standardColors[30+start+r]}))
		}
	}

	for i := 0; i < defaults; i++ {
		fb.data = append(fb.data, newRow(cols))
	}
	return fb
}

func TestGetRegion(t *testing.T) {
	dfb := numberedFBForTest(0, 8, 10, 0)
	cases := []struct {
		fb         *framebuffer
		t, b, l, r int
		want       *framebuffer
		wantErr    error
	}{
		{newFramebuffer(10, 10), 0, 10, 0, 10, newFramebuffer(10, 10), nil},
		{newFramebuffer(10, 10), 0, 11, 0, 10, newFramebuffer(10, 10), invalidRegion},
		{dfb, 0, 8, 0, 10, numberedFBForTest(0, 8, 10, 0), nil},
		{dfb, 1, 8, 0, 10, numberedFBForTest(1, 7, 10, 0), nil},
		{dfb, 1, 8, 1, 9, numberedFBForTest(1, 7, 8, 0), nil},
		{dfb, 1, 8, 1, 9, numberedFBForTest(1, 7, 8, 0), nil},
	}

	for i, c := range cases {

		if got, err := c.fb.getRegion(c.t, c.b, c.l, c.r); !errors.Is(err, c.wantErr) || (got != nil && !got.equal(c.want)) {
			t.Errorf("%d: Got\n%v (%v) wanted:\n%v (%v)", i, got, err, c.want, c.wantErr)
		}
	}
}
