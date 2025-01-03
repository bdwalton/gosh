package vt

import (
	"testing"
)

func TestFormatEquality(t *testing.T) {
	cases := []struct {
		f1, f2 format
		want   bool
	}{
		{
			format{bg: standardColors[BG_RED], italic: true},
			format{bg: standardColors[BG_RED], italic: true},
			true,
		},
		{
			format{bg: standardColors[FG_RED], italic: true},
			format{bg: standardColors[BG_RED], italic: true},
			false,
		},
		{
			format{bg: standardColors[BG_RED], fg: standardColors[FG_WHITE], italic: true},
			format{bg: standardColors[BG_RED], italic: true},
			false,
		},
	}

	for i, c := range cases {
		if got := c.f1.equal(c.f2); got != c.want {
			t.Errorf("%d: Got %t, wanted %t when comparing %v == %v", i, got, c.want, c.f1, c.f2)
		}
	}
}

func TestFormatApplication(t *testing.T) {
	cases := []struct {
		initial format
		params  []int
		want    format
	}{
		{
			format{bg: standardColors[BG_BLUE], underline: UNDERLINE_DOUBLE, brightness: FONT_BOLD},
			[]int{},
			format{},
		},
		{
			format{bg: standardColors[BG_BLUE], underline: UNDERLINE_DOUBLE, brightness: FONT_BOLD},
			[]int{BG_BLACK, UNDERLINE_ON, STRIKEOUT_ON},
			format{bg: standardColors[BG_BLACK], brightness: FONT_BOLD, underline: UNDERLINE_SINGLE, strikeout: true},
		},
		{
			format{},
			[]int{FG_BRIGHT_RED},
			format{fg: standardColors[FG_RED], brightness: FONT_BOLD},
		},
		{
			format{},
			[]int{FG_BRIGHT_RED, BG_BLACK, UNDERLINE_ON, STRIKEOUT_ON},
			format{fg: standardColors[FG_RED], brightness: FONT_BOLD, bg: standardColors[BG_BLACK], underline: UNDERLINE_SINGLE, strikeout: true},
		},
		{
			format{bg: standardColors[BG_BLUE]},
			[]int{INTENSITY_BOLD, SET_FG, 2, 212, 219, 123, STRIKEOUT_ON, STRIKEOUT_OFF},
			format{fg: rgbColor{[]int{212, 219, 123}}, brightness: FONT_BOLD, bg: standardColors[BG_BLUE], strikeout: false},
		},
	}

	for i, c := range cases {
		if got := formatFromParams(c.initial, c.params); !c.want.equal(got) {
			t.Errorf("%d: Got %s, wanted %s after applying %v to %s", i, got.String(), c.want.String(), c.params, c.initial.String())
		}
	}
}
