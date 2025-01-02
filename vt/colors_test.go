package vt

import (
	"testing"
)

func TestStringification(t *testing.T) {
	cases := []struct {
		col  color
		want string
	}{
		{standardColors[FG_BLACK], "30"},
		{standardColors[BG_CYAN], "46"},
		{rgbColor{[]int{248, 123, 0}}, "2;248;123;0"},
		{ansi256Color{165}, "5;165"},
	}

	for i, c := range cases {
		if got := c.col.String(); got != c.want {
			t.Errorf("%d: Got %q, wanted %q, from %v", i, got, c.want, c.col)
		}
	}
}

func TestColorsFromParams(t *testing.T) {
	cases := []struct {
		params []int
		want   string
		wantN  int
	}{
		{[]int{SET_FG, 5}, "5;0", 2}, // Unspecified parameters are treated as 0
		{[]int{SET_BG, 5, 253}, "5;253", 3},
		{[]int{SET_FG, 2, 253, 128, 129}, "2;253;128;129", 5},
		{[]int{SET_BG, 2, 253}, "2;253;0;0", 3},    // Unspecified parameters are treated as 0
		{[]int{SET_FG, 2, 253, 1}, "2;253;1;0", 4}, // Unspecified parameters are treated as 0
	}

	for i, c := range cases {
		col, n := colorFromParams(c.params)
		if col == nil || n != c.wantN || col.String() != c.want {
			t.Errorf("%d: Got %q (%d), wanted %q (%d), from %v", i, col, n, c.want, c.wantN, c.params)
		}
	}
}

func TestEquality(t *testing.T) {
	cases := []struct {
		col, other color
		want       bool
	}{
		{standardColors[FG_WHITE], standardColors[FG_RED], false},
		{standardColors[FG_WHITE], ansi256Color{1}, false},
		{standardColors[FG_WHITE], rgbColor{[]int{1, 2, 3}}, false},
		{standardColors[FG_WHITE], nil, false},
		{ansi256Color{1}, rgbColor{[]int{1, 2, 3}}, false},
		{ansi256Color{1}, nil, false},
		{standardColors[BG_BLUE], standardColors[BG_BLUE], true},
		{ansi256Color{2}, ansi256Color{2}, true},
		{ansi256Color{2}, &ansi256Color{2}, true},
		{&ansi256Color{2}, ansi256Color{2}, true},
		{ansi256Color{4}, &ansi256Color{4}, true},
		{&ansi256Color{3}, &ansi256Color{3}, true},
		{rgbColor{[]int{1, 2, 3}}, nil, false},
		{rgbColor{[]int{1, 2, 3}}, rgbColor{[]int{1, 2, 3}}, true},
		{rgbColor{[]int{1, 2, 3}}, &rgbColor{[]int{1, 2, 3}}, true},
		{&rgbColor{[]int{3, 4, 5}}, &rgbColor{[]int{3, 4, 5}}, true},
	}

	for i, c := range cases {
		if got := c.col.equal(c.other); got != c.want {
			t.Errorf("%d: Got %t, wanted %t, from %s == %s", i, got, c.want, c.col, c.other)
		}

	}
}
