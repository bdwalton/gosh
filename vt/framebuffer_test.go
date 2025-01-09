package vt

import (
	"errors"
	"math/rand"
	"testing"
)

var nonDefFmt = format{
	fg:        standardColors[FG_YELLOW],
	bg:        standardColors[BG_BLUE],
	underline: UNDERLINE_DOUBLE,
	italic:    true,
}

func fillBuffer(fb *framebuffer) *framebuffer {
	for row := 0; row < fb.getRows(); row++ {
		for col := 0; col < fb.getCols(); col++ {
			fb.setCell(row, col, newCell('a'+rune(rand.Intn(26)), nonDefFmt))
		}
	}

	return fb
}

func TestCellEquality(t *testing.T) {
	cases := []struct {
		c1, c2 cell
		want   bool
	}{
		{cell{}, cell{}, true},
		{cell{f: defFmt}, cell{f: defFmt}, true},
		{cell{r: 'a', f: defFmt}, cell{r: 'a', f: defFmt}, true},
		{cell{r: 'a', f: format{italic: true}}, cell{r: 'a', f: format{italic: true}}, true},
		{cell{f: defFmt}, cell{r: 'a', f: defFmt}, false},
		{cell{r: 'a'}, cell{r: 'a', f: defFmt}, true},
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
		want            bool
	}{
		{fillBuffer(newFramebuffer(10, 10)), 0, 0, 5, true},
		{fillBuffer(newFramebuffer(10, 10)), 0, 5, 9, true},
		{fillBuffer(newFramebuffer(10, 10)), -1, 5, 9, false},
		{fillBuffer(newFramebuffer(10, 10)), 10, 5, 9, false},
		{fillBuffer(newFramebuffer(10, 10)), 5, 9, 5, false},
		{fillBuffer(newFramebuffer(10, 10)), 5, 9, 9, true},
	}

	empty := defaultCell()

	for i, c := range cases {
		resetWorked := c.fb.resetCells(c.row, c.start, c.end)
		if resetWorked != c.want {
			t.Errorf("%d: Got %t, wanted %t", i, resetWorked, c.want)
		} else {
			if resetWorked {
				nr := c.fb.getRows()
				nc := c.fb.getCols()
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
				nr := c.fb.getRows()
				nc := c.fb.getCols()
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
			if got && (c.fb.getRows() != c.nrows || c.fb.getCols() != c.ncols) {
				t.Errorf("%d: Expected (%d, %d), got (%d, %d)", i, c.nrows, c.ncols, c.fb.getRows(), c.fb.getCols())
			}
		}
	}
}
