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
	row := t.row()
	if t.vertMargin.isSet() {
		mRow := t.vertMargin.getMin()
		// If we're already above the top of the
		// scroll region, just move
		if row < mRow {
			row -= n
			slog.Debug("cursor up, vert margin set, unbounded", "row", row)
		} else {
			row = maxInt(mRow, row-n)
			slog.Debug("cursor up, vert margin set, bounded", "row", row)
		}
	} else {
		row -= n
		slog.Debug("cursor up, no vert margin", "row", row)
	}
	t.cursorMoveAbs(row, t.col())
}

func (t *Terminal) cursorDown(n int) {
	row := t.row()
	if t.vertMargin.isSet() {
		mRow := t.vertMargin.getMax()
		// If we're already below the bottom of the
		// scroll region, just move
		if row > mRow {
			row += n
			slog.Debug("cursor down, vert margin set, unbounded", "row", row)
		} else {
			row = minInt(mRow, row+n)
			slog.Debug("cursor down, vert margin set, bounded", "row", row)
		}
	} else {
		row += n
		slog.Debug("cursor down, no vert margin", "row", row)
	}
	t.cursorMoveAbs(row, t.col())
}

func (t *Terminal) cursorForward(n int) {
	col := t.col()
	if t.horizMargin.isSet() {
		mCol := t.horizMargin.getMax()
		// If we're already right of the scroll
		// region, just move
		if col > mCol {
			col += n
			slog.Debug("cursor forward, horiz margin set, unbounded", "col", col)
		} else {
			col = minInt(mCol, col+n)
			slog.Debug("cursor forward, horiz margin set, bounded", "col", col)
		}
	} else {
		col += n
		slog.Debug("cursor back, no horiz margin", "col", col)
	}
	t.cursorMoveAbs(t.row(), col)
}

func (t *Terminal) cursorBack(n int) {
	col := t.col()
	if t.horizMargin.isSet() {
		mCol := t.horizMargin.getMin()
		// If we're already left of the scroll region,
		// just move
		if col < mCol {
			col -= n
			slog.Debug("cursor back, horiz margin set, unbounded", "col", col)
		} else {
			col = maxInt(mCol, col-n)
			slog.Debug("cursor back, horiz margin set, bounded", "col", col)
		}
	} else {
		col -= n
		slog.Debug("cursor back, no horiz margin", "col", col)
	}
	t.cursorMoveAbs(t.row(), col)
}
