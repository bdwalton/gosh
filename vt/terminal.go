package vt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"golang.org/x/text/unicode/norm"
)

type cursor struct {
	row, col int
}

func (c cursor) equal(other cursor) bool {
	return c.row == other.row && c.col == other.col
}

func (c cursor) String() string {
	return fmt.Sprintf("(%d, %d)", c.row, c.col)
}

type Terminal struct {
	// Functional members
	p  *parser
	fb *framebuffer

	ptyIO io.Reader

	// State
	title, icon string
	cur         cursor
	curF        format

	// Temp
	oscTemp []rune

	// CSI private flags
	privAutowrap    bool // default reset (false)
	privNewLineMode bool // default reset (false)
}

func NewTerminal(pio io.Reader, rows, cols int) *Terminal {
	t := &Terminal{
		fb:      newFramebuffer(rows, cols),
		oscTemp: make([]rune, 0),
		ptyIO:   pio,
	}
	t.p = newParser()

	return t
}

func (t *Terminal) Run() {
	rr := bufio.NewReader(t.ptyIO)

	for {
		r, sz, err := rr.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if !errors.Is(err, os.ErrDeadlineExceeded) {
				slog.Error("pty ReadRune", "r", r, "sz", sz, "err", err)
			}
			continue
		}
		if r == utf8.RuneError && sz == 1 {
			rr.UnreadRune()
			b, err := rr.ReadByte()
			if err != nil {
				slog.Error("pty ReadByte", "b", b, "err", err)
				continue

			} else {
				t.parseByte(b)
			}
		} else {
			t.parseRune(r)
		}
	}
}

func (t *Terminal) parseByte(b byte) {
	for _, a := range t.p.parseByte(b) {
		switch a.act {
		case VTPARSE_ACTION_EXECUTE:
			t.handleExecute(a.b)
		case VTPARSE_ACTION_CSI_DISPATCH:
			t.handleCSI(a.params, a.data, a.b)
		case VTPARSE_ACTION_OSC_PUT, VTPARSE_ACTION_OSC_END:
			t.handleOSC(a.act, a.b)
		}
	}
}

