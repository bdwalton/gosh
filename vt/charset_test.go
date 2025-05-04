package vt

import (
	"testing"
)

func TestCharsetEqual(t *testing.T) {
	cases := []struct {
		cs1, cs2 *charset
		want     bool
	}{
		{&charset{}, &charset{}, true},
		{&charset{set: 1}, &charset{}, false},
		{&charset{g: [2]uint8{1, 0}}, &charset{g: [2]uint8{0, 1}}, false},
		{&charset{g: [2]uint8{1, 0}}, &charset{g: [2]uint8{1, 0}}, true},
		{&charset{set: 1, g: [2]uint8{1, 0}}, &charset{set: 1, g: [2]uint8{1, 0}}, true},
	}

	for i, c := range cases {
		if got := c.cs1.equal(c.cs2); got != c.want {
			t.Errorf("%d: Got %v.equal(%v) = %t, wanted %t", i, c.cs1, c.cs2, got, c.want)
		}
	}
}

func TestSetCS(t *testing.T) {
	cases := []struct {
		cs    *charset
		which string
		csv   rune
		want  *charset
	}{
		{&charset{}, ")", '0', &charset{g: [2]uint8{0, 1}}},
		{&charset{}, "(", '0', &charset{g: [2]uint8{1, 0}}},
		{&charset{g: [2]uint8{1, 0}}, "(", '0', &charset{g: [2]uint8{1, 0}}},
		{&charset{g: [2]uint8{1, 0}}, ")", '0', &charset{g: [2]uint8{1, 1}}},
		{&charset{g: [2]uint8{1, 1}}, "(", 'B', &charset{g: [2]uint8{0, 1}}},
		{&charset{g: [2]uint8{1, 1}}, ")", 'B', &charset{g: [2]uint8{1, 0}}},
	}

	for i, c := range cases {
		c.cs.setCS(c.which, c.csv)
		if !c.cs.equal(c.want) {
			t.Errorf("%d: Got %v, wanted %v", i, c.cs, c.want)
		}
	}
}

func TestRuneFor(t *testing.T) {
	cases := []struct {
		in   rune
		cs   *charset
		want rune
	}{
		{'a', &charset{}, 'a'},
		{'a', &charset{set: 1}, 'a'},
		{'a', &charset{set: 1, g: [2]uint8{0, 1}}, '▒'},
		{'+', &charset{set: 1, g: [2]uint8{0, 0}}, '+'},
		{'+', &charset{g: [2]uint8{1, 0}}, '→'},
		{'A', &charset{g: [2]uint8{1, 0}}, 'A'},
	}

	for i, c := range cases {
		if got := c.cs.runeFor(c.in); got != c.want {
			t.Errorf("%d: Got %q, wanted %q", i, got, c.want)
		}
	}
}

func TestShiftIn(t *testing.T) {
	cases := []struct {
		cs   *charset
		want *charset
	}{
		{&charset{}, &charset{set: 0}},
		{&charset{set: 0}, &charset{set: 0}},
		{&charset{set: 1}, &charset{set: 0}},
		{&charset{set: 1, g: [2]uint8{0, 0}}, &charset{set: 0}},
		{&charset{set: 1, g: [2]uint8{1, 0}}, &charset{set: 0, g: [2]uint8{1, 0}}},
		{&charset{set: 1, g: [2]uint8{1, 1}}, &charset{set: 0, g: [2]uint8{1, 1}}},
		{&charset{set: 1, g: [2]uint8{0, 1}}, &charset{set: 0, g: [2]uint8{0, 1}}},
	}

	for i, c := range cases {
		c.cs.shiftIn()
		if !c.cs.equal(c.want) {
			t.Errorf("%d: Got %v, wanted %v", i, c.cs, c.want)
		}
	}
}

func TestShiftOut(t *testing.T) {
	cases := []struct {
		cs   *charset
		want *charset
	}{
		{&charset{}, &charset{set: 1}},
		{&charset{set: 0}, &charset{set: 1}},
		{&charset{set: 1}, &charset{set: 1}},
		{&charset{set: 0, g: [2]uint8{0, 0}}, &charset{set: 1}},
		{&charset{set: 0, g: [2]uint8{1, 0}}, &charset{set: 1, g: [2]uint8{1, 0}}},
		{&charset{set: 0, g: [2]uint8{1, 1}}, &charset{set: 1, g: [2]uint8{1, 1}}},
		{&charset{set: 0, g: [2]uint8{0, 1}}, &charset{set: 1, g: [2]uint8{0, 1}}},
	}

	for i, c := range cases {
		c.cs.shiftOut()
		if !c.cs.equal(c.want) {
			t.Errorf("%d: Got %v, wanted %v", i, c.cs, c.want)
		}
	}
}
