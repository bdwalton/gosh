package vt

import (
	"errors"
	"fmt"
	"log/slog"
)

var fbInvalidCell = errors.New("invalid framebuffer cell")

const (
	MIN_ROWS = 1
	MIN_COLS = 2
	MAX_ROWS = 511 // taken from libvte
	MAX_COLS = MAX_ROWS
)

type cell struct {
	r    rune
	f    format
	frag bool // when true, this cell is part of a "wide character"
}

func defaultCell() cell {
	return cell{f: defFmt}
}

func fragCell() cell {
	return cell{f: defFmt, frag: true}
}

func newCell(r rune, f format) cell {
	return cell{r: r, f: f}
}

func (c cell) getFormat() format {
	return c.f
}

func (c cell) equal(other cell) bool {
	return c.r == other.r && c.frag == other.frag && c.getFormat().equal(other.getFormat())
}

func (c cell) String() string {
	return fmt.Sprintf("%s (f:%t) (%s)", string(c.r), c.frag, c.f.String())
}

type framebuffer struct {
	top, bottom, left, right int // scroll window parameters
	data                     [][]cell
}

func newFramebuffer(rows, cols int) *framebuffer {
	d := make([][]cell, rows, rows)
	for r := 0; r < rows; r++ {
		d[r] = newRow(cols)
	}
	return &framebuffer{
		data: d,
	}
}

func (f *framebuffer) scrollRows(n int) {
	nc := f.getCols()
	for i := 0; i < n; i++ {
		f.data = append(f.data, newRow(nc))
	}
	f.data = f.data[n:]
}

func (f *framebuffer) resize(rows, cols int) bool {
	if rows < MIN_ROWS || rows > MAX_ROWS || cols < MIN_COLS || cols > MAX_COLS {
		slog.Debug("won't resize to dimensions too large or small", "rows", rows, "cols", cols)
		return false
	}

	nr := len(f.data)
	nc := len(f.data[0])
	switch {
	case rows < nr:
		f.data = f.data[0:rows]
	case rows > nr:
		for i := 0; i < rows-nr; i++ {
			f.data = append(f.data, newRow(nc))
		}
	}

	for i, row := range f.data {
		switch {
		case cols < nc:
			f.data[i] = row[0:cols]
		case cols > nc:
			for i := 0; i < cols-nc; i++ {
				row = append(row, defaultCell())
			}
			f.data[i] = row
		}
	}

	return true
}

func (f *framebuffer) setTBScroll(top, bottom int) {
	f.top = top
	f.bottom = bottom
}

func (f *framebuffer) resetRows(from, to int) bool {
	if from > to || from < 0 || to >= f.getRows() {
		return false
	}

	nc := len(f.data[0])
	for i := from; i <= to; i++ {
		row := newRow(nc)
		f.data[i] = row
	}

	return true
}

func (f *framebuffer) resetCells(row, from, to int) bool {
	nr := len(f.data)
	nc := len(f.data[0])
	switch {
	case row < 0 || row >= nr:
		return false
	case from < 0 || from >= nc:
		return false
	case to < 0 || from >= nc:
		return false
	case from > to:
		return false
	default:
		for col := from; col < to; col++ {
			f.setCell(row, col, defaultCell())
		}
	}

	return true
}

func newRow(cols int) []cell {
	row := make([]cell, cols, cols)
	for i := 0; i < len(row); i++ {
		row[i] = defaultCell()
	}
	return row
}

func (f *framebuffer) getRows() int {
	return len(f.data)
}

func (f *framebuffer) getCols() int {
	return len(f.data[0])
}

func (f *framebuffer) validPoint(row, col int) bool {
	if row < 0 || row >= f.getRows() || col < 0 || col >= f.getCols() {
		return false
	}
	return true
}

func (f *framebuffer) setCell(row, col int, c cell) {
	if f.validPoint(row, col) {
		f.data[row][col] = c
	}
}

func (f *framebuffer) getCell(row, col int) (cell, error) {
	if f.validPoint(row, col) {
		return f.data[row][col], nil
	}

	return defaultCell(), fmt.Errorf("invalid coordinates (%d, %d): %w", col, row, fbInvalidCell)
}
