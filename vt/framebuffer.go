package vt

type cell struct {
	r rune
	f format
}

type framebuffer struct {
	rows, cols int
	data       [][]cell
}

func newFramebuffer(rows, cols int) *framebuffer {
	d := make([][]cell, rows, rows)
	for r := 0; r < rows; r++ {
		d[r] = make([]cell, cols, cols)
	}
	return &framebuffer{
		rows: rows,
		cols: cols,
		data: d,
	}
}

func (f *framebuffer) setCell(row, col int, fm format, r rune) {
	f.data[row][col] = cell{r: r, f: fm}
}
