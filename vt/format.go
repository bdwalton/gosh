package vt

import (
	"fmt"
	"log/slog"
	"strings"
)

var defFmt = &format{fg: newDefaultColor(), bg: newDefaultColor()}

const FMT_RESET = "\x1b[m"

const (
	BOLD       = 1 << 0
	FAINT      = 1 << 1
	ITALIC     = 1 << 2
	BOLD_FAINT = 1 << 3 // it is valid to set both, we handle is specially
	UNDERLINE  = 1 << 4
	BLINK      = 1 << 5
	REVERSED   = 1 << 6
	INVISIBLE  = 1 << 7
	STRIKEOUT  = 1 << 8
)

var attrs = []uint16{BOLD_FAINT, BOLD, FAINT, ITALIC, UNDERLINE, BLINK, REVERSED, INVISIBLE, STRIKEOUT}
var attrToggle = map[uint16]map[bool]string{
	BOLD:       map[bool]string{true: fmt.Sprintf("%d", INTENSITY_BOLD), false: fmt.Sprintf("%d", INTENSITY_NORMAL)},
	FAINT:      map[bool]string{true: fmt.Sprintf("%d", INTENSITY_FAINT), false: fmt.Sprintf("%d", INTENSITY_NORMAL)},
	ITALIC:     map[bool]string{true: fmt.Sprintf("%d", ITALIC_ON), false: fmt.Sprintf("%d", ITALIC_OFF)},
	BOLD_FAINT: map[bool]string{true: fmt.Sprintf("%d;%d", INTENSITY_BOLD, INTENSITY_FAINT), false: fmt.Sprintf("%d", INTENSITY_NORMAL)},
	UNDERLINE:  map[bool]string{true: fmt.Sprintf("%d", UNDERLINE_ON), false: fmt.Sprintf("%d", UNDERLINE_OFF)},
	BLINK:      map[bool]string{true: fmt.Sprintf("%d", BLINK_ON), false: fmt.Sprintf("%d", BLINK_OFF)},
	REVERSED:   map[bool]string{true: fmt.Sprintf("%d", REVERSED_ON), false: fmt.Sprintf("%d", REVERSED_OFF)},
	INVISIBLE:  map[bool]string{true: fmt.Sprintf("%d", INVISIBLE_ON), false: fmt.Sprintf("%d", INVISIBLE_OFF)},
	STRIKEOUT:  map[bool]string{true: fmt.Sprintf("%d", STRIKEOUT_ON), false: fmt.Sprintf("%d", STRIKEOUT_OFF)},
}

type format struct {
	fg, bg color
	attrs  uint16 // a bitmap of which of the attrs (^ above) are enabled
}

func (f *format) setAttr(attr uint16, val bool) {
	if val {
		f.attrs |= attr
	} else {
		f.attrs &^= attr
	}
}

func (f *format) attrIsSet(attr uint16) bool {
	return (f.attrs & attr) != 0
}

func (src *format) diff(dest *format) []byte {
	if !dest.equal(src) && dest.equal(defFmt) {
		return []byte(FMT_RESET)
	}

	var sb, ts strings.Builder

	if !dest.fg.equal(src.fg) {
		sb.WriteString(fmt.Sprintf("%c%c%s%c", ESC, CSI, dest.fg.ansiString(SET_FG), CSI_SGR))
	}

	if !dest.bg.equal(src.bg) {
		sb.WriteString(fmt.Sprintf("%c%c%s%c", ESC, CSI, dest.bg.ansiString(SET_BG), CSI_SGR))
	}

	for _, attr := range attrs {
		if da := dest.attrIsSet(attr); src.attrIsSet(attr) != da {
			if ts.Len() > 0 {
				ts.WriteByte(';')
			}
			ts.WriteString(attrToggle[attr][da])
		}
	}

	if ts.Len() > 0 {
		sb.Write([]byte{ESC, CSI})
		sb.WriteString(ts.String())
		sb.WriteRune(CSI_SGR)
	}

	return []byte(sb.String())
}

func (f *format) String() string {
	return fmt.Sprintf("fg: %s; bg: %s; bold: %t, underline: %t, blink: %t, reversed: %t, invisible: %t, strikeout: %t", f.fg.ansiString(SET_FG), f.fg.ansiString(SET_BG), f.attrIsSet(BOLD), f.attrIsSet(UNDERLINE), f.attrIsSet(BLINK), f.attrIsSet(REVERSED), f.attrIsSet(INVISIBLE), f.attrIsSet(STRIKEOUT))
}

func (f *format) equal(other *format) bool {
	if !f.bg.equal(other.bg) {
		return false
	}

	if !f.fg.equal(other.fg) {
		return false
	}

	if f.attrs != other.attrs {
		return false
	}

	return true
}

func applyFormat(curF *format, params *parameters) *format {
	if params.numItems() == 0 {
		return defFmt
	}
	f := &format{bg: curF.bg, fg: curF.fg, attrs: curF.attrs}

	for {
		item, ok := params.consumeItem()
		if !ok {
			break
		}

		switch {
		case item == RESET:
			f = defFmt
		case item == INTENSITY_BOLD:
			if f.attrIsSet(BOLD_FAINT) {
				// already handled
			} else {
				if f.attrIsSet(FAINT) {
					f.setAttr(FAINT, false)
					f.setAttr(BOLD_FAINT, true)
				} else {
					f.setAttr(BOLD, true)
				}
			}
		case item == INTENSITY_FAINT:
			if f.attrIsSet(BOLD_FAINT) {
				// already handled
			} else {
				if f.attrIsSet(BOLD) {
					f.setAttr(BOLD, false)
					f.setAttr(BOLD_FAINT, true)
				} else {
					f.setAttr(FAINT, true)
				}
			}
		case item == INTENSITY_NORMAL:
			f.setAttr(BOLD|FAINT|BOLD_FAINT, false)
		case item == UNDERLINE_ON || item == UNDERLINE_OFF:
			f.setAttr(UNDERLINE, item < 10)
		case item == ITALIC_ON || item == ITALIC_OFF:
			f.setAttr(ITALIC, item < 10)
		case item == BLINK_ON || item == BLINK_OFF:
			f.setAttr(BLINK, item < 10)
		case item == REVERSED_ON || item == REVERSED_OFF:
			f.setAttr(REVERSED, item < 10)
		case item == INVISIBLE_ON || item == INVISIBLE_OFF:
			f.setAttr(INVISIBLE, item < 10)
		case item == STRIKEOUT_ON || item == STRIKEOUT_OFF:
			f.setAttr(STRIKEOUT, item < 10)
		case (item >= 30 && item <= 37) || (item >= 90 && item <= 97) || item == 39:
			// item == 39 is foreground
			// default. we treat that as a regular
			// color because we're relying on the
			// vt emulation on the client side
			// doing the right thing.
			f.fg = newColor(item)
		case item == 38:
			f.fg = colorFromParams(params, color{})
		case (item >= 40 && item <= 47) || (item >= 100 && item <= 107) || item == 49:
			// item == 49 is background
			// default. we treat that as a regular
			// color because we're relying on the
			// vt emulation on the client side
			// doing the right thing.
			f.bg = newColor(item)
		case item == 48:
			f.bg = colorFromParams(params, color{})
		default:
			slog.Debug("unimplemented CSI format option", "param", item)
		}
	}

	return f
}
