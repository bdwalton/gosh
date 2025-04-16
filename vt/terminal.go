package vt

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/creack/pty"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
	"golang.org/x/text/unicode/norm"
)

type manageFunc func()

type Terminal struct {
	// Functional members
	p  *parser
	fb *framebuffer

	ptyR, ptyW *os.File

	wait, stop manageFunc

	// State
	lastChg               time.Time
	title, icon           string
	savedTitle, savedIcon string
	cur, savedCur         cursor
	curF                  format
	tabs                  []bool

	// Temp
	oscTemp []rune

	// scroll margin/region parameters
	vertMargin, horizMargin margin

	// CSI private flags
	flags map[int]*privFlag

	// Internal
	mux sync.Mutex
}

// Private flags here will be initialized, diff'd, copied, etc.
var privFlags = []int{
	PRIV_CSI_DECCKM,
	PRIV_CSI_DECCOLM,
	PRIV_CSI_DECAWM,
	PRIV_BLINK_CURSOR,
	PRIV_CSI_LNM,
	PRIV_SHOW_CURSOR,
	PRIV_XTERM_80_132_ALLOW,
	PRIV_DISABLE_MOUSE_XY,
	PRIV_DISABLE_MOUSE_HILITE,
	PRIV_DISABLE_MOUSE_MOTION,
	PRIV_DISABLE_MOUSE_ALL,
	PRIV_DISABLE_MOUSE_FOCUS,
	PRIV_DISABLE_MOUSE_UTF8,
	PRIV_DISABLE_MOUSE_SGR,
	PRIV_CSI_BRACKET_PASTE,
}

func newBasicTerminal(r, w *os.File) *Terminal {
	flags := make(map[int]*privFlag)
	for _, f := range privFlags {
		flags[f] = newPrivFlag(f)
	}
	return &Terminal{
		fb:      newFramebuffer(DEF_ROWS, DEF_COLS),
		oscTemp: make([]rune, 0),
		tabs:    makeTabs(DEF_COLS),
		flags:   flags,
		p:       newParser(),
		ptyR:    r,
		ptyW:    w,
		wait:    func() {},
		stop:    func() {},
	}
}

func NewTerminalWithPty(cmd *exec.Cmd, cancel context.CancelFunc) (*Terminal, error) {
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: DEF_ROWS, Cols: DEF_COLS})
	if err != nil {
		return nil, fmt.Errorf("couldn't start pty: %v", err)
	}

	// Any use of Fd(), including indirectly via the Setsize call
	// above, will set the descriptor non-blocking, so we need to
	// change that here.
	pfd := int(ptmx.Fd())
	if err := syscall.SetNonblock(pfd, true); err != nil {
		return nil, fmt.Errorf("couldn't set ptmx non-blocking: %v", err)
	}

	t := newBasicTerminal(ptmx, ptmx)
	t.wait = func() { cmd.Wait() }
	t.stop = func() { cancel() }

	return t, nil
}

func NewTerminal() (*Terminal, error) {
	// On the client end, we will read from the network and ship
	// the diff into the locally running terminal. To do that,
	// we'll ensure we have a local pipe to work through.
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("couldn't open a pipe: %v", err)
	}

	return newBasicTerminal(pr, pw), nil
}

func (t *Terminal) Wait() {
	t.wait()
}

func (t *Terminal) Stop() {
	t.stop()
	t.ptyR.Close() // ensure Run() stops
}

func (t *Terminal) Write(p []byte) (int, error) {
	return t.ptyW.Write(p)
}

func (t *Terminal) Copy() *Terminal {
	t.mux.Lock()
	defer t.mux.Unlock()

	flags := make(map[int]*privFlag)
	for c, f := range t.flags {
		flags[c] = f.copy()
	}

	return &Terminal{
		fb:      t.fb.copy(),
		title:   t.title,
		icon:    t.icon,
		cur:     t.cur,
		curF:    t.curF,
		flags:   flags,
		lastChg: t.lastChg,
	}
}

