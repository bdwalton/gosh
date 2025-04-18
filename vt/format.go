package vt

import (
	"fmt"
	"log/slog"
	"strings"
)

var defFmt = format{}

const FMT_RESET = "\x1b[m"

type format struct {
	fg, bg                                                 *color
	bold, underline, blink, reversed, invisible, strikeout bool
}

func (f *format) getFG() *color {
	if f.isDefaultFG() {
		return standardColors[FG_DEF]
	}

	return f.fg
}

func (f *format) isDefaultFG() bool {
	if f.fg == nil {
		return true
	}

	return false
}

func (f *format) getBG() *color {
	if f.isDefaultBG() {
		return standardColors[BG_DEF]
	}
	return f.bg
}

func (f *format) isDefaultBG() bool {
	if f.bg == nil {
		return true
	}

	return false
}

func (src format) diff(dest format) []byte {
	if !dest.equal(src) && dest.equal(defFmt) {
		return []byte(FMT_RESET)
	}

	var sb, ts strings.Builder

	if dfg := dest.getFG(); !dfg.equal(src.getFG()) {
		sb.WriteString(fmt.Sprintf("%c%c%s%c", ESC, ESC_CSI, dfg.getAnsiString(SET_FG), CSI_SGR))
	}

	if dbg := dest.getBG(); !dbg.equal(src.getBG()) {
		sb.WriteString(fmt.Sprintf("%c%c%s%c", ESC, ESC_CSI, dbg.getAnsiString(SET_BG), CSI_SGR))
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
		sb.Write([]byte{ESC, ESC_CSI})
		sb.WriteString(ts.String())
		sb.WriteRune(CSI_SGR)
	}

	return []byte(sb.String())
}

func (f *format) String() string {
	return fmt.Sprintf("fg: %s; bg: %s; bold: %t, underline: %t, blink: %t, reversed: %t, invisible: %t, strikeout: %t", f.getFG().getAnsiString(SET_FG), f.getBG().getAnsiString(SET_BG), f.bold, f.underline, f.blink, f.reversed, f.invisible, f.strikeout)
}

func (f format) equal(other format) bool {
	if bg := f.getBG(); !bg.equal(other.getBG()) {
		return false
	}

	if fg := f.getFG(); !fg.equal(other.getFG()) {
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
	ni := params.numItems()
	switch ni {
	case 0: // CSI m
		f = formatters[RESET](f, nil)
	default:
		for {
			item, ok := params.consumeItem()
			if !ok {
				break
			}

			fmer, ok := formatters[item]
			if ok {
				f = fmer(f, params)
			} else {
				slog.Debug("unimplemented CSI format option", "param", item)
			}
		}
	}

	return f
}

// formatter functions take the current format and return a modified
// format. they may consume additional paramters, made available in
// the second argument
type formatter func(format, *parameters) format

var formatters map[int]formatter = map[int]formatter{
	RESET: func(f format, p *parameters) format { return format{} },
	// style formats
	INTENSITY_BOLD:   func(f format, p *parameters) format { f.bold = true; return f },
	UNDERLINE_ON:     func(f format, p *parameters) format { f.underline = true; return f },
	BLINK_ON:         func(f format, p *parameters) format { f.blink = true; return f },
	REVERSED_ON:      func(f format, p *parameters) format { f.reversed = true; return f },
	INVISIBLE_ON:     func(f format, p *parameters) format { f.invisible = true; return f },
	STRIKEOUT_ON:     func(f format, p *parameters) format { f.strikeout = true; return f },
	INTENSITY_NORMAL: func(f format, p *parameters) format { f.bold = false; return f },
	UNDERLINE_OFF:    func(f format, p *parameters) format { f.underline = false; return f },
	BLINK_OFF:        func(f format, p *parameters) format { f.blink = false; return f },
	REVERSED_OFF:     func(f format, p *parameters) format { f.reversed = false; return f },
	INVISIBLE_OFF:    func(f format, p *parameters) format { f.invisible = false; return f },
	STRIKEOUT_OFF:    func(f format, p *parameters) format { f.strikeout = false; return f },
	// colors
	FG_BLACK:          basicFG(FG_BLACK),
	FG_RED:            basicFG(FG_RED),
	FG_GREEN:          basicFG(FG_GREEN),
	FG_YELLOW:         basicFG(FG_YELLOW),
	FG_BLUE:           basicFG(FG_BLUE),
	FG_MAGENTA:        basicFG(FG_MAGENTA),
	FG_CYAN:           basicFG(FG_CYAN),
	FG_WHITE:          basicFG(FG_WHITE),
	SET_FG:            extendedFG(),
	FG_DEF:            basicFG(FG_DEF),
	BG_BLACK:          basicBG(BG_BLACK),
	BG_RED:            basicBG(BG_RED),
	BG_GREEN:          basicBG(BG_GREEN),
	BG_YELLOW:         basicBG(BG_YELLOW),
	BG_BLUE:           basicBG(BG_BLUE),
	BG_MAGENTA:        basicBG(BG_MAGENTA),
	BG_CYAN:           basicBG(BG_CYAN),
	BG_WHITE:          basicBG(BG_WHITE),
	SET_BG:            extendedBG(),
	BG_DEF:            basicBG(BG_DEF),
	FG_BRIGHT_BLACK:   basicFG(FG_BRIGHT_BLACK),
	FG_BRIGHT_RED:     basicFG(FG_BRIGHT_RED),
	FG_BRIGHT_GREEN:   basicFG(FG_BRIGHT_GREEN),
	FG_BRIGHT_YELLOW:  basicFG(FG_BRIGHT_YELLOW),
	FG_BRIGHT_BLUE:    basicFG(FG_BRIGHT_BLUE),
	FG_BRIGHT_MAGENTA: basicFG(FG_BRIGHT_MAGENTA),
	FG_BRIGHT_CYAN:    basicFG(FG_BRIGHT_CYAN),
	FG_BRIGHT_WHITE:   basicFG(FG_BRIGHT_WHITE),
	BG_BRIGHT_BLACK:   basicBG(BG_BRIGHT_BLACK),
	BG_BRIGHT_RED:     basicBG(BG_BRIGHT_RED),
	BG_BRIGHT_GREEN:   basicBG(BG_BRIGHT_GREEN),
	BG_BRIGHT_YELLOW:  basicBG(BG_BRIGHT_YELLOW),
	BG_BRIGHT_BLUE:    basicBG(BG_BRIGHT_BLUE),
	BG_BRIGHT_MAGENTA: basicBG(BG_BRIGHT_MAGENTA),
	BG_BRIGHT_CYAN:    basicBG(BG_BRIGHT_CYAN),
	BG_BRIGHT_WHITE:   basicBG(BG_BRIGHT_WHITE),
}

func basicFG(col int) func(f format, p *parameters) format {
	return func(f format, p *parameters) format {
		f.fg = standardColors[col]
		return f
	}
}

func basicBG(col int) func(f format, p *parameters) format {
	return func(f format, p *parameters) format {
		f.bg = standardColors[col]
		return f
	}
}

func extendedFG() func(f format, p *parameters) format {
	return func(f format, p *parameters) format {
		f.fg = colorFromParams(p, standardColors[FG_DEF])
		return f
	}
}

func extendedBG() func(f format, p *parameters) format {
	return func(f format, p *parameters) format {
		f.bg = colorFromParams(p, standardColors[FG_DEF])
		return f
	}
}
