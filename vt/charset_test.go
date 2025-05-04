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
		{&charset{g: [2]bool{true, false}}, &charset{g: [2]bool{false, true}}, false},
		{&charset{g: [2]bool{true, false}}, &charset{g: [2]bool{true, false}}, true},
		{&charset{set: 1, g: [2]bool{true, false}}, &charset{set: 1, g: [2]bool{true, false}}, true},
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
		{&charset{}, ")", '0', &charset{g: [2]bool{false, true}}},
		{&charset{}, "(", '0', &charset{g: [2]bool{true, false}}},
		{&charset{g: [2]bool{true, false}}, "(", '0', &charset{g: [2]bool{true, false}}},
		{&charset{g: [2]bool{true, false}}, ")", '0', &charset{g: [2]bool{true, true}}},
		{&charset{g: [2]bool{true, true}}, "(", 'B', &charset{g: [2]bool{false, true}}},
		{&charset{g: [2]bool{true, true}}, ")", 'B', &charset{g: [2]bool{true, false}}},
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
		{'a', &charset{set: 1, g: [2]bool{false, true}}, '▒'},
		{'+', &charset{set: 1, g: [2]bool{}}, '+'},
		{'+', &charset{g: [2]bool{true, false}}, '→'},
		{'A', &charset{g: [2]bool{true, false}}, 'A'},
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
		{&charset{set: 1, g: [2]bool{}}, &charset{set: 0}},
		{&charset{set: 1, g: [2]bool{true, false}}, &charset{set: 0, g: [2]bool{true, false}}},
		{&charset{set: 1, g: [2]bool{true, true}}, &charset{set: 0, g: [2]bool{true, true}}},
		{&charset{set: 1, g: [2]bool{false, true}}, &charset{set: 0, g: [2]bool{false, true}}},
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
		{&charset{set: 0, g: [2]bool{}}, &charset{set: 1}},
		{&charset{set: 0, g: [2]bool{true, false}}, &charset{set: 1, g: [2]bool{true, false}}},
		{&charset{set: 0, g: [2]bool{true, true}}, &charset{set: 1, g: [2]bool{true, true}}},
		{&charset{set: 0, g: [2]bool{false, true}}, &charset{set: 1, g: [2]bool{false, true}}},
	}

	for i, c := range cases {
		c.cs.shiftOut()
		if !c.cs.equal(c.want) {
			t.Errorf("%d: Got %v, wanted %v", i, c.cs, c.want)
		}
	}
}
