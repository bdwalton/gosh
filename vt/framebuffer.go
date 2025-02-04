package vt

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"unicode"
)

var fbInvalidCell = errors.New("invalid framebuffer cell")

const (
	MIN_ROWS = 1
	MIN_COLS = 2
	MAX_ROWS = 511 // taken from libvte
	MAX_COLS = MAX_ROWS
)

const (
	FRAG_NONE = iota
	FRAG_PRIMARY
	FRAG_SECONDARY
)

type cell struct {
	r rune
	f format
	// when non-zero, indicates this cell participates in width 2 character
	// 1 = primary rune
	// 2 = spare/empty cell next to primary
	frag int
}

func defaultCell() cell {
	return cell{f: defFmt}
}

// fragCell returns a cell tagged as a fragment (number = fn), with
// content and format as specified.
func fragCell(r rune, f format, fn int) cell {
	c := newCell(r, f)
	c.frag = fn
	return c
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

func (c cell) diff(dest cell) []byte {
	// Rely on consumer of the diff having accepted the
	// FRAG_SECONDARY already and doing the right things with
	// that.
	if dest.frag == FRAG_SECONDARY {
		return []byte{}
	}

	var sb strings.Builder

	cf, df := c.getFormat(), dest.getFormat()
	fe := cf.equal(df)

	if !fe {
		sb.Write(cf.diff(df))
	}

	// When computing cell difference, rewrite the rune if it's
	// different _or_ if the format is different. If we only
	// rewrite the format, the pen color will change, but the cell
	// wouldn't actually be updated.
	if dest.r != c.r || !fe {
		sb.WriteRune(dest.r)
	}

	return []byte(sb.String())
}

func (c cell) efficientDiff(dest cell, f format) []byte {
	nc := newCell(c.r, f)
	if c.frag != FRAG_NONE {
		nc.frag = c.frag
	}
	return nc.diff(dest)
}

func (c cell) String() string {
	return fmt.Sprintf("%s (f:%d) (%s)", string(c.r), c.frag, c.f.String())
}

type framebuffer struct {
	data [][]cell
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

func (f *framebuffer) ansiOSCSize() []byte {
	return []byte(fmt.Sprintf("%c%c%s;%d;%d%c", ESC, ESC_OSC, OSC_SETSIZE, f.getRows(), f.getCols(), ESC_ST))
}

func (src *framebuffer) diff(dest *framebuffer) []byte {
	var sb strings.Builder

	lastF := defFmt
	lastCur := cursor{0, 0}

	sz := dest.ansiOSCSize()
	if !slices.Equal(sz, src.ansiOSCSize()) {
		sb.Write(sz)
	}

	for r, row := range dest.data {
		for c, destCell := range row {
			cur := cursor{r, c}
			srcCell, err := src.getCell(r, c)
			if err != nil {
				srcCell = defaultCell()
			}

			if !srcCell.equal(destCell) {
				if cur.row != lastCur.row || cur.col != lastCur.col+1 {
					sb.WriteString(cur.getMoveToAnsi())
				}

				sb.Write(srcCell.efficientDiff(destCell, lastF))
				lastF = destCell.getFormat()
				lastCur = cur
			}
		}
	}

	return []byte(sb.String())
}

func (f *framebuffer) copy() *framebuffer {
	rows := f.getRows()
	cols := f.getCols()

	nf := &framebuffer{
		data: make([][]cell, rows, rows),
	}

	for row := range f.data {
		nf.data[row] = make([]cell, cols, cols)
		for col, c := range f.data[row] {
			nf.data[row][col] = c
		}
	}

	return nf
}

func (f *framebuffer) String() string {
	var sb strings.Builder
	for _, row := range f.data {
		for _, cell := range row {
			if cell.frag == 2 {
				continue
			}
			if unicode.IsPrint(cell.r) {
				sb.WriteString(string(cell.r))
			} else {
				sb.WriteString(".")
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (f *framebuffer) equal(other *framebuffer) bool {
	if f.getCols() != other.getCols() || f.getRows() != other.getRows() {
		return false
	}

	for r, row := range f.data {
		for c, cell := range row {
			oc, _ := other.getCell(r, c)
			if !cell.equal(oc) {
				return false
			}
		}
	}
	return true
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
			// Don't leave dangling fragments, if we
			// happen to chop one in half.
			c, err := f.getCell(i, cols-1)
			if err == nil && c.frag > 0 {
				f.setCell(i, cols-1, defaultCell())
			}
		case cols > nc:
			for i := 0; i < cols-nc; i++ {
				row = append(row, defaultCell())
			}
			f.data[i] = row
		}
	}

	return true
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

func (f *framebuffer) resetCells(row, from, to int, fm format) bool {
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
		resetCell := defaultCell()
		resetCell.f = fm
		for col := from; col < to; col++ {
			f.setCell(row, col, resetCell)
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

func (f *framebuffer) getRow(row int) []cell {
	return f.data[row]
}

func (f *framebuffer) getRowRegion(row, start, end int) []cell {
	return f.getRow(row)[start:end]
}

var setRowRegionErr = errors.New("setRegion dest and src length don't match")

func (f *framebuffer) setRowRegion(row, start, end int, src []cell) error {
	dest := f.getRowRegion(row, start, end)
	if len(src) != len(dest) {
		return setRowRegionErr
	}
	copy(dest, src)
	return nil
}
