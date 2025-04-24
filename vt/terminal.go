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
	for id, md := range modeDefaults {
		modes[id] = md.copy()
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

	// note that we don't copy margins and we also don't compare
	// them for diffs. this is only important for the server side
	// rendering so we can ignore it as long as we handle it
	// appropriately and ship the visual diff to the client.
	return &Terminal{
		fb:      t.fb.copy(),
		title:   t.title,
		icon:    t.icon,
		cur:     t.cur,
		curF:    t.curF,
		modes:   modes,
		lastChg: t.lastChg,
	}
}

// Diff will generate a sequence of bytes that, when applied, would
// move src to dest. This is at a visual level only as the server will
// ship these diffs to the client which is stateless and only used to
// display output. Because of that, we can ignore some properties like
// margins which are only important if you're applying all of the
// display updating based on the output from the pty directly.
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

	for _, name := range transportModes {
		id, ok := modeNameToID[name]
		if !ok {
			slog.Debug("invalid modeNameToID", "name", name)
			continue
		}
		if !src.modes[id].equal(dest.modes[id]) {
			sb.WriteString(dest.modes[id].ansiString())
		}
	}

	// we always generate diffs as from previous to current
	fbd := src.fb.diff(dest.fb)
	if len(fbd) > 0 {
		sb.WriteString(FMT_RESET)
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
		sb.WriteString(dest.cur.ansiString())
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
				t.handleCSI(a.params, string(a.data), a.r)
			case VTPARSE_ACTION_OSC_START, VTPARSE_ACTION_OSC_PUT, VTPARSE_ACTION_OSC_END:
				t.handleOSC(a.act, a.r)
			case VTPARSE_ACTION_PRINT:
				t.print(a.r)
			case VTPARSE_ACTION_ESC_DISPATCH:
				t.handleESC(a.params, string(a.data), a.r)
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

func (t *Terminal) boundedMarginLeft() int {
	if t.horizMargin.isSet() && t.horizMargin.min() <= t.col() {
		return t.horizMargin.min()
	}
	return 0
}

func (t *Terminal) boundedMarginRight() int {
	if t.horizMargin.isSet() && t.horizMargin.max() >= t.col() {
		return t.horizMargin.max()
	}
	return t.cols() - 1
}

func (t *Terminal) boundedMarginTop() int {
	if t.vertMargin.isSet() && t.vertMargin.min() <= t.row() {
		return t.vertMargin.min()
	}
	return 0
}

func (t *Terminal) boundedMarginBottom() int {
	if t.vertMargin.isSet() && t.vertMargin.max() >= t.row() {
		return t.vertMargin.max()
	}
	return t.rows() - 1
}

func (t *Terminal) leftMargin() int {
	if t.horizMargin.isSet() {
		return t.horizMargin.min()
	}
	return 0
}

func (t *Terminal) rightMargin() int {
	if t.horizMargin.isSet() {
		return t.horizMargin.max()
	}
	return t.cols() - 1
}

func (t *Terminal) topMargin() int {
	if t.vertMargin.isSet() {
		return t.vertMargin.min()
	}
	return 0
}

func (t *Terminal) bottomMargin() int {
	if t.vertMargin.isSet() {
		return t.vertMargin.max()
	}
	return t.rows() - 1
}

// Move to first position on next line. If we're at the bottom margin,
// scroll.
func (t *Terminal) ctrlNEL() {
	t.lineFeed()
	t.carriageReturn()
}

func (t *Terminal) handleESC(params *parameters, data string, r rune) {
	switch r {
	case 'A', 'B', 'C', 'K', 'Q', 'R', 'Y', 'Z', '2', '4', '6', '>', '=', '`':
		slog.Debug("swallowing ESC character set command", "params", params, "data", data, "cmd", string(r))
	case NEL:
		t.ctrlNEL()
	case 'F':
		t.cursorMoveAbs(t.boundedMarginBottom(), t.boundedMarginLeft())
	case HTS: // set tab stop. note that in some vt dialects this
		// would actually be part of character set handling
		// (swedish on vt220).
		t.tabs[t.col()] = true
	case IND: // move cursor one line down, scrolling if needed
		if row, max := t.row(), t.bottomMargin(); row == max {
			t.scrollRegion(1)
		} else {
			t.cursorMoveAbs(row+1, t.col())
		}
	case RI: // move cursor one line up, scrolling if needed
		if row, min := t.row(), t.topMargin(); row == min {
			t.scrollRegion(-1)
		} else {
			t.cursorMoveAbs(row-1, t.col())
		}
	case DECSC: // save cursor
		t.savedCur = t.cur.Copy()
		t.savedF = t.curF
		slog.Debug("saved cursor and format", "save", t.savedCur)
	case DECRC: // restore cursor or decaln screen test
		switch data {
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
		slog.Debug("ignoring ESC", "r", string(r), "params", params, "data", data)
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
			data := string(t.oscTemp)
			slog.Debug("Handling OSC data", "data", data)
			parts := strings.SplitN(data, ";", 2)
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
				slog.Error("Unknown OSC entity", "data", data)
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
	if gc, err := t.fb.cell(row, col); err == nil {
		switch gc.frag {
		case FRAG_PRIMARY: // primary cell
			t.fb.setCell(row, col+1, defaultCell())
		case FRAG_SECONDARY: // secondary/empty cell
			t.fb.setCell(row, col-1, defaultCell())
		}
	}
}

func (t *Terminal) reset() {
	cols := t.cols()
	t.fb = newFramebuffer(t.rows(), cols)
	t.title = ""
	t.icon = ""
	modes := make(map[string]*mode)
	for id, md := range modeDefaults {
		modes[id] = md.copy()
	}
	t.modes = modes
	t.vertMargin = margin{}
	t.horizMargin = margin{}
	t.homeCursor()
	t.savedCur = cursor{0, 0}
	t.tabs = makeTabs(cols)
}

func (t *Terminal) isModeSet(name string) bool {
	m, ok := t.modes[modeNameToID[name]]
	if !ok {
		slog.Debug("asked if unknown mode was set", "mode", name)
		return false
	}
	return m.enabled()
}

func (t *Terminal) print(r rune) {
	row, col := t.row(), t.col()
	rw := runewidth.StringWidth(string(r))

	switch rw {
	case 0: // combining
		combR, combC := row, col
		if t.isModeSet("IRM") {
			combC = col
		} else {
			if col != 0 {
				combC = col - 1
			} else {
				if t.isModeSet("DECAWM") {
					if row != t.boundedMarginTop() {
						combR = row - 1
						combC = t.boundedMarginRight()
					} else {
						slog.Debug("can't find row/col to combine char with", "row", row, "col", col)
						return
					}
				}
			}
		}

		c, err := t.fb.cell(combR, combC)
		if err != nil {
			slog.Debug("couldn't retrieve cell to combine character with", "row", combR, "col", combC, "err", err)
			return
		}

		n := norm.NFC.String(string(c.r) + string(r))
		c.r = []rune(n)[0]
		t.fb.setCell(combR, combC, c)
	default: // default (1 column), wide (2 columns)
		if col > t.cols()-rw { // rune will not fit on row
			if t.isModeSet("DECAWM") { // autowrap is on
				// if we're at bottom of region,
				// scroll it by one row to free new
				// room in all cases, col becomes the
				// left most column - either 0 or left
				// margin
				if row == t.bottomMargin() {
					t.scrollRegion(1)
				} else {
					row += 1
				}
				col = t.boundedMarginLeft()
			} else {
				col = t.cols() - rw // overwrite end of row
			}
		}

		if t.isModeSet("IRM") {
			right := t.boundedMarginRight()

			cells, err := t.fb.subRegion(row, row, col, right)
			if err != nil {
				slog.Debug("invalid framebuffer region", "row", row, "col", col, "right", right, "err", err)
				return
			}

			// Shuffle everything forward by rw columns
			for i := cells.cols() - 1; i >= rw; i-- {
				c, err := cells.cell(0, i-rw)
				if err != nil {
					slog.Debug("invalid cell in region", "row", 0, "col", i-1, "err", err)
					return
				}
				cells.setCell(0, i, c)
			}
		}

		t.clearFrags(row, col)
		nc := newCell(r, t.curF)
		if rw > 1 {
			// Clear adjacent cells and note fragments
			t.fb.setCell(row, col+1, fragCell(0, t.curF, FRAG_SECONDARY))
			nc.frag = FRAG_PRIMARY
		}

		t.fb.setCell(row, col, nc)

		if !t.isModeSet("IRM") {
			col += rw
		}

		t.setRow(row)
		t.setCol(col)
	}
}

func (t *Terminal) handleExecute(last rune) {
	switch last {
	case BEL:
		// just swallow this for now
	case BS:
		t.cursorBack(1)
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

func (t *Terminal) handleCSI(params *parameters, data string, last rune) {
	switch last {
	case CSI_DSR:
		t.handleDSR(params, data)
	case CSI_DA:
		t.replyDeviceAttributes(data)
	case CSI_Q_MULTI:
		t.csiQ(params, data)
	case CSI_XTWINOPS:
		t.xtwinops(params.item(0, 0))
	case CSI_DCH:
		if data != "" {
			slog.Debug("skipping CSI DCH with unexpected data", "params", params, "data", data)
			return
		}
		t.deleteChars(params.itemDefaultOneIfZero(0, 1))
	case CSI_ICH:
		t.insertChars(' ', params.itemDefaultOneIfZero(0, 1))
	case CSI_ECH:
		// Insert n blank characters where n is the provided parameter
		last := t.cur.col + params.item(0, 1)
		if lastCol := t.cols() - 1; last > lastCol {
			last = lastCol
		}
		row := t.row()
		t.fb.setCells(row, row, t.col(), last, newCell(' ', t.curF))
	case CSI_MODE_SET, CSI_MODE_RESET:
		t.setMode(params.item(0, 0), data, last)
	case CSI_DECSTBM:
		t.setTopBottom(params)
	case CSI_DECSLRM:
		t.setLeftRight(params)
	case CSI_DL:
		t.deleteLines(params)
	case CSI_EL:
		t.eraseLine(params.item(0, 0))
	case CSI_SU:
		t.scrollRegion(-params.item(0, 1))
	case CSI_SD:
		t.scrollRegion(params.item(0, 1))
	case CSI_ED:
		t.eraseInDisplay(params.item(0, 0))
	case CSI_VPA, CSI_VPR, CSI_HPA, CSI_HPR, CSI_CUP, CSI_CUU, CSI_CUD, CSI_CUB, CSI_CUF, CSI_CNL, CSI_CPL, CSI_CHA, CSI_HVP:
		t.cursorMove(params, last)
	case CSI_SGR:
		if data != "" {
			slog.Debug("swallowing xterm specific key modifier set/reset or query", "params", params, "data", data)
		} else {
			t.curF = formatFromParams(t.curF, params)
		}
	case CSI_DECST8C:
		t.resetTabs(params, data)
	case CSI_CHT:
		t.stepTabs(params.item(0, 1))
	case CSI_CBT:
		t.stepTabs(-params.item(0, 1))
	case CSI_TBC:
		t.clearTabs(params)
	default:
		slog.Debug("unimplemented CSI code", "last", string(last), "params", params, "data", data)
	}
}

func (t *Terminal) resetTabs(params *parameters, data string) {
	if data != "?" || params.item(0, 0) != 5 {
		slog.Debug("resetTabs called without ? 5 as data and parameter", "data", data, "params", params)
	}
	cols := t.cols()
	tabs := make([]bool, cols, cols)
	for i := 0; i < cols; i += 8 {
		tabs[i] = true
	}
	t.tabs = tabs
}

func (t *Terminal) clearTabs(params *parameters) {
	switch params.item(0, 0) {
	case TBC_CUR:
		t.tabs[t.cur.col] = false
	case TBC_ALL:
		for i := range t.tabs {
			t.tabs[i] = false
		}
	}
}

func (t *Terminal) carriageReturn() {
	t.cursorMoveAbs(t.row(), t.boundedMarginLeft())
}

func (t *Terminal) inScrollingRegion() bool {
	return t.vertMargin.contains(t.row()) && t.horizMargin.contains(t.col())
}

func (t *Terminal) scrollingRegion() (*framebuffer, error) {
	return t.fb.subRegion(t.topMargin(), t.bottomMargin(), t.leftMargin(), t.rightMargin())
}

func (t *Terminal) lineFeed() {
	row := t.row()
	if bottom := t.boundedMarginBottom(); row == bottom {
		// else nothing because we're at the vert margin, but
		// outside the horiz margin and we stop.
		if t.horizMargin.contains(t.col()) {
			t.scrollRegion(1)
		}
	} else {
		t.cursorMoveAbs(row+1, t.col())
	}
}

func (t *Terminal) scrollRegion(n int) {
	fb, err := t.scrollingRegion()
	if err != nil {
		slog.Debug("couldn't get scrolling region", "err", err)
		return
	}
	fb.scrollRows(n)
}

func (t *Terminal) xtwinops(n int) {
	switch n {
	case XTWINOPS_SAVE: // save title and icon
		t.savedTitle = t.title
		t.savedIcon = t.icon
	case XTWINOPS_RESTORE: // restore title and icon
		t.title = t.savedTitle
		t.icon = t.savedIcon
	default:
		slog.Debug("invalid xtwinops command", "n", n)
	}
}

func (t *Terminal) csiQ(params *parameters, data string) {
	switch data {
	case ">":
		if params.item(0, 0) != 0 {
			slog.Debug("invalid xterm_version query", "params", params, "data", data)
			return
		}
		r := fmt.Sprintf("%c%c>|gosh(%s)%c%c", ESC, DCS, GOSH_VT_VER, ESC, ST)
		t.Write([]byte(r))
		slog.Debug("identifying as gosh version", "ver", GOSH_VT_VER)
	default:
		slog.Debug("unhandled CSI q", "params", params, "data", data)
	}
}

func (t *Terminal) handleDSR(params *parameters, data string) {
	slog.Debug("handling DSR request", "params", params, "data", data)
	switch data {
	case "": // General device status report
		switch params.item(0, 0) {
		case 5: // We always report OK (CSI 0 n)
			t.Write([]byte(fmt.Sprintf("%c%c%d%c", ESC, CSI, 0, CSI_DSR)))
		case 6: // Provide cursor location (CSI r ; c R)
			row, col := t.row(), t.col()
			if t.isModeSet("DECOM") {
				row -= t.topMargin()
				col -= t.leftMargin()
			}
			slog.Debug("reporting cursor position", "row", row, "col", col)
			t.Write([]byte(fmt.Sprintf("%c%c%d;%d%c", ESC, CSI, row+1, col+1, CSI_POS)))
		default:
			slog.Debug("unhandled CSI DSR request", "params", params, "data", data)
		}
	case "?": // DEC specific device status report
		switch params.item(0, 0) {
		case 6: // Provide cursor location (CSI ? r ; c R)
			t.Write([]byte(fmt.Sprintf("%c%c?%d;%d%c", ESC, CSI, t.row()+1, t.col()+1, CSI_POS)))
		case 15: // report printer status; always "not ready" (CSI ? 1 1 n)
			t.Write([]byte(fmt.Sprintf("%c%c?11%c", ESC, CSI, CSI_DSR)))
		case 25: // UDK (universal disk kit); always "unlocked" (CSI ? 2 0 n)
			t.Write([]byte(fmt.Sprintf("%c%c?%d%c", ESC, CSI, 20, CSI_DSR)))
		default:
			slog.Debug("unhandled CSI ? DSR request", "params", params, "data", data)
		}
	case ">":
		slog.Debug("swallowing xterm disable key modifiers", "params", params, "data", data)
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

func (t *Terminal) replyDeviceAttributes(data string) {
	switch data {
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
	mid := fmt.Sprintf("%s%d", data, mode)
	m, ok := t.modes[mid]
	if !ok {
		slog.Debug("unknown CSI mode", "id", mid)
		return
	}

	m.setState(state)
	slog.Debug("set CSI mode", "id", mid, "name", m.name, "state", string(state))

	// Don't use code here because codes are duplicated between
	// ansi and DEC private modes.
	switch m.name {
	case "DECCOLM":
		// this mode is mostly ignored, but it does trigger a
		// screen clear and homing of the cursor. we don't do
		// anything with available columsn per row though.
		t.eraseInDisplay(ERASE_ALL)
		t.homeCursor()
	case "DECOM":
		t.homeCursor()
	}
}

func (t *Terminal) setTopBottom(params *parameters) {
	nr := t.rows()
	top := params.item(0, 1)
	bottom := params.item(1, nr)
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
	left := params.item(0, 1)
	right := params.item(1, nc)
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
		t.horizMargin.contains(t.row()) &&
		t.vertMargin.contains(t.col())
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
	col, step, inc := t.col()+1, 1, -1

	switch {
	case steps == 0:
		// shouldn't happen, but don't adjust cursor if it
		// does
		return
	case steps < 0:
		// we're moving backward through the line, not forward
		col = t.col() - 1
		step = -1
		inc = 1
	}

	max := t.cols() - 1
	for {
		switch {
		case col <= 0:
			// TODO: Make this region aware?
			t.cursorMoveAbs(t.row(), 0)
			return
		case col >= max:
			t.cursorMoveAbs(t.row(), max)
			return
		default:
			if t.tabs[col] {
				steps += inc
				if steps == 0 {
					t.cursorMoveAbs(t.row(), col)
					return
				}
			}
			col += step
		}
	}
}

func (t *Terminal) deleteLines(params *parameters) {
	m := params.item(0, 1)
	cols := t.cols()
	row := t.row()

	for i := row; i < row+m && t.vertMargin.contains(i); i++ {
		t.fb.data[i] = newRow(cols)
	}
}

func (t *Terminal) deleteChars(n int) {

	row, col := t.row(), t.col()
	right := t.cols()
	if t.inScrollingRegion() {
		right = t.rightMargin()
	}

	// If the cursor happens to be parked on a fragment cell, we
	// need to adjust for how we do our deletion.
	c, _ := t.fb.cell(row, col)
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

	reg, err := t.fb.subRegion(row, row, col, right)
	if err != nil {
		slog.Debug("invalid framebuffer region", "r", row, "c", col, "right", right, "err", err)
	}

	nc := reg.cols()
	offset := n
	for i := n; i < nc; i++ {
		c, err := reg.cell(0, i)
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

// insertChars  will insert r, n times
func (t *Terminal) insertChars(r rune, n int) {
	slog.Debug("insertChars", "r", string(r), "n", n)
	om := t.isModeSet("IRM")
	t.setMode(IRM, "", CSI_MODE_SET)
	for i := 0; i < n; i++ {
		t.print(r)
	}
	if !om {
		t.setMode(IRM, "", CSI_MODE_RESET)
	}
}

func (t *Terminal) eraseLine(n int) {
	// TODO: Handle BCE properly
	dc := defaultCell()
	dc.f = t.curF

	row, col := t.row(), t.col()
	nc := t.cols() - 1
	switch n {
	case ERASE_FROM_CUR: // to end of line
		t.fb.setCells(row, row, col, nc, dc)
		slog.Debug("erase in line, pos to end", "row", row, "col", col)
	case ERASE_TO_CUR: // to start of line
		t.fb.setCells(row, row, 0, col, dc)
		slog.Debug("erase in line, start of line to pos", "row", row, "col", col)
	case ERASE_ALL: // entire line
		t.fb.setCells(row, row, 0, nc, dc)
		slog.Debug("erase in line, entire line", "row", row, "col", col)
	}
}

func (t *Terminal) eraseInDisplay(n int) {
	// TODO: Handle BCE properly
	switch n {
	case ERASE_FROM_CUR: // active position to end of screen, inclusive
		t.fb.resetRows(t.row()+1, t.rows()-1)
		t.eraseLine(n)
		slog.Debug("CSI erase in display, pos to end of screen")
	case ERASE_TO_CUR: // start of screen to active position, inclusive
		t.fb.resetRows(0, t.row()-1)
		t.eraseLine(n)
		slog.Debug("CSI erase in display, beginning of screen to pos")
	case ERASE_ALL: // entire screen
		t.fb.resetRows(0, t.rows()-1)
		slog.Debug("CSI erase in display, entire screen")
	}
}
