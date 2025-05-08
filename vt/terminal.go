package vt

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
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
	p         *parser
	fb, altFb *framebuffer

	ptyF *os.File

	wait, stop manageFunc

	// keypad mode to ship to the client
	keypad rune // should be = (application) or > (normal)

	// State
	lastChg               time.Time
	title, icon           string
	titlePfx              string
	savedTitle, savedIcon string
	cur, savedCur         cursor
	curF, savedF          format
	tabs                  []bool

	cs, savedCS *charset

	// Temp
	oscTemp []rune

	// scroll margin/region parameters
	vertMargin, horizMargin margin

	// Modes that have been set or reset
	modes map[string]*mode

	// Internal
	mux sync.Mutex
}

func newBasicTerminal(f *os.File) *Terminal {
	modes := make(map[string]*mode)
	for id, md := range modeDefaults {
		modes[id] = md.copy()
	}

	return &Terminal{
		fb:      newFramebuffer(DEF_ROWS, DEF_COLS),
		oscTemp: make([]rune, 0),
		tabs:    makeTabs(DEF_COLS),
		modes:   modes,
		keypad:  PNM, // normal
		p:       newParser(),
		ptyF:    f,
		wait:    func() {},
		stop:    func() {},
		cs:      &charset{},
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

	t := newBasicTerminal(ptmx)
	t.wait = func() { cmd.Wait() }
	t.stop = func() { cancel() }
	t.titlePfx = "[gosh] "

	return t, nil
}

func NewTerminal() (*Terminal, error) {
	// On the client end, we don't need a file as we back Write()
	// with direct parser input instead of routing it via the pty
	// to the program we're running.
	return newBasicTerminal(nil), nil
}

func (t *Terminal) Wait() {
	t.wait()
}

func (t *Terminal) Stop() {
	t.stop()
	t.ptyF.Close() // ensure Run() stops
}

func (t *Terminal) MakeOverlay(text string) []byte {
	var sb strings.Builder

	// save cursor, format
	sb.WriteString("\x1b7")
	// home cursor
	sb.WriteString("\x1b[H")
	// set pen
	sb.WriteString(fmt.Sprintf("%c%c%d;%d;%d%c", ESC, CSI, FG_BRIGHT_YELLOW, BG_RED, BOLD, CSI_SGR))
	// clear line
	sb.WriteString("\x1b[2K")
	// move pen for centered message
	slog.Debug("overlay cols", "cols", t.Cols())
	sb.WriteString(cursor{0, (t.Cols() - len(text) - 1) / 2}.ansiString())
	sb.WriteString(text)
	// restore cursor, format
	sb.WriteString("\x1b8")

	return []byte(sb.String())
}

// FirstRow will return the byte sequence to generate the first row of the vt
func (t *Terminal) FirstRow() ([]byte, error) {
	fb, err := t.fb.subRegion(0, 0, 0, t.Cols()-1)
	if err != nil {
		return nil, fmt.Errorf("couldn't retrieve first row subregion: %v", err)
	}
	fbe := fb.copy()
	fbe.fill(newCell(' ', defFmt))
	return fbe.diff(fb), nil
}

func (t *Terminal) Write(p []byte) (int, error) {
	// The client doesn't have an actual file, so we can
	// differentiate that way. For the client, we just feed the
	// parser directly. We also don't need to handle LNM as we do
	// below for the server because that doesn't affect what we
	// need to do on the client as it's already represented in the
	// bytes shipped from the server.
	if t.ptyF == nil {
		if err := t.doParse(bufio.NewReader(bytes.NewReader(p))); err != nil {
			return 0, err
		}
		return len(p), nil
	}

	inp := p
	if t.isModeSet("LNM") {
		inp = bytes.ReplaceAll(p, []byte("\r"), []byte("\r\n"))
	}

	return t.ptyF.Write(inp)
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
		keypad:  t.keypad,
		modes:   modes,
		lastChg: t.lastChg,
		p:       t.p.copy(),
		ptyF:    t.ptyF,
		cs:      t.cs.copy(),
	}
}

func (t *Terminal) Replace(other *Terminal) {
	t.mux.Lock()
	defer t.mux.Unlock()

	t.fb = other.fb.copy()
	t.title = other.title
	t.icon = other.icon
	t.curF = other.curF
	t.savedF = other.savedF
	t.cur = other.cur
	t.savedCur = other.savedCur
	t.p = other.p
	t.modes = other.modes
}

