package vt

import (
	"log/slog"
)

func (t *Terminal) rows() int {
	return t.fb.rows()
}

func (t *Terminal) row() int {
	return t.cur.row
}

func (t *Terminal) cols() int {
	return t.fb.cols()
}

func (t *Terminal) col() int {
	return t.cur.col
}

func (t *Terminal) homeCursor() {
	if t.isModeSet(privIDToName[DECOM]) {
		t.cursorMoveAbs(t.topMargin(), t.leftMargin())
	} else {
		t.cursorMoveAbs(0, 0)
	}
}

func (t *Terminal) cursorMove(params *parameters, moveType rune) {
	switch moveType {
	case CSI_HPA, CSI_CHA:
		// expects 0 based indexes when called
		t.cursorCHAorHPA(params.item(0, 1) - 1)
	case CSI_CUP, CSI_HVP:
		// expects 0 based indexes when called
		t.cursorCUPorHVP(params.item(0, 1)-1, params.item(1, 1)-1)
	case CSI_VPA:
		// expects 0 based when called
		t.cursorVPA(params.item(0, 1) - 1)
	case CSI_HPR:
		t.cursorHPR(params.item(0, 1))
	case CSI_VPR:
		t.cursorVPR(params.item(0, 1))
	case CSI_CUU:
		t.cursorUp(params.itemDefaultOneIfZero(0, 1))
	case CSI_CUD:
		t.cursorDown(params.itemDefaultOneIfZero(0, 1))
	case CSI_CUB:
		t.cursorBack(params.itemDefaultOneIfZero(0, 1))
	case CSI_CUF:
		t.cursorForward(params.itemDefaultOneIfZero(0, 1))
	case CSI_CNL:
		t.cursorCNL(params.item(0, 1))
	case CSI_CPL:
		t.cursorCPL(params.item(0, 1))
	}
}

// Move to an absolute column. Param n is assumed to be normalized to
// our 0 indexing by the caller.
func (t *Terminal) cursorCHAorHPA(col int) {
	slog.Debug("horizontal position absolute / horizontal attribute", "col", col)
	if t.isModeSet(privIDToName[DECOM]) {
		col += t.leftMargin()
		if r := t.rightMargin(); col > r {
			col = r
		}
		slog.Debug("adjusting column for ORIGIN MODE", "col", col)
	}

	t.cursorMoveAbs(t.row(), col)
}

// Move to an absolute position. Param row and col are assumed to be
// normalized to our 0 indexing by the caller.
func (t *Terminal) cursorCUPorHVP(row, col int) {
	// TODO: What does "format effector" mean for HVP
	slog.Debug("horizontal vertical position/cursor position", "row", row, "col", col)
	if t.isModeSet(privIDToName[DECOM]) && t.inScrollingRegion() {
		col += t.leftMargin()
		if r := t.rightMargin(); col > r {
			col = r
		}

		row += t.topMargin()
		if b := t.bottomMargin(); row > b {
			row = b
		}
		slog.Debug("adjusting for ORIGIN MODE", "row", row, "col", col)
	}

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
	row := t.row() - n
	if top := t.boundedMarginTop(); row < top {
		row = top
	}
	t.cursorMoveAbs(row, t.col())
}

func (t *Terminal) cursorDown(n int) {
	row := t.row() + n
	if bottom := t.boundedMarginBottom(); row > bottom {
		row = bottom
	}
	t.cursorMoveAbs(row, t.col())
}

func (t *Terminal) cursorForward(n int) {
	col := t.col() + n
	if right := t.boundedMarginRight(); col > right {
		col = right
	}
	t.cursorMoveAbs(t.row(), col)
}

func (t *Terminal) cursorBack(n int) {
	col := t.col() - n
	if left := t.boundedMarginLeft(); col < left {
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
