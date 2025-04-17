package vt

import (
	"log/slog"
)

func (t *Terminal) row() int {
	return t.cur.row
}

func (t *Terminal) col() int {
	return t.cur.col
}

// Move to an absolute column. Param n is assumed to be normalized to
// our 0 indexing by the caller.
func (t *Terminal) cursorCHAorHPA(col int) {
	slog.Debug("horizontal position absolute / horizontal attribute", "col", col)
	t.cursorMoveAbs(t.row(), col)
}

// Move to an absolute position. Param m and n are assumed to be
// normalized to our 0 indexing by the caller.
func (t *Terminal) cursorCUPorHVP(row, col int) {
	// TODO: What does "format effector" mean for HVP
	slog.Debug("horizontal vertical position/cursor position", "row", row, "col", col)
	t.cursorMoveAbs(row, col)
}

func (t *Terminal) cursorHPR(n int) {
	col := t.col() + n // we don't need to be 0 based for this
	slog.Debug("horizontal position relative", "col", col)
	t.cursorMoveAbs(t.row(), col)
}

// Move to an absolute row. Param n is assumed to be normalized to our
// 0 indexing by the caller.
func (t *Terminal) cursorVPA(row int) {
	slog.Debug("vertical position absolute", "row", row)
	t.cursorMoveAbs(row, t.col())
}

func (t *Terminal) cursorVPR(n int) {
	row := t.row() + n // we don't need to be 0 based for this
	slog.Debug("vertical position relative", "row", row)
	t.cursorMoveAbs(row, t.col())
}

func (t *Terminal) cursorCNL(n int) {
	row := t.row() + n
	slog.Debug("next line", "row", row)
	t.cursorMoveAbs(row, 0)
}

func (t *Terminal) cursorCPL(n int) {
	row := t.row() - n
	slog.Debug("previous line", "row", row)
	t.cursorMoveAbs(row, 0)
}

func (t *Terminal) cursorUp(n int) {
	if n == 0 {
		n = 1
	}
	row := t.row()
	top := t.getTopMargin()
	if row < top {
		top = 0
	}

	row -= n
	if row < top {
		row = top
	}

	t.cursorMoveAbs(row, t.col())
}

func (t *Terminal) cursorDown(n int) {
	if n == 0 {
		n = 1
	}
	row := t.row()
	bottom := t.getBottomMargin()
	if row > bottom {
		bottom = t.rows()
	}

	row += n
	if row > bottom {
		row = bottom
	}
	t.cursorMoveAbs(row, t.col())
}

func (t *Terminal) cursorForward(n int) {
	if n == 0 {
		n += 1
	}
	col := t.col()
	right := t.getRightMargin()
	if col > right {
		right = t.cols() - 1
	}

	col += n
	if col > right {
		// TODO: handle wrap
		col = right
	}

	t.cursorMoveAbs(t.row(), col)
}

func (t *Terminal) cursorBack(n int) {
	if n == 0 {
		n += 1
	}

	col := t.col()
	left := t.getLeftMargin()
	if col < left {
		left = 0
	}

	col -= n
	if col < left {
		col = left
	}

	t.cursorMoveAbs(t.row(), col)
}

func (t *Terminal) cursorMoveAbs(row, col int) {
	t.cur.col = col
	t.cur.row = row

	nc := t.cols()
	switch {
	case t.cur.col < 0:
		t.cur.col = 0
	case t.cur.col >= nc:
		t.cur.col = nc - 1
	}

	nr := t.rows()
	// TODO: Fix this
	switch {
	case t.cur.row < 0:
		t.cur.row = 0
	case t.cur.row >= nr:
		t.cur.row = nr - 1
	}
}