func (t *Terminal) LastChange() time.Time {
	return t.lastChg
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

	if src.keypad != dest.keypad {
		sb.WriteString(fmt.Sprintf("%c%c", ESC, dest.keypad))
	}

	for _, name := range transportModes {
		id, ok := modeNameToID[name]
		if !ok {
			// should never happen, so we'll elevate to an
			// error as it won't pollute logs and should
			// be made easily visible if something goes
			// wrong.
			slog.Error("invalid modeNameToID when writing modes", "name", name)
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
	rr := bufio.NewReader(t.ptyF)

	if err := t.doParse(rr); err != nil {
		slog.Error("doParse error", "err", err)
	}
}

func (t *Terminal) doParse(rr *bufio.Reader) error {
	for {
		r, sz, err := rr.ReadRune()
		if err != nil {
			if !errors.Is(err, os.ErrDeadlineExceeded) {
				slog.Error("pty ReadRune", "r", r, "sz", sz, "err", err)
				return nil
			}
			return err
		}

		if r == utf8.RuneError && sz == 1 {
			rr.UnreadRune()
			b, err := rr.ReadByte()
			if err != nil {
				return err
			}

			// Note that this is really gross and we
			// should handle 8bit inputs more cleanly. It
			// works because we've coerced all of the
			// 8-bit bytes into runes in the lookup
			// table. This means that there exists a small
			// subset of valid 2-byte runes that we won't
			// see as printable characters.
			//
			// TODO: Fix parsing to cleanly handle 8-bit
			// input.
			r = rune(b)
		}

		for _, a := range t.p.parse(r) {
			t.mux.Lock()
			t.lastChg = time.Now().UTC()
			switch a.act {
			case VTPARSE_ACTION_EXECUTE:
				t.handleExecute(a.cmd)
			case VTPARSE_ACTION_CSI_DISPATCH:
				t.handleCSI(a.params, string(a.data), a.cmd)
			case VTPARSE_ACTION_OSC_START, VTPARSE_ACTION_OSC_PUT, VTPARSE_ACTION_OSC_END:
				t.handleOSC(a.act, a.cmd)
			case VTPARSE_ACTION_PRINT:
				t.print(a.cmd)
			case VTPARSE_ACTION_ESC_DISPATCH:
				t.handleESC(a.params, string(a.data), a.cmd)
			default:
				slog.Debug("unhandled parser action", "action", ACTION_NAMES[a.act], "params", a.params, "data", a.data, "rune", a.cmd)
			}
			t.mux.Unlock()
		}
	}

	return nil
}

func (t *Terminal) Resize(rows, cols int) {
	pts := &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	}

	if term.IsTerminal(int(t.ptyF.Fd())) {
		if err := pty.Setsize(t.ptyF, pts); err != nil {
			slog.Error("couldn't set size on pty", "err", err)
		}
		// Any use of Fd(), including in the InheritSize call above,
		// will set the descriptor non-blocking, so we need to change
		// that here.
		pfd := int(t.ptyF.Fd())
		if err := syscall.SetNonblock(pfd, true); err != nil {
			slog.Error("couldn't set pty to nonblocking", "err", err)
			return
		}
	}

	t.mux.Lock()
	t.fb.resize(rows, cols)
	t.resizeTabs(cols)
	t.lastChg = time.Now().UTC()
	t.mux.Unlock()
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
	return t.Cols() - 1
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
	return t.Rows() - 1
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
	return t.Cols() - 1
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
	return t.Rows() - 1
}

// Move to first position on next line. If we're at the bottom margin,
// scroll.
func (t *Terminal) ctrlNEL() {
	t.lineFeed()
	t.carriageReturn()
}

func (t *Terminal) handleESC(params *parameters, data string, cmd rune) {
	switch data {
	case "(", ")": // desginate g0 or g1 charset
		switch cmd {
		case '0', 'B':
			t.cs.setCS(data, cmd)
		default:
			slog.Debug("swallowing ESC character set command", "params", params, "data", data, "cmd", string(cmd))
		}
	case "":
		switch cmd {
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
			t.cursorSave()
		case DECRC: // restore cursor or decaln screen test
			switch data {
			case "":
				t.cursorRestore()
			case "#": // DECALN vt100 screen test
				t.doDECALN()
			}
		case PAM, PNM:
			t.keypad = cmd
		case RIS:
			t.reset()
		default:
			slog.Debug("unhandled ESC command", "cmd", string(cmd), "params", params, "data", data)
		}
	default:
		slog.Debug("unimplemented ESC command selector", "cmd", string(cmd), "params", params, "data", data)
	}
}

// cursorSave does what it says on the tin, but the tin is
// misleading. The cursor position, current pen format and character
// set state are all snapshotted.
func (t *Terminal) cursorSave() {
	t.savedCur = t.cur.Copy()
	t.savedF = t.curF
	t.savedCS = t.cs.copy()
}

// Restore the cursor position, pen format and charset state.
func (t *Terminal) cursorRestore() {
	t.cur = t.savedCur.Copy()
	t.curF = t.savedF
	t.cs = t.savedCS.copy()
}

func (t *Terminal) handleOSC(act pAction, cmd rune) {
	switch act {
	case VTPARSE_ACTION_OSC_START:
		t.oscTemp = make([]rune, 0)
	case VTPARSE_ACTION_OSC_PUT:
		t.oscTemp = append(t.oscTemp, cmd)
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
			parts := strings.SplitN(data, ";", 3)
			switch parts[0] {
			case OSC_ICON_TITLE:
				t.title = t.titlePfx + parts[1]
				t.icon = parts[1]
			case OSC_ICON:
				t.icon = parts[1]
			case OSC_TITLE:
				t.title = t.titlePfx + parts[1]
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

						// Can't use t.Resize
						// here as it takes
						// the same lock.
						t.fb.resize(rows, cols)

						// we only use the for loop so
						// that we can break and clean
						// up osctemp no matter what.
						break
					}

				} else {
					slog.Debug("expected 2 inputs to X;rows;cols osc setsize", "len", len(parts), "osctemp", t.oscTemp)
				}
			default:
				slog.Debug("unknown OSC command", "data", data)
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
	cols := t.Cols()
	t.fb = newFramebuffer(t.Rows(), cols)
	t.title = ""
	t.icon = ""
	modes := make(map[string]*mode)
	for id, md := range modeDefaults {
		modes[id] = md.copy()
	}
	t.modes = modes
	t.keypad = PNM
	t.vertMargin = margin{}
	t.horizMargin = margin{}
	t.homeCursor()
	t.savedCur = cursor{0, 0}
	t.tabs = makeTabs(cols)
	t.cs = &charset{}
}

func (t *Terminal) isModeSet(name string) bool {
	m, ok := t.modes[modeNameToID[name]]
	if !ok {
		slog.Debug("unknown mode queried", "mode", name)
		return false
	}
	return m.enabled()
}

func (t *Terminal) print(r rune) {
	row, col := t.row(), t.col()

	ar := t.cs.runeFor(r)
	rw := runewidth.StringWidth(string(ar))

	switch rw {
	case 0: // combining
		// if we're in insert mode, we should always use the
		// same cell, so we don't adjust that below.
		combR, combC := row, col
		if !t.isModeSet("IRM") {
			if col != 0 {
				combC = col - 1
			} else {
				if t.isModeSet("DECAWM") {
					if row != t.boundedMarginTop() {
						combR = row - 1
						for i := t.boundedMarginRight(); i >= 0; i-- {
							if c, err := t.fb.cell(combR, i); err == nil {
								if c.set {
									combC = i
									break
								}
							}
						}
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

		c.r = []rune(norm.NFC.String(string(c.r) + string(ar)))[0]
		t.fb.setCell(combR, combC, c)
	default: // default (1 column), wide (2 columns)
		if col > t.Cols()-rw { // rune will not fit on row
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
				col = t.Cols() - rw // overwrite end of row
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
		nc := newCell(ar, t.curF)
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

func (t *Terminal) handleExecute(cmd rune) {
	switch cmd {
	case BEL:
		// just swallow this for now
	case BS:
		t.cursorBack(1)
	case CR:
		t.carriageReturn()
	case LF, FF, VT: // libvte treats lf and ff the same, so we do too
		t.lineFeed()
		if t.isModeSet("LNM") {
			t.carriageReturn()
		}
	case TAB:
		t.stepTabs(1)
	case SO: // shift out
		t.cs.shiftOut()
	case SI: // shift out
		t.cs.shiftIn()
	default:
		slog.Error("unknown execution command", "cmd", string(cmd))
	}
}

func makeCommand(params *parameters, data string, cmd rune) string {
	return fmt.Sprintf("%s%s%c", data, params, cmd)
}

func (t *Terminal) handleCSI(params *parameters, data string, cmd rune) {
	slog.Debug("handling CSI command", "cmd", makeCommand(params, data, cmd))
	switch cmd {
	case CSI_DSR:
		t.handleDSR(params.item(0, 0), data)
	case CSI_DA:
		t.replyDeviceAttributes(data)
	case CSI_Q_MULTI:
		t.csiQ(params.item(0, 0), data)
	case CSI_XTWINOPS:
		t.xtwinops(params.item(0, 0))
	case CSI_DCH:
		if data != "" {
			return
		}
		t.deleteChars(params.itemDefaultOneIfZero(0, 1))
	case CSI_ICH:
		t.insertChars(' ', params.itemDefaultOneIfZero(0, 1))
	case CSI_ECH:
		// Insert n blank characters where n is the provided parameter
		l := t.cur.col + params.item(0, 1)
		if lastCol := t.Cols() - 1; l > lastCol {
			l = lastCol
		}
		row := t.row()
		t.fb.setCells(row, row, t.col(), l, newCell(' ', t.curF))
	case CSI_MODE_SET, CSI_MODE_RESET:
		for i := 0; i < params.numItems(); i++ {
			t.setMode(params.item(i, 0), data, cmd)
		}
	case CSI_DECSTBM:
		t.setTopBottom(params.item(0, 1), params.item(1, t.Rows()-1))
	case CSI_DECSLRM:
		t.setLeftRight(params.item(0, 1), params.item(1, t.Cols()-1))
	case CSI_IL:
		t.insertLines(params.item(0, 1))
	case CSI_DL:
		t.deleteLines(params.item(0, 1))
	case CSI_EL:
		t.eraseLine(params.item(0, 0))
	case CSI_SU:
		t.scrollRegion(params.item(0, 1))
	case CSI_SD:
		t.scrollRegion(-params.item(0, 1))
	case CSI_ED:
		t.eraseInDisplay(params.item(0, 0))
	case CSI_VPA, CSI_VPR, CSI_HPA, CSI_HPR, CSI_CUP, CSI_CUU, CSI_CUD, CSI_CUB, CSI_CUF, CSI_CNL, CSI_CPL, CSI_CHA, CSI_HVP:
		t.cursorMove(params, cmd)
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
		slog.Debug("unimplemented CSI command", "cmd", makeCommand(params, data, cmd))
	}
}

func (t *Terminal) resetTabs(params *parameters, data string) {
	if data != "?" || params.item(0, 0) != 5 {
		slog.Debug("resetTabs called without ? 5 as data and parameter", "data", data, "params", params)
	}
	cols := t.Cols()
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

func (t *Terminal) csiQ(n int, data string) {
	switch data {
	case ">":
		if n != 0 {
			slog.Debug("invalid xterm_version query", "n", n, "data", data)
			return
		}
		t.Write([]byte(fmt.Sprintf("%c%c>|gosh(%s)%c%c", ESC, DCS, GOSH_VT_VER, ESC, ST)))
	default:
		slog.Debug("unhandled CSI q", "n", n, "data", data)
	}
}

func (t *Terminal) handleDSR(n int, data string) {
	switch data {
	case "": // General device status report
		switch n {
		case 5: // We always report OK (CSI 0 n)
			t.Write([]byte(fmt.Sprintf("%c%c%d%c", ESC, CSI, 0, CSI_DSR)))
		case 6: // Provide cursor location (CSI r ; c R)
			row, col := t.row(), t.col()
			if t.isModeSet("DECOM") {
				row -= t.topMargin()
				col -= t.leftMargin()
			}
			t.Write([]byte(fmt.Sprintf("%c%c%d;%d%c", ESC, CSI, row+1, col+1, CSI_POS)))
		default:
			slog.Debug("unhandled CSI DSR request", "n", n, "data", data)
		}
	case "?": // DEC specific device status report
		switch n {
		case 6: // Provide cursor location (CSI ? r ; c R)
			t.Write([]byte(fmt.Sprintf("%c%c?%d;%d%c", ESC, CSI, t.row()+1, t.col()+1, CSI_POS)))
		case 15: // report printer status; always "not ready" (CSI ? 1 1 n)
			t.Write([]byte(fmt.Sprintf("%c%c?11%c", ESC, CSI, CSI_DSR)))
		case 25: // UDK (universal disk kit); always "unlocked" (CSI ? 2 0 n)
			t.Write([]byte(fmt.Sprintf("%c%c?%d%c", ESC, CSI, 20, CSI_DSR)))
		default:
			slog.Debug("unhandled CSI ? DSR request", "n", n, "data", data)
		}
	case ">":
		slog.Debug("swallowing xterm disable key modifiers", "n", n, "data", data)
	default:
		slog.Debug("unknown CSI DSR modifier string", "n", n, "data", data)
	}
}

func (t *Terminal) doDECALN() {
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
	case "": // primary attributes
		t.Write([]byte("\033[?62c")) // vt220
	default:
		slog.Debug("unexpected CSI device attributes request")
	}
}

func (t *Terminal) setMode(mode int, data string, state rune) {
	mid := fmt.Sprintf("%s%d", data, mode)
	m, ok := t.modes[mid]
	if !ok {
		slog.Debug("unknown mode change request", "id", mid, "state", modeStateNames[state])
		return
	}

	slog.Debug("mode change", "mid", mid, "name", m.name, "state", modeStateNames[state])

	m.setState(state)

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
	case "XTERM_ALT_BUFFER":
		t.swapFramebuffer(state)
	case "XTERM_SAVE_RESTORE":
		if state == CSI_MODE_SET {
			t.cursorSave()
		} else {
			t.cursorRestore()
		}
	case "XTERM_SAVE_ALT":
		t.swapFramebuffer(state)
		if state == CSI_MODE_SET {
			t.cursorSave()
			t.homeCursor()
			t.curF = defFmt
			t.cs = &charset{}
		} else {
			t.cursorRestore()
		}

	}
}

func (t *Terminal) swapFramebuffer(state rune) {
	if state == CSI_MODE_SET {
		t.altFb = t.fb.copy()
		t.fb = newFramebuffer(t.altFb.rows(), t.altFb.cols())
	} else {
		// This could have interesting effects, but we need to
		// ensure the buffer is the same as that expected, in
		// case the window was resized while the alt buffer
		// was visible.
		if r, c := t.fb.rows(), t.fb.cols(); r != t.altFb.rows() || c != t.altFb.cols() {
			t.altFb.resize(r, c)
		}
		t.fb = t.altFb.copy()
	}
}

func (t *Terminal) setTopBottom(top, bottom int) {
	if bottom <= top || top >= t.Rows() || (top == 0 && bottom == 1) {
		return // matches xterm
	}
	// https://vt100.net/docs/vt510-rm/DECSTBM.html
	// STBM sets the cursor to 1,1 (0,0)
	t.vertMargin = newMargin(top-1, bottom-1)
	t.homeCursor()
}

func (t *Terminal) setLeftRight(left, right int) {
	if right <= left || left >= t.Cols() || (left == 0 && right == 1) {
		return // matches xterm
	}

	// https://vt100.net/docs/vt510-rm/DECSLRM.html
	// STBM sets the cursor to 1,1 (0,0)
	t.horizMargin = newMargin(left-1, right-1)
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

	max := t.Cols() - 1
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

func (t *Terminal) insertLines(n int) {
	sr, err := t.fb.subRegion(t.row(), t.boundedMarginBottom(), t.boundedMarginLeft(), t.boundedMarginRight())
	if err != nil {
		slog.Error("invalid subregion request", "row", t.row(), "bottom", t.boundedMarginBottom(), "err", err)
		return
	}
	sr.scrollRows(-n)
}

func (t *Terminal) deleteLines(n int) {
	sr, err := t.fb.subRegion(t.row(), t.boundedMarginBottom(), t.boundedMarginLeft(), t.boundedMarginRight())
	if err != nil {
		slog.Error("invalid subregion request", "row", t.row(), "bottom", t.boundedMarginBottom(), "err", err)
		return
	}
	sr.scrollRows(n)
}

func (t *Terminal) deleteChars(n int) {

	row, col := t.row(), t.col()
	right := t.Cols()
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
	nc := t.Cols() - 1
	switch n {
	case ERASE_FROM_CUR: // to end of line
		t.fb.setCells(row, row, col, nc, dc)
	case ERASE_TO_CUR: // to start of line
		t.fb.setCells(row, row, 0, col, dc)
	case ERASE_ALL: // entire line
		t.fb.setCells(row, row, 0, nc, dc)
	default:
		slog.Error("unknown command for erase in line", "cmd", n)
	}
}

func (t *Terminal) eraseInDisplay(n int) {
	// TODO: Handle BCE properly
	switch n {
	case ERASE_FROM_CUR: // active position to end of screen, inclusive
		t.fb.resetRows(t.row()+1, t.Rows()-1)
		t.eraseLine(n)
	case ERASE_TO_CUR: // start of screen to active position, inclusive
		t.fb.resetRows(0, t.row()-1)
		t.eraseLine(n)
	case ERASE_ALL: // entire screen
		t.fb.resetRows(0, t.Rows()-1)
	default:
		slog.Error("unknown command for erase in display", "cmd", n)
	}
}
