package vt

import (
	"fmt"
	"log/slog"
	"slices"
)

const (
	UNSET = iota
	BASIC
	ANSI256
	RGB
)

type color struct {
	colType int
	data    []int
}

func (c color) equal(other color) bool {
	return c.colType == other.colType && slices.Equal(c.data, other.data)
}

func (c color) getAnsiString(set int) string {
	switch c.colType {
	case UNSET:
		if set == SET_FG {
			return fmt.Sprintf("%d", FG_DEF)
		}
		return fmt.Sprintf("%d", BG_DEF)
	case BASIC:
		return fmt.Sprintf("%d", c.data[0])
	case ANSI256:
		return fmt.Sprintf("%d;5;%d", set, c.data[0])
	case RGB:
		return fmt.Sprintf("%d;2;%d;%d;%d", set, c.data[0], c.data[1], c.data[2])
	default:
		slog.Error("invalid color type")
		return ""
	}
}

func newColor(col int) color {
	return color{colType: BASIC, data: []int{col}}
}

func newAnsiColor(col int) color {
	return color{colType: ANSI256, data: []int{col}}
}

func newRGBColor(cols []int) color {
	return color{colType: RGB, data: cols}
}

// colorFromParams takes a paramter object and interprets it as
// either a 256 color or 24-bit true color ansi sequence. It expects
// the parameters to be prefixed by either SET_FG, SET_BG that specify
// what the color will be used for, but that parameter itself is
// ignored. It returns a color and the number of parameters consumed
// by the color, including the SET* parameter. Upon error, it will
// return nil and 0 (no parameters consumed)
func colorFromParams(params *parameters, def color) color {
	cm, ok := params.consumeItem()
	if !ok {
		slog.Debug("invalid parameters to provide extended color", "params", params.items)
		return def
	}

	switch cm { // consume the color mode
	case 2: // 24 bit true color
		cols := []int{0, 0, 0}
		var ok bool
		for i := 0; i < len(cols); i++ {
			cols[i], ok = params.consumeItem()
			if !ok {
				break
			}
		}

		// TODO: Handle invalid values (!0-255)
		return newRGBColor(cols)
	case 5: // 256 color selection
		// TODO: Handle invalid values (!0-255)
		item, ok := params.consumeItem()
		if !ok {
			return newAnsiColor(0)
		}
		return newAnsiColor(item)
	}

	slog.Debug("invalid color type selector, returning default", "selector param", cm)
	return def
}
