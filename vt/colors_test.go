package vt

import (
	"testing"
)

func TestColorAnsiString(t *testing.T) {
	cases := []struct {
		col  color
		fgbg int
		want string
	}{
		{newColor(FG_BLACK), SET_FG, "30"},
		{newColor(BG_CYAN), SET_BG, "46"},
		{newRGBColor([]int{248, 123, 0}), SET_FG, "38:2:248:123:0"},
		{newRGBColor([]int{248, 123, 0}), SET_BG, "48:2:248:123:0"},
		{newAnsiColor(165), SET_FG, "38;5;165"},
		{newAnsiColor(165), SET_BG, "48;5;165"},
	}

	for i, c := range cases {
		if got := c.col.ansiString(c.fgbg); got != c.want {
			t.Errorf("%d: Got %q, wanted %q, from %v", i, got, c.want, c.col)
		}
	}
}

func paramsFromInts(items []int) *parameters {
	return &parameters{items: items, num: len(items)}
}

func TestColorsFromParams(t *testing.T) {
	cases := []struct {
		params *parameters
		want   color
	}{
		{paramsFromInts([]int{5}), newAnsiColor(0)}, // Unspecified parameters are treated as 0
		{paramsFromInts([]int{5, 253}), newAnsiColor(253)},
		{paramsFromInts([]int{2, 253, 128, 129}), newRGBColor([]int{253, 128, 129})},
		{paramsFromInts([]int{2, 253}), newRGBColor([]int{253, 0, 0})},            // Unspecified parameters are treated as 0
		{paramsFromInts([]int{2, 253, 1}), newRGBColor([]int{253, 1, 0})},         // Unspecified parameters are treated as 0
		{paramsFromInts([]int{2, 253, 1, 32, 1}), newRGBColor([]int{253, 1, 32})}, // Additional parameters not consumed
	}

	for i, c := range cases {
		col := colorFromParams(c.params, color{})
		if !col.equal(c.want) {
			t.Errorf("%d: Got %q, wanted %q, from %v", i, col, c.want, c.params.items)
		}
	}
}

func TestColorEquality(t *testing.T) {
	cases := []struct {
		col, other color
		want       bool
	}{
		{newColor(FG_WHITE), newColor(FG_RED), false},
		{newColor(FG_WHITE), newAnsiColor(1), false},
		{newColor(FG_WHITE), newRGBColor([]int{1, 2, 3}), false},
		{newColor(FG_WHITE), color{}, false},
		{newAnsiColor(1), newRGBColor([]int{1, 2, 3}), false},
		{newAnsiColor(1), color{}, false},
		{newColor(BG_BLUE), newColor(BG_BLUE), true},
		{newAnsiColor(2), newAnsiColor(2), true},
		{newRGBColor([]int{1, 2, 3}), color{}, false},
		{newRGBColor([]int{1, 2, 3}), newRGBColor([]int{1, 2, 3}), true},
	}

	for i, c := range cases {
		if got := c.col.equal(c.other); got != c.want {
			t.Errorf("%d: Got %t, wanted %t, from %s == %s", i, got, c.want, c.col.ansiString(SET_FG), c.other.ansiString(SET_FG))
		}

	}
}
