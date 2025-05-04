package vt

import (
	"log/slog"
)

type charset struct {
	set uint8    // index into g. toggled for shift in/out
	g   [2]uint8 // glyphs (g0, g1) // val = 0 means 'B' (US ASCII), 1 means '0' (DEC Special line drawing)
}

func (c *charset) copy() *charset {
	return &charset{
		set: c.set,
		g:   c.g,
	}
}

func (c *charset) equal(other *charset) bool {
	return c.set == other.set && c.g[0] == other.g[0] && c.g[1] == other.g[1]
}

func (c *charset) runeFor(r rune) rune {
	if c.g[c.set] == 1 {
		if rr, ok := acs[r]; ok {
			slog.Debug("replacing rune", "r", string(r), "or", string(rr))
			return rr
		}
	}

	slog.Debug("not replacing rune", "r", string(r))
	return r
}

func (c *charset) shiftIn() {
	c.set = 0
}

func (c *charset) shiftOut() {
	c.set = 1
}

func (c *charset) setCS(s string, r rune) {
	gset, val := 0, 0 // set g0 to 0 by default
	if s == ")" {
		gset = 1
	}
	if r == '0' {
		val = 1
	}

	c.g[gset] = uint8(val)
}

// This maps from the natural character set into the "B" character
// set. We only support ESC {),()} {0,B} for alternate charset handling
var acs = map[rune]rune{
	'+': '→',
	',': '←',
	'-': '↑',
	'.': '↓',
	'0': '▮',
	'`': '◆',
	'a': '▒',
	'b': '␉',
	'c': '␌',
	'd': '␍',
	'e': '␊',
	'f': '°',
	'g': '±',
	'h': '␤',
	'i': '␋',
	'j': '┘',
	'k': '┐',
	'l': '┌',
	'm': '└',
	'n': '┼',
	'o': '⎺',
	'p': '⎻',
	'q': '─',
	'r': '⎼',
	's': '⎽',
	't': '├',
	'u': '┤',
	'v': '┴',
	'w': '┬',
	'x': '│',
	'y': '≤',
	'z': '≥',
	'{': 'π',
	'|': '≠',
	'}': '£',
	'~': '·',
}
