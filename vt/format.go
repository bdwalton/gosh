package vt

import (
	"fmt"
	"log/slog"
	"strings"
)

var defFmt = format{}

const FMT_RESET = "\x1b[m"

type format struct {
	fg, bg                                                 color
	bold, underline, blink, reversed, invisible, strikeout bool
}

func (src format) diff(dest format) []byte {
	if !dest.equal(src) && dest.equal(defFmt) {
		return []byte(FMT_RESET)
	}

	var sb, ts strings.Builder

	if !dest.fg.equal(src.fg) {
		sb.WriteString(fmt.Sprintf("%c%c%s%c", ESC, CSI, dest.fg.getAnsiString(SET_FG), CSI_SGR))
	}

	if !dest.bg.equal(src.bg) {
		sb.WriteString(fmt.Sprintf("%c%c%s%c", ESC, CSI, dest.bg.getAnsiString(SET_BG), CSI_SGR))
	}

	if src.bold != dest.bold {
		if dest.bold {
			ts.WriteString(fmt.Sprintf("%d", INTENSITY_BOLD))
		} else {
			ts.WriteString(fmt.Sprintf("%d", INTENSITY_NORMAL))
		}
	}

	if src.underline != dest.underline {
		if ts.Len() > 0 {
			ts.WriteByte(';')
		}
		if dest.underline {
			ts.WriteString(fmt.Sprintf("%d", UNDERLINE_ON))
		} else {
			ts.WriteString(fmt.Sprintf("%d", UNDERLINE_OFF))
		}
	}

	if src.blink != dest.blink {
		if ts.Len() > 0 {
			ts.WriteByte(';')
		}
		b := BLINK_ON
		if !dest.blink {
			b = BLINK_OFF
		}
		ts.WriteString(fmt.Sprintf("%d", b))
	}

	if src.reversed != dest.reversed {
		if ts.Len() > 0 {
			ts.WriteByte(';')
		}
		r := REVERSED_ON
		if !dest.reversed {
			r = REVERSED_OFF
		}
		ts.WriteString(fmt.Sprintf("%d", r))
	}

	if src.invisible != dest.invisible {
		if ts.Len() > 0 {
			ts.WriteByte(';')
		}
		iv := INVISIBLE_ON
		if !dest.invisible {
			iv = INVISIBLE_OFF
		}
		ts.WriteString(fmt.Sprintf("%d", iv))
	}

	if src.strikeout != dest.strikeout {
		if ts.Len() > 0 {
			ts.WriteByte(';')
		}
		s := STRIKEOUT_ON
		if !dest.strikeout {
			s = STRIKEOUT_OFF
		}
		ts.WriteString(fmt.Sprintf("%d", s))
	}

	if ts.Len() > 0 {
		sb.Write([]byte{ESC, CSI})
		sb.WriteString(ts.String())
		sb.WriteRune(CSI_SGR)
	}

	return []byte(sb.String())
}

func (f *format) String() string {
	return fmt.Sprintf("fg: %s; bg: %s; bold: %t, underline: %t, blink: %t, reversed: %t, invisible: %t, strikeout: %t", f.fg.getAnsiString(SET_FG), f.fg.getAnsiString(SET_BG), f.bold, f.underline, f.blink, f.reversed, f.invisible, f.strikeout)
}

func (f format) equal(other format) bool {
	if !f.bg.equal(other.bg) {
		return false
	}

	if !f.fg.equal(other.fg) {
		return false
	}

	if f.bold != other.bold || f.underline != other.underline || f.blink != other.blink || f.reversed != other.reversed || f.invisible != other.invisible || f.strikeout != other.strikeout {
		return false
	}

	return true
}

func formatFromParams(curF format, params *parameters) format {
	slog.Debug("consuming SGR formatting parameters", "params", params)
	f := curF
	// CSI m
	if params.numItems() == 0 {
		return format{}
	}

	for {
		item, ok := params.consumeItem()
		if !ok {
			break
		}

		switch {
		case item == RESET:
			f = format{}
		case item == INTENSITY_BOLD || item == INTENSITY_NORMAL:
			f.bold = (item < 10)
		case item == UNDERLINE_ON || item == UNDERLINE_OFF:
			f.underline = (item < 10)
		case item == BLINK_ON || item == BLINK_OFF:
			f.blink = (item < 10)
		case item == REVERSED_ON || item == REVERSED_OFF:
			f.reversed = (item < 10)
		case item == INVISIBLE_ON || item == INVISIBLE_OFF:
			f.invisible = (item < 10)
		case item == STRIKEOUT_ON || item == STRIKEOUT_OFF:
			f.strikeout = (item < 10)
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
