package vt

import (
	"log/slog"
	"strings"
)

type Terminal struct {
	// Functional members
	p  *parser
	fb *framebuffer

	// State
	title, icon string
	curX, curY  int
	curF        format

	// Temp
	oscTemp []rune

	// CSI private flags
	privAutowrap    bool // default reset (false)
	privNewLineMode bool // default reset (false)
}

func NewTerminal(rows, cols int) *Terminal {
	t := &Terminal{
		fb:      newFramebuffer(rows, cols),
		oscTemp: make([]rune, 0),
	}
	t.p = newParser(t)

	return t
}

func (t *Terminal) Resize(rows, cols int) {
	t.fb.resize(rows, cols)
}

func (t *Terminal) handle(action pAction, params *parameters, data []rune, lastbyte byte) {
	switch action {
	case VTPARSE_ACTION_EXECUTE:
		t.handleExecute(lastbyte)
	case VTPARSE_ACTION_CSI_DISPATCH:
		t.handleCSI(params, data, lastbyte)
	case VTPARSE_ACTION_OSC_PUT, VTPARSE_ACTION_OSC_END:
		t.handleOSC(action, lastbyte)
	}
}

func (t *Terminal) handleOSC(act pAction, lastbyte byte) {
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

func (t *Terminal) print(r rune) {
	t.fb.setCell(t.curY, t.curX, newCell(r, t.curF))
}

func (t *Terminal) handleExecute(lastbyte byte) {
	switch lastbyte {
	case CTRL_BEL:
		// just swallow this for now
	case CTRL_BS:
		t.cursorMoveAbs(t.curY, t.curX-1)
	case CTRL_CR:
		t.cursorMoveAbs(t.curY, 0)
	}
}

func (t *Terminal) handleCSI(params *parameters, data []rune, lastbyte byte) {
	switch lastbyte {
	case CSI_PRIV_ENABLE:
		t.setPriv(params, data, true)
	case CSI_PRIV_DISABLE:
		t.setPriv(params, data, false)
	case CSI_DECSTBM:
		t.setTopBottom(params)
	case CSI_DECSLRM:
		t.setLeftRight(params)
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

func (t *Terminal) setPriv(params *parameters, data []rune, val bool) {
	priv, ok := params.consumeItem()
	if len(data) != 1 || data[0] != '?' || !ok {
		slog.Debug("togglePriv called without ? intermediate or missing params", "data", data, "params", params.items, "enabled?", val)
		return
	}

	switch priv {
	case PRIV_CSI_DECAWM:
		t.privAutowrap = val
	case PRIV_CSI_LNM:
		t.privNewLineMode = val
	default:
		slog.Debug("unimplmented private csi mode", "priv", priv)
	}
}

func (t *Terminal) setTopBottom(params *parameters) {
	nr := t.fb.getRows()
	top, _ := params.getItem(0, 1)
	bottom, _ := params.getItem(1, nr)
	if bottom <= top || top > nr || (top == 0 && bottom == 1) {
		return // matches xterm
	}

	t.fb.setTBScroll(top-1, bottom-1)
}

func (t *Terminal) setLeftRight(params *parameters) {
	nc := t.fb.getCols()
	left, _ := params.getItem(0, 1)
	right, _ := params.getItem(1, nc)
	if right <= left || left >= nc || (left == 0 && right == 1) {
		return // matches xterm
	}

	t.fb.setTBScroll(left-1, right-1)
}

func (t *Terminal) cursorMove(params *parameters, moveType byte) {
	// No paramter indicates a 0 paramter, but for cursor
	// movement, we always default to 1. That allows more
	// efficient specification of the common movements.
	n, _ := params.getItem(0, 1)
	m, _ := params.getItem(1, 1)

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

func (t *Terminal) cursorMoveAbs(row, col int) {
	t.curX = col
	t.curY = row

	nc := t.fb.getCols()
	switch {
	case t.curX < 0:
		t.curX = 0
	case t.curX >= nc:
		t.curX = nc - 1
	}

	nr := t.fb.getRows()
	// TODO: Fix this
	switch {
	case t.curY < 0:
		t.curY = 0
	case t.curY >= nr:
		t.curY = nr - 1
	}
}

func (t *Terminal) eraseLine(params *parameters) {
	m, _ := params.getItem(0, 0)

	nc := t.fb.getCols()
	switch m {
	case 0: // to end of line
		t.fb.resetCells(t.curY, t.curX, nc)
	case 1: // to start of line
		t.fb.resetCells(t.curY, 0, t.curX)
	case 2: // entire line
		t.fb.resetCells(t.curY, 0, nc)
	}
}

func (t *Terminal) eraseInDisplay(params *parameters) {
	m, _ := params.getItem(0, 0)

	nr := t.fb.getRows()
	switch m {
	case 0: // active position to end of screen, inclusive
		t.fb.resetRows(t.curY, nr)
		t.eraseLine(params)
	case 1: // start of screen to active position, inclusive
		t.fb.resetRows(0, t.curY-1)
		t.eraseLine(params)
	case 2: // entire screen
		t.fb.resetRows(0, nr)
	}
}
