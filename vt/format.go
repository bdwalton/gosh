package vt

import (
	"fmt"
	"log/slog"
)

type intensity uint8 // font intensity
type ulstyle uint8   // underline style

var defFmt = format{}

type format struct {
	fg, bg                                        color
	brightness                                    intensity
	underline                                     ulstyle
	italic, blink, reversed, invisible, strikeout bool
}

func (f *format) getFG() color {
	if f.fg == nil {
		return standardColors[FG_DEF]
	}
	return f.fg
}

func (f *format) getBG() color {
	if f.bg == nil {
		return standardColors[BG_DEF]
	}
	return f.bg
}

func (f *format) String() string {
	return fmt.Sprintf("fg: %s; bg: %s; bright: %d, underline: %d, italic: %t, blink: %t, reversed: %t, invisible: %t, strikeout: %t", f.getFG(), f.getBG(), f.brightness, f.underline, f.italic, f.blink, f.reversed, f.invisible, f.strikeout)
}

func (f format) equal(other format) bool {
	if bg := f.getBG(); !bg.equal(other.getBG()) {
		return false
	}

	if fg := f.getFG(); !fg.equal(other.getFG()) {
		return false
	}

	if f.brightness != other.brightness || f.underline != other.underline || f.italic != other.italic || f.blink != other.blink || f.reversed != other.reversed || f.invisible != other.invisible || f.strikeout != other.strikeout {
		return false
	}

	return true
}

func formatFromParams(curF format, params *parameters) format {
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
// the second argument, and if they do, they must return the number
// that should be skipped for future consideration.  TOOD: How to
// indicate errors? Just ignore and return the unmodified format? Log?
// Return error?
type formatter func(format, *parameters) format

var formatters map[int]formatter = map[int]formatter{
	RESET: func(f format, p *parameters) format { return format{} },
	// style formats
	INTENSITY_BOLD:   func(f format, p *parameters) format { f.brightness = FONT_BOLD; return f },
	INTENSITY_DIM:    func(f format, p *parameters) format { f.brightness = FONT_DIM; return f },
	ITALIC_ON:        func(f format, p *parameters) format { f.italic = true; return f },
	UNDERLINE_ON:     func(f format, p *parameters) format { f.underline = UNDERLINE_SINGLE; return f },
	BLINK_ON:         func(f format, p *parameters) format { f.blink = true; return f },
	REVERSED_ON:      func(f format, p *parameters) format { f.reversed = true; return f },
	INVISIBLE_ON:     func(f format, p *parameters) format { f.invisible = true; return f },
	STRIKEOUT_ON:     func(f format, p *parameters) format { f.strikeout = true; return f },
	DBL_UNDERLINE:    func(f format, p *parameters) format { f.underline = UNDERLINE_DOUBLE; return f },
	INTENSITY_NORMAL: func(f format, p *parameters) format { f.brightness = FONT_NORMAL; return f },
	ITALIC_OFF:       func(f format, p *parameters) format { f.italic = false; return f },
	UNDERLINE_OFF:    func(f format, p *parameters) format { f.underline = UNDERLINE_NONE; return f },
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
	FG_BRIGHT_BLACK:   basicBrightFG(FG_BLACK),
	FG_BRIGHT_RED:     basicBrightFG(FG_RED),
	FG_BRIGHT_GREEN:   basicBrightFG(FG_GREEN),
	FG_BRIGHT_YELLOW:  basicBrightFG(FG_YELLOW),
	FG_BRIGHT_BLUE:    basicBrightFG(FG_BLUE),
	FG_BRIGHT_MAGENTA: basicBrightFG(FG_MAGENTA),
	FG_BRIGHT_CYAN:    basicBrightFG(FG_CYAN),
	FG_BRIGHT_WHITE:   basicBrightFG(FG_WHITE),
	BG_BRIGHT_BLACK:   basicBrightBG(BG_BRIGHT_BLACK),
	BG_BRIGHT_RED:     basicBrightBG(BG_BRIGHT_RED),
	BG_BRIGHT_GREEN:   basicBrightBG(BG_BRIGHT_GREEN),
	BG_BRIGHT_YELLOW:  basicBrightBG(BG_BRIGHT_YELLOW),
	BG_BRIGHT_BLUE:    basicBrightBG(BG_BRIGHT_BLUE),
	BG_BRIGHT_MAGENTA: basicBrightBG(BG_BRIGHT_MAGENTA),
	BG_BRIGHT_CYAN:    basicBrightBG(BG_BRIGHT_CYAN),
	BG_BRIGHT_WHITE:   basicBrightBG(BG_BRIGHT_WHITE),
}

func basicFG(col int) func(f format, p *parameters) format {
	return func(f format, p *parameters) format {
		f.fg = standardColors[col]
		return f
	}
}

func basicBrightFG(col int) func(f format, p *parameters) format {
	return func(f format, p *parameters) format {
		f.fg = standardColors[col]
		f.brightness = FONT_BOLD
		return f
	}
}

func basicBG(col int) func(f format, p *parameters) format {
	return func(f format, p *parameters) format {
		f.bg = standardColors[col]
		return f
	}
}

func basicBrightBG(col int) func(f format, p *parameters) format {
	return func(f format, p *parameters) format {
		f.bg = ansiBasicColor{col}
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
