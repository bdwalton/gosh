package vt

import (
	"fmt"
	"log/slog"
	"slices"
)

type color interface {
	fmt.Stringer
	Equal(color) bool
}

type ansiBasicColor struct {
	col int
}

func (c ansiBasicColor) Equal(other color) bool {
	switch other.(type) {
	case ansiBasicColor:
		return c.col == other.(ansiBasicColor).col
	case *ansiBasicColor:
		return c.col == other.(*ansiBasicColor).col
	default:
		return false
	}
}

func (c ansiBasicColor) String() string {
	return fmt.Sprintf("%d", c.col)
}

type ansi256Color struct {
	col int
}

func (c ansi256Color) Equal(other color) bool {
	switch other.(type) {
	case ansi256Color:
		return c.col == other.(ansi256Color).col
	case *ansi256Color:
		return c.col == other.(*ansi256Color).col
	default:
		return false
	}
}

func (c ansi256Color) String() string {
	return fmt.Sprintf("5;%d", c.col)
}

type rgbColor struct {
	rgb []int
}

func (c rgbColor) Equal(other color) bool {
	switch other.(type) {
	case rgbColor:
		o := other.(rgbColor)
		return slices.Equal(c.rgb, o.rgb)
	case *rgbColor:
		o := other.(*rgbColor)
		return slices.Equal(c.rgb, o.rgb)
	default:
		return false
	}
}

func (c rgbColor) String() string {
	return fmt.Sprintf("2;%d;%d;%d", c.rgb[0], c.rgb[1], c.rgb[2])
}

// colorFromParams takes a list of integers and interprets them as
// either a 256 color or 24-bit true color ansi sequence. It expects
// the parameters to be prefixed by either SET_FG, SET_BG that specify
// what the color will be used for, but that parameter itself is
// ignored. It returns a color and the number of parameters consumed
// by the color, including the SET* parameter. Upon error, it will
// return nil and 0 (no parameters consumed)
func colorFromParams(params []int) (color, int) {
	if len(params) < 2 {
		slog.Debug("invalid parameters to provide extended color", "params", params)
		return nil, 0
	}

	params = params[1:]
	lp := len(params)

	switch params[0] {
	case 2: // 24 bit true color
		if lp == 1 {
			return rgbColor{[]int{0, 0, 0}}, lp + 1
		}

		cols := []int{0, 0, 0}
		for i := 0; i < lp-1; i++ {
			cols[i] = params[1+i]
		}

		// TODO: Handle invalid values (!0-255)
		return rgbColor{cols}, lp + 1
	case 5: // 256 color selection
		if lp == 1 {
			return ansi256Color{0}, lp + 1
		}
		// TODO: Handle invalid values (!0-255)
		return ansi256Color{params[1]}, lp + 1
	}

	slog.Debug("invalid color type selector, returning default", "selector param", params[1])
	return nil, 0
}

// Publish common color codes as standard variables
var standardColors = map[int]color{
	FG_BLACK:   &ansiBasicColor{FG_BLACK},
	FG_RED:     &ansiBasicColor{FG_RED},
	FG_GREEN:   &ansiBasicColor{FG_GREEN},
	FG_YELLOW:  &ansiBasicColor{FG_YELLOW},
	FG_BLUE:    &ansiBasicColor{FG_BLUE},
	FG_MAGENTA: &ansiBasicColor{FG_MAGENTA},
	FG_CYAN:    &ansiBasicColor{FG_CYAN},
	FG_WHITE:   &ansiBasicColor{FG_WHITE},
	BG_BLACK:   &ansiBasicColor{BG_BLACK},
	BG_RED:     &ansiBasicColor{BG_RED},
	BG_GREEN:   &ansiBasicColor{BG_GREEN},
	BG_YELLOW:  &ansiBasicColor{BG_YELLOW},
	BG_BLUE:    &ansiBasicColor{BG_BLUE},
	BG_MAGENTA: &ansiBasicColor{BG_MAGENTA},
	BG_CYAN:    &ansiBasicColor{BG_CYAN},
	BG_WHITE:   &ansiBasicColor{BG_WHITE},
}
