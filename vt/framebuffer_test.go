package vt

import (
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
	for row := 0; row < fb.rows; row++ {
		for col := 0; col < fb.cols; col++ {
			fb.setCell(row, col, 'a'+rune(rand.Intn(26)), nonDefFmt)
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

func TestResetRows(t *testing.T) {
	cases := []struct {
		fb         *framebuffer
		start, end int
	}{
		{fillBuffer(newFramebuffer(2, 2)), 0, 0},
		{fillBuffer(newFramebuffer(24, 80)), 15, 18},
	}

	empty := emptyCell(defFmt)

	for i, c := range cases {
		c.fb.resetRows(c.start, c.end, defFmt)
		for row := 0; row < c.fb.rows; row++ {
			for col := 0; col < c.fb.cols; col++ {
				got := c.fb.getCell(row, col)
				if row < c.start || row > c.end {
					if got.equal(empty) {
						t.Errorf("%d: (row:%d, col:%d) Got %v, wanted non-default", i, row, col, got)
					}
				} else {
					if !got.equal(empty) {
						t.Errorf("%d: (row:%d, col:%d) Got %v, wanted %v", i, row, col, got, empty)
					}
				}
			}
		}
	}
}