func (src *Terminal) Diff(dest *Terminal) []byte {
	var sb strings.Builder

	if src.lastChg == dest.lastChg {
		return []byte{}
	}

	if src.title != dest.title || src.icon != dest.icon {
		switch {
		case dest.title == dest.icon:
			sb.WriteString(fmt.Sprintf("%c%c%s;%s%c", ESC, ESC_OSC, OSC_ICON_TITLE, string(dest.title), CTRL_BEL))
		default:
			if src.icon != dest.icon {
				sb.WriteString(fmt.Sprintf("%c%c%s;%s%c", ESC, ESC_OSC, OSC_ICON, string(dest.icon), CTRL_BEL))
			}
			if src.title != dest.title {
				sb.WriteString(fmt.Sprintf("%c%c%s;%s%c", ESC, ESC_OSC, OSC_TITLE, string(dest.title), CTRL_BEL))
			}
		}
	}

	if !src.horizMargin.equal(dest.horizMargin) {
		sb.WriteString(dest.horizMargin.getAnsi(CSI_DECSLRM))
	}

	if !src.vertMargin.equal(dest.vertMargin) {
		sb.WriteString(dest.vertMargin.getAnsi(CSI_DECSTBM))
	}

	for _, c := range privFlags {
		if !src.flags[c].equal(dest.flags[c]) {
			sb.WriteString(dest.flags[c].getAnsiString())
		}
	}

	// we always generate diffs as from previous to current
	fbd := src.fb.diff(dest.fb)
	if len(fbd) > 0 {
		if dest.curF.equal(defFmt) {
			sb.WriteString(FMT_RESET)
		}

		sb.Write(fbd)
		// Always reset the cursor
		sb.WriteString(dest.cur.getMoveToAnsi())

		// We assume that the pen was changed during the
		// writing of the framebuffer diff, so always generate
		// a full format reset for the diff
		sb.Write(defFmt.diff(dest.curF))
	} else {
		// If we didn't write anything, the pen may still be
		// different so we should ship the delta.
		sb.Write(src.curF.diff(dest.curF))
	}

	return []byte(sb.String())
}

func (t *Terminal) Run() {
	rr := bufio.NewReader(t.ptyR)

	for {
		var r rune
		r, sz, err := rr.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if !errors.Is(err, os.ErrDeadlineExceeded) {
				slog.Error("pty ReadRune", "r", r, "sz", sz, "err", err)
				break
			}
			continue
		}

		if r == utf8.RuneError && sz == 1 {
			rr.UnreadRune()
			b, err := rr.ReadByte()
			if err != nil {
				slog.Error("pty ReadByte", "b", b, "err", err)
				continue
			}

			r = rune(b)
		}

		for _, a := range t.p.parse(r) {
			t.mux.Lock()
			t.lastChg = time.Now()
			switch a.act {
			case VTPARSE_ACTION_EXECUTE:
				t.handleExecute(a.r)
			case VTPARSE_ACTION_CSI_DISPATCH:
				t.handleCSI(a.params, a.data, a.r)
			case VTPARSE_ACTION_OSC_START, VTPARSE_ACTION_OSC_PUT, VTPARSE_ACTION_OSC_END:
				t.handleOSC(a.act, a.r)
			case VTPARSE_ACTION_PRINT:
				t.print(a.r)
			case VTPARSE_ACTION_ESC_DISPATCH:
				t.handleESC(a.params, a.data, a.r)
			default:
				slog.Debug("unhandled action", "action", ACTION_NAMES[a.act], "params", a.params, "data", a.data, "rune", a.r)
			}
			t.mux.Unlock()
		}
	}
}

func (t *Terminal) Resize(rows, cols int) {
	pts := &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	}

	if term.IsTerminal(int(t.ptyW.Fd())) {
		if err := pty.Setsize(t.ptyW, pts); err != nil {
			slog.Error("couldn't set size on pty", "err", err)
		}
		// Any use of Fd(), including in the InheritSize call above,
		// will set the descriptor non-blocking, so we need to change
		// that here.
		pfd := int(t.ptyW.Fd())
		if err := syscall.SetNonblock(pfd, true); err != nil {
			slog.Error("couldn't set pty to nonblocking", "err", err)
			return
		}
	}

	t.mux.Lock()
	t.fb.resize(rows, cols)
	t.resizeTabs(cols)
	t.lastChg = time.Now()
	defer t.mux.Unlock()

	slog.Debug("changed window size", "rows", rows, "cols", rows)
}

