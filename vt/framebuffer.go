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
	set bool // true if non-default
	r   rune
	f   format
	hl  *osc8
	// when non-zero, indicates this cell participates in width 2 character
	// 1 = primary rune
	// 2 = spare/empty cell next to primary
	frag int
}

func (c cell) isFragment() bool {
	return c.frag != FRAG_NONE
}

func (c cell) isPrimaryFrag() bool {
	return c.frag == FRAG_PRIMARY
}

func (c cell) isSecondaryFrag() bool {
	return c.frag == FRAG_SECONDARY
}

func defaultCell() cell {
	return cell{r: ' ', hl: defOSC8} // set == false, so our placeholder rune is a space
}

// fragCell returns a cell tagged as a fragment (number = fn), with
// content and format as specified.
func fragCell(r rune, f format, hl *osc8, fn int) cell {
	c := newCell(r, f, hl)
	c.frag = fn
	return c
}

func newCell(r rune, f format, hl *osc8) cell {
	return cell{set: true, r: r, f: f, hl: hl}
}

func (c cell) copy() cell {
	return cell{r: c.r, set: c.set, f: c.f, frag: c.frag, hl: c.hl}
}

func (c cell) format() format {
	return c.f
}

func (c cell) hyperlink() *osc8 {
	return c.hl
}

func (c cell) equal(other cell) bool {
	return c.set == other.set && c.r == other.r && c.frag == other.frag && c.format().equal(other.format()) && c.hl.equal(other.hl)
}

func (c cell) diff(dest cell) []byte {
	// Rely on consumer of the diff having accepted the
	// FRAG_SECONDARY already and doing the right things with
	// that.
	if dest.frag == FRAG_SECONDARY {
		return []byte{}
	}

	var sb strings.Builder

	cf, df := c.format(), dest.format()
	fe := cf.equal(df)

	if !fe {
		sb.Write(cf.diff(df))
	}

	hle := c.hl.equal(dest.hl)
	if !hle {
		sb.WriteString(dest.hl.ansiString())
	}

	// When computing cell difference, rewrite the rune if it's
	// different _or_ if the format is different. If we only
	// rewrite the format, the pen color will change, but the cell
	// wouldn't actually be updated.
	if dest.set != c.set || dest.r != c.r || !fe || !hle {
		sb.WriteRune(dest.r)
	}

	return []byte(sb.String())
}

func (c cell) efficientDiff(dest cell, f format, hl *osc8) []byte {
	nc := c.copy()
	nc.f = f
	nc.hl = hl
	return nc.diff(dest)
}

func (c cell) String() string {
	return fmt.Sprintf("%s (f:%d) (%s; %q)", string(c.r), c.frag, c.f.String(), c.hl.data)
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
	return []byte(fmt.Sprintf("%c%c%s;%d;%d%c", ESC, OSC, OSC_SETSIZE, f.rows(), f.cols(), BEL))
}

func (src *framebuffer) diff(dest *framebuffer) []byte {
	var sb strings.Builder

	lastF := defFmt
	lastHL := defOSC8
	lastCur := cursor{-1, -1}

	sz := dest.ansiOSCSize()
	if !slices.Equal(sz, src.ansiOSCSize()) {
		sb.Write(sz)
	}

	for r, row := range dest.data {
		for c, destCell := range row {
			cur := cursor{r, c}
			srcCell, err := src.cell(r, c)
			if err != nil {
				srcCell = defaultCell()
			}

			if destCell.frag != FRAG_SECONDARY && !srcCell.equal(destCell) {
				if cur.row != lastCur.row || cur.col != lastCur.col+1 {
					sb.WriteString(cur.ansiString())
				}

				d := srcCell.efficientDiff(destCell, lastF, lastHL)
				if len(d) == 0 {
					d = []byte(fmt.Sprintf("%c", destCell.r))
				}
				sb.Write(d)
				lastF = destCell.format()
				lastHL = destCell.hyperlink()
				lastCur = cur
			}
		}
	}

	return []byte(sb.String())
}

