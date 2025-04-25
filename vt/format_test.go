package vt

import (
	"fmt"
	"slices"
	"testing"
)

func TestFormatEquality(t *testing.T) {
	cases := []struct {
		f1, f2 format
		want   bool
	}{
		{
			format{bg: newColor(BG_RED), attrs: UNDERLINE},
			format{bg: newColor(BG_RED), attrs: UNDERLINE},
			true,
		},
		{
			format{bg: newColor(BG_GREEN), attrs: UNDERLINE},
			format{bg: newColor(BG_RED), attrs: UNDERLINE},
			false,
		},
		{
			format{bg: newColor(BG_RED), attrs: BOLD},
			format{bg: newColor(BG_RED), attrs: BOLD},
			true,
		},
		{
			format{bg: newColor(BG_RED), fg: newColor(FG_YELLOW), attrs: BOLD},
			format{bg: newColor(BG_RED), attrs: BOLD},
			false,
		},
		{
			format{fg: newColor(FG_RED), bg: newColor(BG_YELLOW), attrs: STRIKEOUT},
			format{fg: newColor(FG_RED), attrs: STRIKEOUT},
			false,
		},
		{
			format{}, defFmt, true,
		},
		{
			defFmt, defFmt, true,
		},
	}

	for i, c := range cases {
		if got := c.f1.equal(c.f2); got != c.want {
			t.Errorf("%d: Got %t, wanted %t when comparing\n\t%v ==\n\t%v", i, got, c.want, c.f1.String(), c.f2.String())
		}
	}
}

func TestFormatApplication(t *testing.T) {
	cases := []struct {
		initial format
		params  *parameters
		want    format
	}{
		{
			format{bg: newColor(BG_BLUE), attrs: BOLD | UNDERLINE},
			paramsFromInts([]int{}),
			format{},
		},
		{
			format{bg: newColor(BG_BLUE), attrs: BOLD | UNDERLINE},
			paramsFromInts([]int{BG_BLACK, UNDERLINE_ON, STRIKEOUT_ON}),
			format{bg: newColor(BG_BLACK), attrs: BOLD | UNDERLINE | STRIKEOUT},
		},
		{
			format{},
			paramsFromInts([]int{FG_BRIGHT_RED}),
			format{fg: newColor(FG_BRIGHT_RED)},
		},
		{
			format{},
			paramsFromInts([]int{FG_BRIGHT_RED, BG_BLACK, UNDERLINE_ON, STRIKEOUT_ON}),
			format{fg: newColor(FG_BRIGHT_RED), bg: newColor(BG_BLACK), attrs: UNDERLINE | STRIKEOUT},
		},
		{
			format{bg: newColor(BG_BLUE)},
			paramsFromInts([]int{INTENSITY_BOLD, SET_FG, 2, 212, 219, 123, STRIKEOUT_ON, STRIKEOUT_OFF}),
			format{fg: newRGBColor([]int{212, 219, 123}), bg: newColor(BG_BLUE), attrs: BOLD},
		},
		{
			format{bg: newColor(BG_BLUE), attrs: BOLD | UNDERLINE},
			paramsFromInts([]int{INTENSITY_FAINT}),
			format{bg: newColor(BG_BLUE), attrs: BOLD_FAINT | UNDERLINE},
		},
		{
			format{bg: newColor(BG_BLUE), attrs: FAINT | UNDERLINE},
			paramsFromInts([]int{INTENSITY_BOLD}),
			format{bg: newColor(BG_BLUE), attrs: BOLD_FAINT | UNDERLINE},
		},
		{
			format{bg: newColor(BG_BLUE), attrs: BOLD_FAINT | UNDERLINE},
			paramsFromInts([]int{INTENSITY_NORMAL}),
			format{bg: newColor(BG_BLUE), attrs: UNDERLINE},
		},
		{
			format{bg: newColor(BG_BLUE), attrs: BOLD | UNDERLINE},
			paramsFromInts([]int{INTENSITY_NORMAL}),
			format{bg: newColor(BG_BLUE), attrs: UNDERLINE},
		},
		{
			format{bg: newColor(BG_BLUE), attrs: FAINT | UNDERLINE},
			paramsFromInts([]int{INTENSITY_NORMAL}),
			format{bg: newColor(BG_BLUE), attrs: UNDERLINE},
		},
	}

	for i, c := range cases {
		if got := formatFromParams(c.initial, c.params); !c.want.equal(got) {
			t.Errorf("%d: Got\n\t%s, wanted\n\t%s after applying %v to %s", i, got.String(), c.want.String(), c.params, c.initial.String())
		}
	}
}

