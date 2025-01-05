package vt

import "fmt"

type cell struct {
	r rune
	f format
}

func defaultCell() cell {
	return cell{f: defFmt}
}

func emptyCell(fm format) cell {
	c := defaultCell()
	c.f = fm
	return c
}

func newCell(r rune, f format) cell {
	return cell{r: r, f: f}
}

func (c cell) getFormat() format {
	return c.f
}

func (c cell) equal(other cell) bool {
	return c.getFormat().equal(other.getFormat()) && c.r == other.r
}

func (c cell) String() string {
	return fmt.Sprintf("%s (%s)", string(c.r), c.f.String())
}

type framebuffer struct {
	rows, cols int
	data       [][]cell
}

func newFramebuffer(rows, cols int) *framebuffer {
	d := make([][]cell, rows, rows)
	for r := 0; r < rows; r++ {
		d[r] = newRow(cols, defFmt)
	}
	return &framebuffer{
		rows: rows,
		cols: cols,
		data: d,
	}
}

func (f *framebuffer) resetRows(from, to int, fm format) {
	for i := from; i <= to; i++ {
		if i > len(f.data) {
			break
		}
		f.data[i] = newRow(f.cols, fm)
	}
}

func (f *framebuffer) resetCells(row, from, to int, fm format) {
	switch {
	case row < 0 || row >= f.rows:
		return
	case from < 0 || from >= f.cols:
		return
	case to < 0 || from >= f.cols:
		return
	default:
		for i := from; i < to; i++ {
			f.data[row][i] = newCell(fm)
		}
	}
}

func newRow(cols int, f format) []cell {
	row := make([]cell, cols, cols)
	for i := 0; i < len(row); i++ {
		row[i] = newCell(f)
	}
	return row
}

func (f *framebuffer) setCell(row, col int, fm format, r rune) {
	f.data[row][col] = cell{r: r, f: fm}
}
