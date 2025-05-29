package vt

type charset struct {
	// index into g. toggled for shift in/out
	set uint8
	// glyphs (g0, g1) // false means 'B' (US ASCII; default),
	// true means '0' (DEC Special line drawing)
	g [2]bool
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
	if c.g[c.set] {
		if rr, ok := acs[r]; ok {
			return rr
		}
	}

	return r
}

func (c *charset) shiftIn() {
	c.set = 0
}

func (c *charset) shiftOut() {
	c.set = 1
}

func (c *charset) setCS(s string, r rune) {
	gset, val := 0, false // set g0 to 'B' (US ASCII) by default
	if s == ")" {
		gset = 1
	}
	if r == '0' {
		val = true
	}

	c.g[gset] = val
}

// This maps from the natural character set into the "B" character
// set. We only support ESC {),(} {0,B} for alternate charset handling
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
