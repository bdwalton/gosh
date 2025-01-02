package vt

type glyph struct {
	r rune
	f format
}

type framebuffer struct {
	rows, cols int
	data       [][]glyph
}

func newFramebuffer(rows, cols int) *framebuffer {
	d := make([][]glyph, rows, rows)
	for r := 0; r < rows; r++ {
		d[r] = make([]glyph, cols, cols)
	}
	return &framebuffer{
		rows: rows,
		cols: cols,
		data: d,
	}
}

func (f *framebuffer) setCell(row, col int, fm format, r rune) {
	f.data[row][col] = glyph{r: r, f: fm}
}