func (t *Terminal) parseRune(r rune) {
	for _, a := range t.p.parseRune(r) {
		switch a.act {
		case VTPARSE_ACTION_EXECUTE:
			t.handleExecute(a.b)
		case VTPARSE_ACTION_CSI_DISPATCH:
			t.handleCSI(a.params, a.data, a.b)
		case VTPARSE_ACTION_OSC_PUT, VTPARSE_ACTION_OSC_END:
			t.handleOSC(a.act, a.b)
		}
	}
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

// clearFrags will ensure we never leave a dangling fragment when we
// write to a cell. If the row and column to be written to is part of
// a fragment, it will clear the previous or next cell, depending on
// whether the current cell is the primary or secondary piece of the
// fragment.
func (t *Terminal) clearFrags(row, col int) {
	if gc, err := t.fb.getCell(row, col); err == nil {
		switch gc.frag {
		case FRAG_PRIMARY: // primary cell
			t.fb.setCell(row, col+1, defaultCell())
		case FRAG_SECONDARY: // secondary/empty cell
			t.fb.setCell(row, col-1, defaultCell())
		}
	}
}

func (t *Terminal) print(r rune) {
	row, col := t.cur.row, t.cur.col
	rw := runewidth.StringWidth(string(r))

	switch rw {
	case 0: // combining
		if col == 0 && !t.privAutowrap {
			// can't do anything with this. if we're in
			// the first position but hadn't wrapped, we
			// don't have something to combine with, so
			// just punt.
			return
		}

		switch {
		case col == 0 && t.privAutowrap: // we wrapped
			col = t.fb.getCols() - 1
			row -= 1
		case col >= t.fb.getCols(): // we're at the end of a row but didn't wrap
			col = t.fb.getCols() - 1
		default:
			col -= 1
		}
		c, err := t.fb.getCell(row, col)
		if err != nil {
			slog.Debug("couldn't fetch cell", "row", row, "col", col)
			return
		} else {
			n := norm.NFC.String(string(c.r) + string(r))
			c.r = []rune(n)[0]
			// TODO: Update format here too? Possible that
			// an escape sequence updated the pen between
			// the combining characters...if a bit daft.
		}

		t.fb.setCell(row, col, c)
	case 1, 2: // default (1 colume), wide (2 columns)
		if col <= t.fb.getCols()-rw {
			t.clearFrags(row, col)
			nc := newCell(r, t.curF)

			if rw > 1 {
				// Clear adjacent cells and note fragments
				t.fb.setCell(row, col+1, fragCell(0, t.curF, FRAG_SECONDARY))
				nc.frag = FRAG_PRIMARY
			}

			t.fb.setCell(row, col, nc)
			t.cur.col += rw
			return
		}

		if t.privAutowrap {
			col = 0
			if row == t.fb.getRows()-1 {
				t.fb.scrollRows(1)
			} else {
				row += 1
			}
		} else {
			// overwrite chars at the end
			col = t.fb.getRows() - rw
		}

		t.clearFrags(row, col)
		nc := newCell(r, t.curF)

		if rw > 1 {
			// Clear adjacent cells and note fragments
			t.fb.setCell(row, col+1, fragCell(0, t.curF, FRAG_SECONDARY))
			nc.frag = FRAG_PRIMARY
		}

		t.fb.setCell(row, col, nc)

		t.cur.col = col + rw
		t.cur.row = row

		// punt, otherwise
		return
	default:
		// We could panic, but since we punt on other unknowns
		// here, let's do that for a wonky width, too.
		return
	}
}

func (t *Terminal) handleExecute(lastbyte byte) {
	switch lastbyte {
	case CTRL_BEL:
		// just swallow this for now
	case CTRL_BS:
		t.cursorMoveAbs(t.cur.row, t.cur.col-1)
	case CTRL_CR:
		t.cursorMoveAbs(t.cur.row, 0)
	case CTRL_LF, CTRL_FF: // libvte treats lf and ff the same, so we do too
		t.lineFeed()

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

func (t *Terminal) lineFeed() {
	if !t.fb.validPoint(t.cur.row+1, t.cur.col) {
		// Add new row, but keep cursor in the same position
		// TODO: fill the new row with BCE color?
		t.fb.scrollRows(1)
	} else {
		t.cursorMoveAbs(t.cur.row+1, t.cur.col)
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

	row := t.cur.row
	col := t.cur.col

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
	t.cur.col = col
	t.cur.row = row

	nc := t.fb.getCols()
	switch {
	case t.cur.col < 0:
		t.cur.col = 0
	case t.cur.col >= nc:
		t.cur.col = nc - 1
	}

	nr := t.fb.getRows()
	// TODO: Fix this
	switch {
	case t.cur.row < 0:
		t.cur.row = 0
	case t.cur.row >= nr:
		t.cur.row = nr - 1
	}
}

func (t *Terminal) eraseLine(params *parameters) {
	m, _ := params.getItem(0, 0)

	nc := t.fb.getCols()
	switch m {
	case 0: // to end of line
		t.fb.resetCells(t.cur.row, t.cur.col, nc)
	case 1: // to start of line
		t.fb.resetCells(t.cur.row, 0, t.cur.col)
	case 2: // entire line
		t.fb.resetCells(t.cur.row, 0, nc)
	}
}

func (t *Terminal) eraseInDisplay(params *parameters) {
	m, _ := params.getItem(0, 0)

	nr := t.fb.getRows()
	switch m {
	case 0: // active position to end of screen, inclusive
		t.fb.resetRows(t.cur.row, nr)
		t.eraseLine(params)
	case 1: // start of screen to active position, inclusive
		t.fb.resetRows(0, t.cur.row-1)
		t.eraseLine(params)
	case 2: // entire screen
		t.fb.resetRows(0, nr)
	}
}
