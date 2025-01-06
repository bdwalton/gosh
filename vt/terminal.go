package vt

import (
	"log/slog"
	"strings"
)

type terminal struct {
	// Functional members
	p  *parser
	fb *framebuffer

	// State
	title, icon string
	curX, curY  int
	curF        format

	// Temp
	oscTemp []rune
}

func NewTerminal(rows, cols int) *terminal {
	t := &terminal{
		fb:      newFramebuffer(rows, cols),
		oscTemp: make([]rune, 0),
	}
	t.p = newParser(t)

	return t
}

func (t *terminal) handle(action pAction, params []int, data []rune, lastbyte byte) {
	switch action {
	case VTPARSE_ACTION_EXECUTE:
		t.handleExecute(lastbyte)
	case VTPARSE_ACTION_CSI_DISPATCH:
		t.handleCSI(params, data, lastbyte)
	case VTPARSE_ACTION_OSC_PUT, VTPARSE_ACTION_OSC_END:
		t.handleOSC(action, lastbyte)
	}
}

func (t *terminal) handleOSC(act pAction, lastbyte byte) {
	switch act {
	case VTPARSE_ACTION_OSC_PUT:
		t.oscTemp = append(t.oscTemp, rune(lastbyte))
	case VTPARSE_ACTION_OSC_END:
		if len(t.oscTemp) > 0 {
			parts := strings.SplitN(string(t.oscTemp), ";", 2)
			switch parts[0] {
			case "0":
				t.title = parts[1]
				t.icon = parts[1]
			case "1":
				t.icon = parts[1]
			case "2":
				t.title = parts[1]
			}
			t.oscTemp = t.oscTemp[:0]
		}
	}
}

func (t *terminal) print(r rune) {
	t.fb.setCell(t.curY, t.curX, newCell(r, t.curF))
}

func (t *terminal) handleExecute(lastbyte byte) {
	switch lastbyte {
	case CTRL_BEL:
		// just swallow this for now
	case CTRL_BS:
		t.cursorMoveAbs(t.curY, t.curX-1)
	case CTRL_CR:
		t.cursorMoveAbs(t.curY, 0)
	}
}

func (t *terminal) handleCSI(params []int, data []rune, lastbyte byte) {
	switch lastbyte {
	case CSI_EL:
		t.eraseLine(params)
	case CSI_ED:
		t.eraseInDisplay(params)
	case CSI_CUP, CSI_CUD, CSI_CUB, CSI_CUF, CSI_CNL, CSI_CPL, CSI_CHA, CSI_HVP:
		t.cursorMove(params, lastbyte)
	case CSI_SGR:
		t.curF = formatFromParams(t.curF, params)
	default:
		slog.Debug("unimplemented CSI code", "lastbyte", lastbyte, "params", params, "data", data)
	}
}

func (t *terminal) cursorMove(params []int, moveType byte) {
	// No paramter indicates a 0 paramter, but for cursor
	// movement, we always default to 1. That allows more
	// efficient specification of the common movements.
	n := 1
	m := 1
	if len(params) > 0 {
		n = params[0]
	}
	if len(params) > 1 {
		m = params[1]
	}

	row := t.curY
	col := t.curX

	switch moveType {
	case CSI_CUU:
		row -= n
	case CSI_CUD:
		row += n
	case CSI_CUB:
		col -= n
	case CSI_CUF:
		col += n
	case CSI_CNL:
		col = 0
		row += n
	case CSI_CPL:
		col = 0
		row -= n
	case CSI_CHA:
		col = n - 1 // our indexing is zero based
	case CSI_CUP, CSI_HVP: // TODO: What does "format effector" mean for HVP
		row = n - 1 // out indexing is zero based
		col = m - 1
	}

	t.cursorMoveAbs(row, col)
}

func (t *terminal) cursorMoveAbs(row, col int) {
	t.curX = col
	t.curY = row

	switch {
	case t.curX < 0:
		t.curX = 0
	case t.curX >= t.fb.cols:
		t.curX = t.fb.cols - 1
	}

	switch {
	case t.curY < 0:
		t.curY = 0
	case t.curY >= t.fb.rows:
		t.curY = t.fb.rows - 1
	}
}

func (t *terminal) eraseLine(params []int) {
	m := 0
	if len(params) > 0 {
		m = params[0]
	}

	switch m {
	case 0: // to end of line
		t.fb.resetCells(t.curY, t.curX, t.fb.cols)
	case 1: // to start of line
		t.fb.resetCells(t.curY, 0, t.curX)
	case 2: // entire line
		t.fb.resetCells(t.curY, 0, t.fb.cols)
	}
}

func (t *terminal) eraseInDisplay(params []int) {
	m := 0
	if len(params) > 0 {
		m = params[0]
	}

	switch m {
	case 0: // active position to end of screen, inclusive
		t.fb.resetRows(t.curY, t.fb.rows, defFmt)
		t.eraseLine(params)
	case 1: // start of screen to active position, inclusive
		t.fb.resetRows(0, t.curY-1, defFmt)
		t.eraseLine(params)
	case 2: // entire screen
		t.fb = newFramebuffer(t.fb.cols, t.fb.rows)
	}
}