func (t *Terminal) handleESC(params *parameters, data []rune, r rune) {
	switch r {
	case 'A', 'B', 'C', 'E', 'K', 'Q', 'R', 'Y', 'Z', '2', '4', '6', '>', '=', '`':
		slog.Debug("swallowing ESC character set command", "data", string(data))
	case 'F':
		t.cursorMoveAbs(t.fb.getNumRows()-1, 0)
	case 'H': // set tab stop. note that in some vt dialects this
		// would actually be part of character set handling
		// (swedish on vt220).
		t.tabs[t.cur.col] = true
	case 'M': // move cursor one line up, scrolling if needed
		if t.cur.row == 0 {
			t.fb.scrollRows(-1)
		} else {
			t.cursorMoveAbs(t.cur.row-1, t.cur.col)
		}
	case '7': // save cursor
		t.savedCur = t.cur.Copy()
	case '8': // restore cursor
		t.cur = t.savedCur.Copy()
	case 'c':
		t.reset()
	default:
		slog.Debug("ignoring ESC", "r", string(r), "params", params, "data", string(data))
	}

}

func (t *Terminal) handleOSC(act pAction, last rune) {
	switch act {
	case VTPARSE_ACTION_OSC_START:
		t.oscTemp = make([]rune, 0)
	case VTPARSE_ACTION_OSC_PUT:
		t.oscTemp = append(t.oscTemp, last)
	case VTPARSE_ACTION_OSC_END:
		// https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Operating-System-Commands
		// is a good description of many of the options
		// here. So many of them are completely legacy that we
		// won't implement them here unless it proves to be
		// useful as we gain experience with things in the
		// wild.
		//
		// NOTE: Per the xterm documentation, we're going to
		// steal "X" to mean reasize and expect 2 parameters
		// (rows, cols) as arguments (eg: PS X ; r ; c ST)
		// which will allow us to succinctly pass along window
		// size information from the emulation on the server
		// to the emulation on the client. (The client passes
		// this to the server with a special message, but we
		// prefer to ship it back via "diff" which will be in
		// the form of ANSI code using this capability.)
		if len(t.oscTemp) > 0 {
			slog.Debug("Handling OSC data", "data", string(t.oscTemp))
			parts := strings.SplitN(string(t.oscTemp), ";", 2)
			switch parts[0] {
			case OSC_ICON_TITLE:
				t.title = parts[1]
				t.icon = parts[1]
			case OSC_ICON:
				t.icon = parts[1]
			case OSC_TITLE:
				t.title = parts[1]
			case OSC_SETSIZE: // a Gosh convention
				if len(parts) == 3 {
					for {
						var rows, cols int
						var err error
						if rows, err = strconv.Atoi(parts[1]); err != nil {
							break
						}
						if cols, err = strconv.Atoi(parts[2]); err != nil {
							break
						}

						t.Resize(rows, cols)
						break
					}

				}
			default:
				slog.Error("Unknown OSC entity", "data", string(t.oscTemp))
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

func (t *Terminal) reset() {
	rows, cols := t.fb.getNumRows(), t.fb.getNumCols()
	t.fb = newFramebuffer(rows, cols)
	t.title = ""
	t.icon = ""
	t.cur = cursor{0, 0}
	t.savedCur = cursor{0, 0}
	t.tabs = makeTabs(cols)
	t.vertMargin = newMargin(0, rows-1)
	t.horizMargin = newMargin(0, cols-1)
	flags := make(map[int]*privFlag)
	for _, f := range privFlags {
		flags[f] = newPrivFlag(f)
	}
	t.flags = flags
}

func (t *Terminal) setFlag(code int, val bool) {
	t.flags[code].set(val)
}

func (t *Terminal) getFlag(code int) bool {
	return t.flags[code].get()
}

func (t *Terminal) print(r rune) {
	row, col := t.cur.row, t.cur.col
	rw := runewidth.StringWidth(string(r))

	switch rw {
	case 0: // combining
		if col == 0 && !t.getFlag(PRIV_CSI_DECAWM) {
			// can't do anything with this. if we're in
			// the first position but hadn't wrapped, we
			// don't have something to combine with, so
			// just punt.
			slog.Debug("Punting on 0 width rune", "r", r)
			return
		}

		switch {
		case col == 0 && t.getFlag(PRIV_CSI_DECAWM): // we wrapped
			col = t.fb.getNumCols() - 1
			row -= 1
		case col >= t.fb.getNumCols(): // we're at the end of a row but didn't wrap
			col = t.fb.getNumCols() - 1
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
	default: // default (1 column), wide (2 columns)
		if col <= t.fb.getNumCols()-rw {
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

		if t.getFlag(PRIV_CSI_DECAWM) {
			col = 0
			if row == t.fb.getNumRows()-1 {
				t.fb.scrollRows(1)
			} else {
				row += 1
			}
		} else {
			// overwrite chars at the end
			col = t.fb.getNumRows() - rw
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
	}
}

func (t *Terminal) handleExecute(last rune) {
	switch last {
	case CTRL_BEL:
		// just swallow this for now
	case CTRL_BS:
		t.cursorMoveAbs(t.cur.row, t.cur.col-1)
	case CTRL_CR:
		t.carriageReturn()
	case CTRL_LF, CTRL_FF: // libvte treats lf and ff the same, so we do too
		t.lineFeed()
	case CTRL_TAB:
		t.stepTabs(1)
	default:
		slog.Debug("handleExecute: UNHANDLED Command", "last", string(last))
	}
}

func (t *Terminal) handleCSI(params *parameters, data []rune, last rune) {
	switch last {
	case CSI_DSR:
		t.handleDSR(params, data)
	case CSI_DA:
		t.replyDeviceAttributes(data)
	case CSI_Q_MULTI:
		t.csiQ(params, data)
	case CSI_XTWINOPS:
		t.xtwinops(params)
	case CSI_ICH:
		// Insert n blank characters
		n, _ := params.getItem(0, 1)
		lastCol := t.fb.getNumCols() - 1
		for i := 0; i < n; i++ {
			if t.cur.col == lastCol {
				break
			}
			t.print(' ')
		}
	case CSI_ECH:
		// Insert n blank characters
		n, _ := params.getItem(0, 1)
		last := t.cur.col + n
		if lastCol := t.fb.getNumCols() - 1; last > lastCol {
			last = lastCol
		}
		t.fb.resetCells(t.cur.row, t.cur.col, last, t.curF)
	case CSI_PRIV_ENABLE:
		t.setPriv(params, data, true)
	case CSI_PRIV_DISABLE:
		t.setPriv(params, data, false)
	case CSI_DECSTBM:
		t.setTopBottom(params)
	case CSI_DECSLRM:
		t.setLeftRight(params)
	case CSI_DL:
		t.deleteLines(params)
	case CSI_EL:
		t.eraseLine(params)
	case CSI_SU:
		n, _ := params.getItem(0, 1)
		t.fb.scrollRows(-n)
	case CSI_SD:
		n, _ := params.getItem(0, 1)
		t.fb.scrollRows(n)
	case CSI_ED:
		t.eraseInDisplay(params)
	case CSI_VPA, CSI_VPR, CSI_HPA, CSI_HPR, CSI_CUP, CSI_CUU, CSI_CUD, CSI_CUB, CSI_CUF, CSI_CNL, CSI_CPL, CSI_CHA, CSI_HVP:
		t.cursorMove(params, last)
	case CSI_SGR:
		if string(data) != "" {
			slog.Debug("swallowing xterm specific key modifier set/reset or query", "params", params, "data", string(data))

		} else {
			t.curF = formatFromParams(t.curF, params)
		}
	case CSI_DECST8C:
		t.resetTabs(params, data)
	case CSI_CHT:
		n, _ := params.getItem(0, 1)
		t.stepTabs(n)
	case CSI_CBT:
		n, _ := params.getItem(0, 1)
		t.stepTabs(-n)
	case CSI_TBC:
		t.clearTabs(params)
	default:
		slog.Debug("unimplemented CSI code", "last", string(last), "params", params, "data", string(data))
	}
}

func (t *Terminal) resetTabs(params *parameters, data []rune) {
	n, ok := params.getItem(0, 0)
	if len(data) != 1 || data[0] != '?' || !ok || n != 5 {
		slog.Debug("resetTabs called without ? 5 as data and parameter", "data", string(data), "params", params)
	}
	cols := t.fb.getNumCols()
	tabs := make([]bool, cols, cols)
	for i := 0; i < cols; i += 8 {
		tabs[i] = true
	}
	t.tabs = tabs
}

func (t *Terminal) clearTabs(params *parameters) {
	m, _ := params.getItem(0, 0)
	switch m {
	case TBC_CUR:
		t.tabs[t.cur.col] = false
	case TBC_ALL:
		for i := range t.tabs {
			t.tabs[i] = false
		}
	}
}

func (t *Terminal) carriageReturn() {
	nc := 0
	if c := t.horizMargin.getMin(); t.horizMargin.isSet() && t.cur.col > c {
		nc = c
	}

	t.cursorMoveAbs(t.cur.row, nc)
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

func (t *Terminal) xtwinops(params *parameters) {
	slog.Debug("handling xtwinops", "params", params)
	cmd, _ := params.getItem(0, 0)
	switch cmd {
	case 0:
		slog.Debug("invalid xtwinops command (0)")
	case 22: // save title and icon
		t.savedTitle = t.title
		t.savedIcon = t.icon
	case 23: // restore title and icon
		t.title = t.savedTitle
		t.icon = t.savedIcon
	}
}

func (t *Terminal) csiQ(params *parameters, data []rune) {
	switch string(data) {
	case ">":
		if n, _ := params.getItem(0, 0); n != 0 {
			slog.Debug("invalid xterm_version query", "params", params, "data", string(data))
			return
		}
		r := fmt.Sprintf("%c%c>|gosh(%s)%c%c", ESC, ESC_DCS, GOSH_VT_VER, ESC, ESC_ST)
		t.Write([]byte(r))
		slog.Debug("identifying as gosh version", "ver", GOSH_VT_VER)
	default:
		slog.Debug("unhandled CSI q", "params", params, "data", string(data))
	}
}

func (t *Terminal) handleDSR(params *parameters, data []rune) {
	n, _ := params.getItem(0, 0)
	switch string(data) {
	case "": // General device status report
		switch n {
		case 5: // We always report OK (CSI 0 n)
			t.Write([]byte(fmt.Sprintf("%c%c0%c", ESC, ESC_CSI, CSI_DSR)))
		case 6: // Provide cursor location (CSI r ; c R)
			t.Write([]byte(fmt.Sprintf("%c%c%d;%dR", ESC, ESC_CSI, t.cur.row+1, t.cur.col+1)))
		}
	case "?": // DEC specific device status report
		switch n {
		case 6: // Provide cursor location (CSI ? r ; c R)

			t.Write([]byte(fmt.Sprintf("%c%c?%d;%dR", ESC, ESC_CSI, t.cur.row+1, t.cur.col+1)))
		case 15: // report printer status; always "not ready" (CSI ? 1 1 n)
			t.Write([]byte(fmt.Sprintf("%c%c?11%c", ESC, ESC_CSI, CSI_DSR)))
		default:
			slog.Debug("swallowing CSI ? DSR code", "params", params, "data", string(data))
		}
	case ">":
		slog.Debug("swallowing xterm disable key modifiers", "params", params, "data", string(data))
	default:
		slog.Debug("unknown CSI DSR modifier string", "params", params, "data", data)
	}
}

func (t *Terminal) replyDeviceAttributes(data []rune) {
	switch string(data) {
	case "=": // teritatary attributes
		slog.Debug("ignoring request for tertiary device attributes")
	case ">": // secondary attributes
		t.Write([]byte("\033[>1;10;0c")) // vt220
		slog.Debug("identifying secondary attributes as a vt220")
	case "": // primary attributes
		t.Write([]byte("\033[?62c")) // vt220
		slog.Debug("identifying primary attributes as a vt220")
	default:
		slog.Debug("Unexpected CSI device attributes request")
	}
}

func (t *Terminal) setPriv(params *parameters, data []rune, val bool) {
	priv, ok := params.consumeItem()
	if len(data) != 1 || data[0] != '?' || !ok {
		slog.Debug("setPriv called without ? intermediate or missing params", "data", data, "params", params.items, "enabled?", val)
		return
	}

	if _, ok := t.flags[priv]; ok {
		t.setFlag(priv, val)
	} else {
		slog.Debug("unimplmented private csi mode", "priv", priv)
	}
}

func (t *Terminal) setTopBottom(params *parameters) {
	nr := t.fb.getNumRows()
	top, _ := params.getItem(0, 1)
	bottom, _ := params.getItem(1, nr)
	if bottom <= top || top > nr || (top == 0 && bottom == 1) {
		return // matches xterm
	}

	// https://vt100.net/docs/vt510-rm/DECSTBM.html
	// STBM sets the cursor to 1,1 (0,0)
	t.vertMargin = newMargin(top-1, bottom-1)
	slog.Debug("set top/bottom margin", "margin", t.vertMargin)
	t.cursorMoveAbs(0, 0)
}

func (t *Terminal) setLeftRight(params *parameters) {
	nc := t.fb.getNumCols()
	left, _ := params.getItem(0, 1)
	right, _ := params.getItem(1, nc)
	if right <= left || left >= nc || (left == 0 && right == 1) {
		return // matches xterm
	}

	// https://vt100.net/docs/vt510-rm/DECSLRM.html
	// STBM sets the cursor to 1,1 (0,0)
	t.horizMargin = newMargin(left-1, right-1)
	slog.Debug("set left/right margin", "margin", t.horizMargin)
	t.cursorMoveAbs(0, 0)
}

func minInt(i1, i2 int) int {
	if i1 <= i2 {
		return i1
	}
	return i2
}

func maxInt(i1, i2 int) int {
	if i1 >= i2 {
		return i1
	}
	return i2
}

func (t *Terminal) cursorInScrollingRegion() bool {
	return t.horizMargin.isSet() &&
		t.vertMargin.isSet() &&
		t.horizMargin.contains(t.cur.row) &&
		t.vertMargin.contains(t.cur.col)
}

func (t *Terminal) cursorMove(params *parameters, moveType rune) {
	// No paramter indicates a 0 paramter, but for cursor
	// movement, we always default to 1. That allows more
	// efficient specification of the common movements.
	n, _ := params.getItem(0, 1)
	m, _ := params.getItem(1, 1)

	row := t.cur.row
	col := t.cur.col

	switch moveType {
	case CSI_HPA:
		col = n - 1 // 0 based columns
		slog.Debug("cursor horizontal position absolute", "col", col)
	case CSI_HPR:
		col += n // we don't need to be 0 based for this
		slog.Debug("cursor horizontal position relative", "col", col)
	case CSI_VPA:
		row = n - 1 // 0 based rows
		slog.Debug("cursor vertical position absolute", "row", row)
	case CSI_VPR:
		row += n // we don't need to be 0 based for this
		slog.Debug("cursor vertical position relative", "row", row)
	case CSI_CUU:
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
	case CSI_CUD:
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
	case CSI_CUB:
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
	case CSI_CUF:
		if t.horizMargin.isSet() {
			mCol := t.horizMargin.getMax()
			// If we're already right of the scroll
			// region, just move
			if col > mCol {
				col += n
				slog.Debug("cursor back, horiz margin set, unbounded", "col", col)
			} else {
				col = minInt(mCol, col+n)
				slog.Debug("cursor back, horiz margin set, bounded", "col", col)
			}
		} else {
			col += n
			slog.Debug("cursor back, no horiz margin", "col", col)
		}
	case CSI_CNL:
		col = 0
		row += n
		slog.Debug("cursor next line", "row", row, "col", col)
	case CSI_CPL:
		col = 0
		row -= n
		slog.Debug("cursor previous line", "row", row, "col", col)
	case CSI_CHA:
		col = n - 1 // our indexing is zero based
		slog.Debug("cursor horiztonal attribute", "col", col)
	case CSI_CUP, CSI_HVP: // TODO: What does "format effector" mean for HVP
		row = n - 1 // our indexing is zero based
		col = m - 1
		slog.Debug("horizontal vertical position/cursor position", "row", row, "col", col)
	}

	t.cursorMoveAbs(row, col)
}

func (t *Terminal) cursorMoveAbs(row, col int) {
	t.cur.col = col
	t.cur.row = row

	nc := t.fb.getNumCols()
	switch {
	case t.cur.col < 0:
		t.cur.col = 0
	case t.cur.col >= nc:
		t.cur.col = nc - 1
	}

	nr := t.fb.getNumRows()
	// TODO: Fix this
	switch {
	case t.cur.row < 0:
		t.cur.row = 0
	case t.cur.row >= nr:
		t.cur.row = nr - 1
	}
}

func makeTabs(cols int) []bool {
	tabs := make([]bool, cols, cols)
	for i := range tabs {
		tabs[i] = (i%8 == 0)
	}
	return tabs
}

func (t *Terminal) resizeTabs(cols int) {
	l := len(t.tabs)
	switch {
	case cols < l:
		t.tabs = t.tabs[0 : cols-1]
	case cols > l:
		tabs := makeTabs(cols)
		copy(tabs, t.tabs)
		t.tabs = tabs
	}
}

func (t *Terminal) stepTabs(steps int) {
	// column under consideration, step increment for next column,
	// count increment to know when we've tabbed enough.
	col, step, inc := t.cur.col+1, 1, -1

	switch {
	case steps == 0:
		// shouldn't happen, but don't adjust cursor if it
		// does
		return
	case steps < 0:
		// we're moving backward through the line, not forward
		col = t.cur.col - 1
		step = -1
		inc = 1
	}

	max := t.fb.getNumCols() - 1
	for {
		switch {
		case col <= 0:
			t.cur.col = 0
			return
		case col >= max:
			t.cur.col = max
			return
		default:
			if t.tabs[col] {
				steps += inc
				if steps == 0 {
					t.cur.col = col
					return
				}
			}
			col += step
		}
	}
}

func (t *Terminal) deleteLines(params *parameters) {
	m, _ := params.getItem(0, 1)
	cols := t.fb.getNumCols()

	for i := t.cur.row; i < t.cur.row+m && t.vertMargin.contains(i); i++ {
		t.fb.data[i] = newRow(cols)
	}
}

func (t *Terminal) eraseLine(params *parameters) {
	m, _ := params.getItem(0, 0)

	// TODO: Handle BCE properly
	nc := t.fb.getNumCols()
	switch m {
	case 0: // to end of line
		t.fb.resetCells(t.cur.row, t.cur.col, nc, t.curF)
		slog.Debug("erase in line, pos to end", "row", t.cur.row, "col", t.cur.col)
	case 1: // to start of line
		t.fb.resetCells(t.cur.row, 0, t.cur.col, t.curF)
		slog.Debug("erase in line, start of line to pos", "row", t.cur.row, "col", t.cur.col)
	case 2: // entire line
		t.fb.resetCells(t.cur.row, 0, nc, t.curF)
		slog.Debug("erase in line, entire line", "row", t.cur.row, "col", t.cur.col)
	}
}

func (t *Terminal) eraseInDisplay(params *parameters) {
	m, _ := params.getItem(0, 0)

	// TODO: Handle BCE properly
	nr := t.fb.getNumRows()
	switch m {
	case 0: // active position to end of screen, inclusive
		t.fb.resetRows(t.cur.row+1, nr-1)
		t.eraseLine(params)
		slog.Debug("CSI erase in display, pos to end of screen")
	case 1: // start of screen to active position, inclusive
		t.fb.resetRows(0, t.cur.row-1)
		t.eraseLine(params)
		slog.Debug("CSI erase in display, beginning of screen to pos")
	case 2: // entire screen
		t.fb.resetRows(0, nr-1)
		slog.Debug("CSI erase in display, entire screen")
	}
}