func (f *framebuffer) copy() *framebuffer {
	rows := f.rows()
	cols := f.cols()

	nf := &framebuffer{
		data: make([][]cell, rows, rows),
	}

	for row := range f.data {
		nf.data[row] = make([]cell, cols, cols)
		copy(nf.data[row], f.row(row))
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
	if f.cols() != other.cols() || f.rows() != other.rows() {
		return false
	}

	for r, row := range other.data {
		for c, cell := range row {
			oc, err := f.cell(r, c)
			if err != nil {
				oc = defaultCell()
			}
			if !cell.equal(oc) {
				return false
			}
		}
	}
	return true
}

func (f *framebuffer) scrollRows(n int) {
	nc := f.cols()
	nr := f.rows()

	if n < 0 {
		// i starts at the bottom, backed up n (negative) so
		// we can copy those rows down. iterate until i <= -n
		// (meaning n rows above 0)
		for i := nr + n - 1; i >= -n-1; i-- {
			x := i - n
			copy(f.data[x], f.data[i])

		}
		for i := 0; i < -n; i++ {
			copy(f.data[i], newRow(nc))
		}
	} else {
		for i := 0; i < nr-n; i++ {
			x := i + n
			if x < nr {
				copy(f.data[i], f.row(x))
			} else {
				copy(f.data[i], newRow(nc))
			}
		}

		for i := nr - n; i < nr; i++ {
			copy(f.data[i], newRow(nc))
		}
	}
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
			c, err := f.cell(i, cols-1)
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
	if from > to || from < 0 || to >= f.rows() {
		return false
	}

	nc := len(f.data[0])
	for i := from; i <= to; i++ {
		row := newRow(nc)
		f.data[i] = row
	}

	return true
}

func (f *framebuffer) setCells(rowFrom, rowTo, colFrom, colTo int, c cell) {
	fr, err := f.subRegion(rowFrom, rowTo, colFrom, colTo)
	if err != nil {
		slog.Debug("couldn't get region to set cells", "err", err)
		return
	}

	fr.fill(c)
}

func newRow(cols int) []cell {
	row := make([]cell, cols, cols)
	for i := 0; i < len(row); i++ {
		row[i] = defaultCell()
	}
	return row
}

func (f *framebuffer) rows() int {
	return len(f.data)
}

func (f *framebuffer) cols() int {
	return len(f.data[0])
}

func (f *framebuffer) validPoint(row, col int) bool {
	if row < 0 || row >= f.rows() || col < 0 || col >= f.cols() {
		return false
	}
	return true
}

func (f *framebuffer) setCell(row, col int, c cell) {
	if f.validPoint(row, col) {
		f.row(row)[col] = c
	}
}

func (f *framebuffer) cell(row, col int) (cell, error) {
	if f.validPoint(row, col) {
		return f.row(row)[col], nil
	}

	return defaultCell(), fmt.Errorf("invalid coordinates (%d, %d): %w", col, row, fbInvalidCell)
}

func (f *framebuffer) row(row int) []cell {
	return f.data[row]
}

var invalidRegion = errors.New("invalid region specification")

func (f *framebuffer) subRegion(t, b, l, r int) (*framebuffer, error) {
	nr := f.rows() - 1
	nc := f.cols() - 1

	if t < 0 || t > b || b > nr || l < 0 || l > r || r > nc {
		return nil, invalidRegion
	}

	sz := b - t + 1
	fb := &framebuffer{
		data: make([][]cell, sz, sz),
	}

	for i := 0; i < sz; i++ {
		if r == nc {
			fb.data[i] = f.data[i+t][l:]
		} else {
			fb.data[i] = f.data[i+t][l : r+1]
		}
	}

	return fb, nil
}

func (f *framebuffer) fill(c cell) {
	for row := range f.data {
		for col := range f.data[row] {
			f.setCell(row, col, c)
		}
	}
}