func TestFormatDiff(t *testing.T) {
	cases := []struct {
		srcF, destF format
		want        []byte
	}{
		{
			format{fg: newColor(FG_WHITE), attrs: UNDERLINE},
			format{fg: newColor(FG_WHITE), attrs: UNDERLINE},
			[]byte{},
		},
		{
			// Any diff against "dest == default" should just reset the pen
			format{fg: newRGBColor([]int{10, 20, 30}), bg: newRGBColor([]int{30, 20, 10})},
			defFmt,
			[]byte(FMT_RESET),
		},
		{
			format{fg: newRGBColor([]int{10, 20, 30})},
			format{bg: newColor(BG_YELLOW)},
			[]byte(fmt.Sprintf("%c%c%d%c%c%c%d%c", ESC, CSI, FG_DEF, CSI_SGR, ESC, CSI, BG_YELLOW, CSI_SGR)),
		},
		{
			defFmt,
			format{fg: newColor(FG_WHITE), attrs: BOLD},
			[]byte(fmt.Sprintf("%c%c%dm%c%c%d%c", ESC, CSI, FG_WHITE, ESC, CSI, INTENSITY_BOLD, CSI_SGR)),
		},
		{
			format{fg: newColor(FG_WHITE), attrs: STRIKEOUT},
			format{bg: newAnsiColor(243), attrs: REVERSED},
			[]byte(fmt.Sprintf("%c%c%d%c%c%c%d;5;%d%c%c%c%d;%d%c", ESC, CSI, FG_DEF, CSI_SGR, ESC, CSI, SET_BG, 243, CSI_SGR, ESC, CSI, REVERSED_ON, STRIKEOUT_OFF, CSI_SGR)),
		},
		{
			format{fg: newRGBColor([]int{10, 20, 30}), bg: newRGBColor([]int{30, 20, 10})},
			format{fg: newRGBColor([]int{30, 20, 10}), bg: newRGBColor([]int{10, 20, 30})},
			[]byte(fmt.Sprintf("%c%c%d;2;%d;%d;%d%c%c%c%d;2;%d;%d;%d%c", ESC, CSI, SET_FG, 30, 20, 10, CSI_SGR, ESC, CSI, SET_BG, 10, 20, 30, CSI_SGR)),
		},
		{
			format{fg: newRGBColor([]int{10, 20, 30}), bg: newRGBColor([]int{30, 20, 10})},
			format{fg: newColor(FG_BLUE), bg: newAnsiColor(124)},
			[]byte(fmt.Sprintf("%c%c%d%c%c%c%d;5;%d%c", ESC, CSI, FG_BLUE, CSI_SGR, ESC, CSI, SET_BG, 124, CSI_SGR)),
		},

		{
			format{fg: newRGBColor([]int{10, 20, 30}), bg: newRGBColor([]int{30, 20, 10})},
			format{fg: newRGBColor([]int{10, 20, 30}), bg: newAnsiColor(124)},
			[]byte(fmt.Sprintf("%c%c%d;5;%d%c", ESC, CSI, SET_BG, 124, CSI_SGR)),
		},
		{
			defFmt,
			format{attrs: UNDERLINE},
			[]byte(fmt.Sprintf("%c%c%d%c", ESC, CSI, UNDERLINE_ON, CSI_SGR)),
		},
		{
			defFmt,
			format{attrs: BOLD_FAINT},
			[]byte(fmt.Sprintf("%c%c%d;%d%c", ESC, CSI, INTENSITY_BOLD, INTENSITY_FAINT, CSI_SGR)),
		},
		{
			format{attrs: BOLD_FAINT},
			format{attrs: BOLD},
			[]byte(fmt.Sprintf("%c%c%d;%d%c", ESC, CSI, INTENSITY_NORMAL, INTENSITY_BOLD, CSI_SGR)),
		},
		{
			format{attrs: BOLD_FAINT},
			format{attrs: FAINT},
			[]byte(fmt.Sprintf("%c%c%d;%d%c", ESC, CSI, INTENSITY_NORMAL, INTENSITY_FAINT, CSI_SGR)),
		},
	}

	for i, c := range cases {
		if got := c.srcF.diff(c.destF); !slices.Equal(got, c.want) {
			t.Errorf("%d: Got\n\t%q, wanted\n\t%q\n\t%v\n\t%v", i, string(got), string(c.want), c.srcF, c.destF)
		}
	}
}
