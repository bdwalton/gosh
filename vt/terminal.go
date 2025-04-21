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
	"slices"
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
	curF, savedF          format
	tabs                  []bool

	// Temp
	oscTemp []rune

	// scroll margin/region parameters
	vertMargin, horizMargin margin

	// Modes that have been set or reset
	modes map[string]*mode

	// Internal
	mux sync.Mutex
}

func newBasicTerminal(r, w *os.File) *Terminal {
	modes := make(map[string]*mode)
	for name, id := range privModeToID {
		modes[name] = newPrivMode(id)
	}
	for name, id := range pubModeToID {
		modes[name] = newMode(id)
	}
	return &Terminal{
		fb:      newFramebuffer(DEF_ROWS, DEF_COLS),
		oscTemp: make([]rune, 0),
		tabs:    makeTabs(DEF_COLS),
		modes:   modes,
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

	modes := make(map[string]*mode)
	for name, m := range t.modes {
		modes[name] = m.copy()
	}

	return &Terminal{
		fb:          t.fb.copy(),
		title:       t.title,
		icon:        t.icon,
		cur:         t.cur,
		curF:        t.curF,
		modes:       modes,
		lastChg:     t.lastChg,
		vertMargin:  t.vertMargin,
		horizMargin: t.horizMargin,
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
			sb.WriteString(fmt.Sprintf("%c%c%s;%s%c", ESC, OSC, OSC_ICON_TITLE, string(dest.title), BEL))
		default:
			if src.icon != dest.icon {
				sb.WriteString(fmt.Sprintf("%c%c%s;%s%c", ESC, OSC, OSC_ICON, string(dest.icon), BEL))
			}
			if src.title != dest.title {
				sb.WriteString(fmt.Sprintf("%c%c%s;%s%c", ESC, OSC, OSC_TITLE, string(dest.title), BEL))
			}
		}
	}

	if !src.horizMargin.equal(dest.horizMargin) {
		sb.WriteString(dest.horizMargin.getAnsi(CSI_DECSLRM))
	}

	if !src.vertMargin.equal(dest.vertMargin) {
		sb.WriteString(dest.vertMargin.getAnsi(CSI_DECSTBM))
	}

	modeNames := make([]string, len(src.modes), len(src.modes))
	var i int
	for n := range src.modes {
		modeNames[i] = n
		i += 1
	}
	slices.Sort(modeNames)

	for _, name := range modeNames {
		if !src.modes[name].equal(dest.modes[name]) {
			sb.WriteString(dest.modes[name].getAnsiString())
		}
	}

	// we always generate diffs as from previous to current
	fbd := src.fb.diff(dest.fb)
	if len(fbd) > 0 {
		if dest.curF.equal(defFmt) {
			sb.WriteString(FMT_RESET)
		}

		sb.Write(fbd)

		// We assume that the pen was changed during the
		// writing of the framebuffer diff, so always generate
		// a full format reset for the diff
		sb.Write(defFmt.diff(dest.curF))
	} else {
		// If we didn't write anything, the pen may still be
		// different so we should ship the delta.
		sb.Write(src.curF.diff(dest.curF))
	}

	if len(fbd) > 0 || !src.cur.equal(dest.cur) {
		sb.WriteString(dest.cur.getMoveToAnsi())
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

		slog.Debug("parsing rune", "r", string(r), "cur", t.cur)
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

func (t *Terminal) cols() int {
	return t.fb.getNumCols()
}

func (t *Terminal) rows() int {
	return t.fb.getNumRows()
}

func (t *Terminal) getLeftMargin() int {
	if t.horizMargin.isSet() {
		return t.horizMargin.getMin()
	}
	return 0
}

func (t *Terminal) getRightMargin() int {
	if t.horizMargin.isSet() {
		return t.horizMargin.getMax()
	}
	return t.cols() - 1
}

func (t *Terminal) getTopMargin() int {
	if t.vertMargin.isSet() {
		return t.vertMargin.getMin()
	}
	return 0
}

func (t *Terminal) getBottomMargin() int {
	if t.vertMargin.isSet() {
		return t.vertMargin.getMax()
	}
	return t.rows() - 1
}

func (t *Terminal) handleESC(params *parameters, data []rune, r rune) {
	dstr := string(data)
	switch r {
	case 'A', 'B', 'C', 'K', 'Q', 'R', 'Y', 'Z', '2', '4', '6', '>', '=', '`':
		slog.Debug("swallowing ESC character set command", "params", params, "data", string(data), "cmd", string(r))
	case NEL:
		max := t.rows() - 1
		if t.inScrollingRegion() {
			max = t.getBottomMargin()
			if t.cur.row == max {
				t.scrollRegion(1)
			} else {
				t.cursorMoveAbs(t.cur.row+1, t.getLeftMargin())
			}
		} else {
			if t.cur.row == max {
				t.scrollAll(1)
			} else {
				t.cursorMoveAbs(t.cur.row+1, 0)
			}
		}
	case 'F':
		if t.inScrollingRegion() {
			t.cursorMoveAbs(t.rows()-1, t.getLeftMargin())
		} else {
			t.cursorMoveAbs(t.rows()-1, 0)
		}
	case HTS: // set tab stop. note that in some vt dialects this
		// would actually be part of character set handling
		// (swedish on vt220).
		t.tabs[t.cur.col] = true
	case IND: // move cursor one line down, scrolling if needed
		max := t.rows() - 1
		if t.inScrollingRegion() {
			max = t.getBottomMargin()
			if t.cur.row == max {
				t.scrollRegion(1)
			} else {
				t.cursorMoveAbs(t.cur.row+1, t.cur.col)
			}
		} else {
			if t.cur.row == max {
				t.scrollAll(1)
			} else {
				t.cursorMoveAbs(t.cur.row+1, t.cur.col)
			}
		}
	case RI: // move cursor one line up, scrolling if needed
		min := 0
		if t.inScrollingRegion() {
			min = t.getTopMargin()
			if t.cur.row == min {
				t.scrollRegion(-1)
			} else {
				t.cursorMoveAbs(t.cur.row-1, t.cur.col)
			}
		} else {
			if t.cur.row == min {
				t.scrollAll(-11)
			} else {
				t.cursorMoveAbs(t.cur.row-1, t.cur.col)
			}
		}
	case DECSC: // save cursor
		t.savedCur = t.cur.Copy()
		t.savedF = t.curF
		slog.Debug("saved cursor and format", "save", t.savedCur)
	case DECRC: // restore cursor or decaln screen test
		switch dstr {
		case "":
			slog.Debug("restoring cursor and format", "was", t.cur, "now", t.savedCur)
			t.cur = t.savedCur.Copy()
			t.curF = t.savedF
		case "#": // DECALN vt100 screen test
			t.doDECALN()
		}
	case RIS:
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
	rows, cols := t.rows(), t.cols()
	t.fb = newFramebuffer(rows, cols)
	t.title = ""
	t.icon = ""
	t.homeCursor()
	t.savedCur = cursor{0, 0}
	t.tabs = makeTabs(cols)
	t.vertMargin = newMargin(0, rows-1)
	t.horizMargin = newMargin(0, cols-1)
	modes := make(map[string]*mode)
	for name, n := range privModeToID {
		modes[name] = newPrivMode(n)
	}
	for name, n := range pubModeToID {
		modes[name] = newMode(n)
	}
	t.modes = modes
}

func (t *Terminal) isModeSet(name string) bool {
	m, ok := t.modes[name]
	if !ok {
		slog.Debug("asked if unknown mode was set", "mode", name)
		return false
	}
	return m.get()
}

func (t *Terminal) print(r rune) {
	row, col := t.row(), t.col()
	rw := runewidth.StringWidth(string(r))

	switch rw {
	case 0: // combining
		if col == 0 && !t.isModeSet(privIDToName[DECAWM]) {
			// can't do anything with this. if we're in
			// the first position but hadn't wrapped, we
			// don't have something to combine with, so
			// just punt.
			slog.Debug("Punting on 0 width rune", "r", r)
			return
		}

		switch {
		case col == 0 && t.isModeSet(privIDToName[DECAWM]): // we wrapped
			col = t.cols() - 1
			row -= 1
		case col >= t.cols(): // we're at the end of a row but didn't wrap
			col = t.cols() - 1
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
		if col <= t.cols()-rw {
			t.clearFrags(row, col)
			nc := newCell(r, t.curF)

			ins := t.isModeSet(pubIDToName[IRM])
			if ins {
				right := t.cols()
				if t.inScrollingRegion() {
					right = t.getRightMargin()
				}
				r, c := t.row(), t.col()
				cells, err := t.fb.getRegion(r, r, c, right)
				if err != nil {
					slog.Debug("invalid framebuffer region", "r", r, "c", c, "right", right, "err", err)
					return
				}
				last := cells.getNumCols() - 1
				if last > rw {
					for i := last; i >= rw; i-- {
						c, err := cells.getCell(0, i-rw)
						if err != nil {
							slog.Debug("invalid cell in region", "row", 0, "col", i-1, "err", err)
							return
						}
						cells.setCell(0, i, c)
					}
				} else {
					cells.fill(newCell(' ', t.curF))
				}
			}

			if rw > 1 {
				// Clear adjacent cells and note fragments
				t.fb.setCell(row, col+1, fragCell(0, t.curF, FRAG_SECONDARY))
				nc.frag = FRAG_PRIMARY
			}

			t.fb.setCell(row, col, nc)

			if !ins {
				t.cur.col += rw
			}

			return
		}

		if t.isModeSet(privIDToName[DECAWM]) {
			if t.inScrollingRegion() {
				col = t.getLeftMargin()
				if row == t.getBottomMargin() {
					t.scrollRegion(1)
				} else {
					row += 1
				}
			} else {
				col = 0
				if row == t.rows() {
					t.scrollAll(1)
				} else {
					row += 1
				}
			}
		} else {
			// overwrite chars at the end
			col = t.cols() - rw
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
	case BEL:
		// just swallow this for now
	case BS:
		t.cursorMoveAbs(t.cur.row, t.cur.col-1)
	case CR:
		t.carriageReturn()
	case LF, FF: // libvte treats lf and ff the same, so we do too
		t.lineFeed()
	case TAB:
		t.stepTabs(1)
	case VT:
		t.cursorDown(1)
	case SO, SI:
		slog.Debug("swallowing charset switching command", "cmd", string(last))
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
	case CSI_DCH:
		t.deleteChars(params, data)
	case CSI_ICH:
		// Insert n blank characters
		n := params.getItem(0, 1)
		lastCol := t.cols() - 1
		for i := 0; i < n; i++ {
			if t.cur.col == lastCol {
				break
			}
			t.print(' ')
		}
	case CSI_ECH:
		// Insert n blank characters where n is the provided parameter
		last := t.cur.col + params.getItem(0, 1)
		if lastCol := t.cols() - 1; last > lastCol {
			last = lastCol
		}
		t.fb.setCells(t.cur.row, t.cur.row, t.cur.col, last, newCell(' ', t.curF))
	case CSI_MODE_SET, CSI_MODE_RESET:
		t.setMode(params.getItem(0, 0), string(data), last)
	case CSI_DECSTBM:
		t.setTopBottom(params)
	case CSI_DECSLRM:
		t.setLeftRight(params)
	case CSI_DL:
		t.deleteLines(params)
	case CSI_EL:
		t.eraseLine(params)
	case CSI_SU:
		n := params.getItem(0, 1)
		if t.inScrollingRegion() {
			t.scrollRegion(-n)
		} else {
			t.scrollAll(-n)
		}
	case CSI_SD:
		n := params.getItem(0, 1)
		if t.inScrollingRegion() {
			t.scrollRegion(n)
		} else {
			t.scrollAll(n)
		}
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
		t.stepTabs(params.getItem(0, 1))
	case CSI_CBT:
		t.stepTabs(-params.getItem(0, 1))
	case CSI_TBC:
		t.clearTabs(params)
	default:
		slog.Debug("unimplemented CSI code", "last", string(last), "params", params, "data", string(data))
	}
}

func (t *Terminal) resetTabs(params *parameters, data []rune) {
	if len(data) != 1 || data[0] != '?' || params.getItem(0, 0) != 5 {
		slog.Debug("resetTabs called without ? 5 as data and parameter", "data", string(data), "params", params)
	}
	cols := t.cols()
	tabs := make([]bool, cols, cols)
	for i := 0; i < cols; i += 8 {
		tabs[i] = true
	}
	t.tabs = tabs
}

func (t *Terminal) clearTabs(params *parameters) {
	switch params.getItem(0, 0) {
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
	if t.inScrollingRegion() {
		nc = t.getLeftMargin()
	}
	t.cursorMoveAbs(t.cur.row, nc)
}

func (t *Terminal) inScrollingRegion() bool {
	if t.vertMargin.contains(t.cur.row) && t.horizMargin.contains(t.cur.col) {
		return true
	}
	return false
}

func (t *Terminal) getScrollingRegion() (*framebuffer, error) {
	return t.fb.getRegion(t.getTopMargin(), t.getBottomMargin(), t.getLeftMargin(), t.getRightMargin())
}

func (t *Terminal) lineFeed() {
	var err error
	fb := t.fb
	cur := t.cur
	if t.inScrollingRegion() {
		// adjust cursor so it is relative to top margin
		cur.row -= t.getTopMargin()
		cur.col -= t.getLeftMargin()
		slog.Debug("in scrolling region, adjusting cursor", "cur", cur, "orig", t.cur)
		fb, err = t.getScrollingRegion()
		if err != nil {
			slog.Debug("error obtaining framebuffer region", "err", err)
			return
		}
	}

	if !fb.validPoint(cur.row+1, cur.col) {
		// Add new row, but keep cursor in the same position
		// TODO: fill the new row with BCE color?
		fb.scrollRows(1)
	} else {
		t.cursorMoveAbs(t.cur.row+1, t.cur.col)
	}
}

func (t *Terminal) scrollRegion(n int) {
	fb, err := t.getScrollingRegion()
	if err != nil {
		slog.Debug("couldn't get scrolling region", "err", err)
		return
	}
	fb.scrollRows(n)
}

func (t *Terminal) scrollAll(n int) {
	t.fb.scrollRows(n)
}

func (t *Terminal) xtwinops(params *parameters) {
	slog.Debug("handling xtwinops", "params", params)
	switch params.getItem(0, 0) {
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
		if params.getItem(0, 0) != 0 {
			slog.Debug("invalid xterm_version query", "params", params, "data", string(data))
			return
		}
		r := fmt.Sprintf("%c%c>|gosh(%s)%c%c", ESC, DCS, GOSH_VT_VER, ESC, ST)
		t.Write([]byte(r))
		slog.Debug("identifying as gosh version", "ver", GOSH_VT_VER)
	default:
		slog.Debug("unhandled CSI q", "params", params, "data", string(data))
	}
}

func (t *Terminal) handleDSR(params *parameters, data []rune) {
	switch string(data) {
	case "": // General device status report
		switch params.getItem(0, 0) {
		case 5: // We always report OK (CSI 0 n)
			t.Write([]byte(fmt.Sprintf("%c%c0%c", ESC, CSI, CSI_DSR)))
		case 6: // Provide cursor location (CSI r ; c R)
			t.Write([]byte(fmt.Sprintf("%c%c%d;%dR", ESC, CSI, t.cur.row+1, t.cur.col+1)))
		}
	case "?": // DEC specific device status report
		switch params.getItem(0, 0) {
		case 6: // Provide cursor location (CSI ? r ; c R)

			t.Write([]byte(fmt.Sprintf("%c%c?%d;%dR", ESC, CSI, t.cur.row+1, t.cur.col+1)))
		case 15: // report printer status; always "not ready" (CSI ? 1 1 n)
			t.Write([]byte(fmt.Sprintf("%c%c?11%c", ESC, CSI, CSI_DSR)))
		default:
			slog.Debug("swallowing CSI ? DSR code", "params", params, "data", string(data))
		}
	case ">":
		slog.Debug("swallowing xterm disable key modifiers", "params", params, "data", string(data))
	default:
		slog.Debug("unknown CSI DSR modifier string", "params", params, "data", data)
	}
}

func (t *Terminal) doDECALN() {
	slog.Debug("DECALN screen test triggered")
	t.curF = defFmt
	t.horizMargin = margin{}
	t.vertMargin = margin{}
	t.cursorMoveAbs(0, 0)
	t.fb.fill(newCell('E', t.curF))
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

func (t *Terminal) setMode(mode int, data string, state rune) {
	switch data {
	case "?":
		name := privIDToName[mode]
		m, ok := t.modes[name]
		if !ok {
			slog.Debug("unknown CSI private mode toggled; ignoring", "mode", name, "data", data, "state", string(state))
			return
		}
		m.setState(state)
		slog.Debug("setting private mode", "mode", name, "state", string(state))
		switch mode {
		case DECCOLM:
			t.fb.fill(newCell(' ', t.curF))
			t.homeCursor()
		case DECOM:
			t.homeCursor()
		}
	case "":
		name := pubIDToName[mode]
		m, ok := t.modes[name]
		if !ok {
			slog.Debug("unknown CSI public mode toggled; ignoring", "mode", name, "data", data, "state", string(state))
			return
		}
		m.setState(state)
		slog.Debug("setting public mode", "mode", name, "state", string(state))
	default:
		slog.Debug("unexpected CSI set/reset data", "mode", mode, "data", data, "state", string(state))
	}
}

func (t *Terminal) setTopBottom(params *parameters) {
	nr := t.rows()
	top := params.getItem(0, 1)
	bottom := params.getItem(1, nr)
	if bottom <= top || top > nr || (top == 0 && bottom == 1) {
		return // matches xterm
	}

	// https://vt100.net/docs/vt510-rm/DECSTBM.html
	// STBM sets the cursor to 1,1 (0,0)
	t.vertMargin = newMargin(top-1, bottom-1)
	slog.Debug("set top/bottom margin", "margin", t.vertMargin)
	t.homeCursor()
}

func (t *Terminal) setLeftRight(params *parameters) {
	nc := t.cols()
	left := params.getItem(0, 1)
	right := params.getItem(1, nc)
	if right <= left || left >= nc || (left == 0 && right == 1) {
		return // matches xterm
	}

	// https://vt100.net/docs/vt510-rm/DECSLRM.html
	// STBM sets the cursor to 1,1 (0,0)
	t.horizMargin = newMargin(left-1, right-1)
	slog.Debug("set left/right margin", "margin", t.horizMargin)
	t.homeCursor()
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

	max := t.cols() - 1
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
	m := params.getItem(0, 1)
	cols := t.cols()

	for i := t.cur.row; i < t.cur.row+m && t.vertMargin.contains(i); i++ {
		t.fb.data[i] = newRow(cols)
	}
}

func (t *Terminal) deleteChars(params *parameters, data []rune) {
	if len(data) != 0 {
		slog.Debug("skipping CSI DCH with unexpected data", "params", params, "data", string(data))
		return
	}

	n := params.getItem(0, 1)
	if n == 0 {
		n = 1
	}

	row, col := t.row(), t.col()
	right := t.cols()
	if t.inScrollingRegion() {
		right = t.getRightMargin()
	}

	// If the cursor happens to be parked on a fragment cell, we
	// need to adjust for how we do our deletion.
	c, _ := t.fb.getCell(row, col)
	if c.isFragment() {
		// we need to ensure we overwrite this character and
		// the secondary fragment, so pull everything back by
		// an additional 1 column.
		n += 1
	}
	if c.isSecondaryFrag() {
		// We need to account for the the primary fragment in
		// how we handle this, so in addition to pulling all
		// chars back an extra column, we also start our range
		// on the primary
		col -= 1
	}

	reg, err := t.fb.getRegion(row, row, col, right)
	if err != nil {
		slog.Debug("invalid framebuffer region", "r", row, "c", col, "right", right, "err", err)
	}

	nc := reg.getNumCols()
	offset := n
	for i := n; i < nc; i++ {
		c, err := reg.getCell(0, i)
		// If we start on a secondary fragment, we need to
		// skip it and then increase the offset at which we
		// shift cells back.
		if i == n && c.isSecondaryFrag() {
			offset += 1
			continue
		}
		if err != nil {
			slog.Error("invalid cell request during deleteChars", "col", i, "cur", t.cur, "err", err)
		}

		reg.setCell(0, i-offset, c)
	}
	// TODO: Handle format more appropriately here.  Leave
	// it intact? Default as we do now? Does BCE come into
	// play?
	for i := nc - 1; i > nc-1-n; i-- {
		reg.setCell(0, i, defaultCell())
	}
}

func (t *Terminal) eraseLine(params *parameters) {
	// TODO: Handle BCE properly
	dc := defaultCell()
	dc.f = t.curF

	nc := t.cols() - 1
	switch params.getItem(0, 0) {
	case 0: // to end of line
		t.fb.setCells(t.cur.row, t.cur.row, t.cur.col, nc, dc)
		slog.Debug("erase in line, pos to end", "row", t.cur.row, "col", t.cur.col)
	case 1: // to start of line
		t.fb.setCells(t.cur.row, t.cur.row, 0, t.cur.col, dc)
		slog.Debug("erase in line, start of line to pos", "row", t.cur.row, "col", t.cur.col)
	case 2: // entire line
		t.fb.setCells(t.cur.row, t.cur.row, 0, nc, dc)
		slog.Debug("erase in line, entire line", "row", t.cur.row, "col", t.cur.col)
	}
}

func (t *Terminal) eraseInDisplay(params *parameters) {
	// TODO: Handle BCE properly
	nr := t.rows()
	switch params.getItem(0, 0) {
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
