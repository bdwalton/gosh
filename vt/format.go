package vt

import (
	"fmt"
	"log/slog"
)

type intensity uint8 // font intensity
type ulstyle uint8   // underline style

// TODO: Validate these as sane, but for now they'll work
var defFG = ansiBasicColor{FG_WHITE}
var defBG = ansiBasicColor{BG_BLACK}

type format struct {
	fg, bg                                        color
	brightness                                    intensity
	underline                                     ulstyle
	italic, blink, reversed, invisible, strikeout bool
}

func (f *format) getFG() color {
	if f.fg == nil {
		return defFG
	}
	return f.fg
}

func (f *format) getBG() color {
	if f.bg == nil {
		return defBG
	}
	return f.bg
}

func (f *format) String() string {
	return fmt.Sprintf("fg: %s; bg: %s; bright: %d, underline: %d, italic: %t, blink: %t, reversed: %t, invisible: %t, strikeout: %t", f.fg, f.bg, f.brightness, f.underline, f.italic, f.blink, f.reversed, f.invisible, f.strikeout)
}

func (f format) equal(other format) bool {
	if f.bg != nil {
		if !f.bg.equal(other.bg) {
			return false
		}
	}

	if f.fg != nil {
		if !f.fg.equal(other.fg) {
			return false
		}
	}

	if f.brightness != other.brightness || f.underline != other.underline || f.italic != other.italic || f.blink != other.blink || f.reversed != other.reversed || f.invisible != other.invisible || f.strikeout != other.strikeout {
		return false
	}

	return true
}

func formatFromParams(curF format, params []int) format {
	f := curF
	switch len(params) {
	case 0: // CSI m
		f, _ = formatters[RESET](f, nil)
	default:
		for i := 0; i < len(params); {
			fmer, ok := formatters[params[i]]
			if ok {
				var n int
				f, n = fmer(f, params[i:])
				// We always consume the current parameter,
				// and to avoid this function needing to know
				// about how many extra params are consumed,
				// we just pass current param and any
				// remaining ones into the formatter. If the
				// formatter indicates it consumes 1 extra
				// param, we need to step forward by 2 places
				// in the params slice. If it consumes zero
				// extra params, we still need to step forward
				// 1. So n+1 ensures we always consume the
				// current parameter here.
				i += n + 1
			} else {
				slog.Debug("unimplemented CSI format option", "param", params[i], "remaining", params[i:])
				i += 1
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
type formatter func(format, []int) (format, int)

var formatters map[int]formatter = map[int]formatter{
	RESET: func(f format, p []int) (format, int) { return format{}, 0 },
	// style formats
	INTENSITY_BOLD:   func(f format, p []int) (format, int) { f.brightness = FONT_BOLD; return f, 0 },
	INTENSITY_DIM:    func(f format, p []int) (format, int) { f.brightness = FONT_DIM; return f, 0 },
	ITALIC_ON:        func(f format, p []int) (format, int) { f.italic = true; return f, 0 },
	UNDERLINE_ON:     func(f format, p []int) (format, int) { f.underline = UNDERLINE_SINGLE; return f, 0 },
	BLINK_ON:         func(f format, p []int) (format, int) { f.blink = true; return f, 0 },
	REVERSED_ON:      func(f format, p []int) (format, int) { f.reversed = true; return f, 0 },
	INVISIBLE_ON:     func(f format, p []int) (format, int) { f.invisible = true; return f, 0 },
	STRIKEOUT_ON:     func(f format, p []int) (format, int) { f.strikeout = true; return f, 0 },
	DBL_UNDERLINE:    func(f format, p []int) (format, int) { f.underline = UNDERLINE_DOUBLE; return f, 0 },
	INTENSITY_NORMAL: func(f format, p []int) (format, int) { f.brightness = FONT_NORMAL; return f, 0 },
	ITALIC_OFF:       func(f format, p []int) (format, int) { f.italic = false; return f, 0 },
	UNDERLINE_OFF:    func(f format, p []int) (format, int) { f.underline = UNDERLINE_NONE; return f, 0 },
	BLINK_OFF:        func(f format, p []int) (format, int) { f.blink = false; return f, 0 },
	REVERSED_OFF:     func(f format, p []int) (format, int) { f.reversed = false; return f, 0 },
	INVISIBLE_OFF:    func(f format, p []int) (format, int) { f.invisible = false; return f, 0 },
	STRIKEOUT_OFF:    func(f format, p []int) (format, int) { f.strikeout = false; return f, 0 },
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
	BG_DEF:            basicFG(BG_DEF),
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

func basicFG(col int) func(f format, p []int) (format, int) {
	return func(f format, p []int) (format, int) {
		f.fg = standardColors[col]
		return f, 0
	}
}

func basicBrightFG(col int) func(f format, p []int) (format, int) {
	return func(f format, p []int) (format, int) {
		f.fg = standardColors[col]
		f.brightness = FONT_BOLD
		return f, 0
	}
}

func basicBG(col int) func(f format, p []int) (format, int) {
	return func(f format, p []int) (format, int) {
		f.bg = standardColors[col]
		return f, 0
	}
}

func basicBrightBG(col int) func(f format, p []int) (format, int) {
	return func(f format, p []int) (format, int) {
		f.bg = ansiBasicColor{col}
		return f, 0
	}
}

func extendedFG() func(f format, p []int) (format, int) {
	return func(f format, p []int) (format, int) {
		c, n := colorFromParams(p)
		if n == 0 {
			// We will always indicate consumption of the
			// SET* parameter, even if we couldn't
			// determine a color
			return f, n + 1
		}

		f.fg = c
		return f, n
	}
}

func extendedBG() func(f format, p []int) (format, int) {
	return func(f format, p []int) (format, int) {
		c, n := colorFromParams(p)
		if n == 0 {
			// We will always indicate consumption of the
			// SET* parameter, even if we couldn't
			// determine a color
			return f, n + 1
		}

		f.bg = c
		return f, n
	}
}
