package vt

import (
	"fmt"
	"slices"
	"testing"
)

var dFG = standardColors[FG_DEF]
var dBG = standardColors[BG_DEF]

func TestGetFG(t *testing.T) {
	cases := []struct {
		f    *format
		want *color
	}{
		{&format{}, dFG},
		{&format{fg: dFG}, dFG},
		{&format{fg: standardColors[FG_BLUE]}, standardColors[FG_BLUE]},
	}

	for i, c := range cases {
		if got := c.f.getFG(); !got.equal(c.want) {
			t.Errorf("%d: Got %v, wanted %v", i, got, c.want)
		}
	}
}

func TestGetBG(t *testing.T) {
	cases := []struct {
		f    *format
		want *color
	}{
		{&format{}, dBG},
		{&format{bg: dBG}, dBG},
		{&format{bg: standardColors[BG_BLUE]}, standardColors[BG_BLUE]},
	}

	for i, c := range cases {
		if got := c.f.getBG(); !got.equal(c.want) {
			t.Errorf("%d: Got %v, wanted %v", i, got, c.want)
		}
	}
}

func TestFormatEquality(t *testing.T) {
	cases := []struct {
		f1, f2 format
		want   bool
	}{
		{
			format{bg: standardColors[BG_RED], underline: true},
			format{bg: standardColors[BG_RED], underline: true},
			true,
		},
		{
			format{bg: standardColors[BG_GREEN], underline: true},
			format{bg: standardColors[BG_RED], underline: true},
			false,
		},
		{
			format{bg: standardColors[BG_RED], fg: dFG, bold: true},
			format{bg: standardColors[BG_RED], bold: true},
			true,
		},
		{
			format{bg: standardColors[BG_RED], fg: standardColors[FG_YELLOW], bold: true},
			format{bg: standardColors[BG_RED], bold: true},
			false,
		},
		{
			format{fg: standardColors[FG_RED], bg: standardColors[BG_YELLOW], strikeout: true},
			format{fg: standardColors[FG_RED], strikeout: true},
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
			format{bg: standardColors[BG_BLUE], underline: true, bold: true},
			paramsFromInts([]int{}),
			format{},
		},
		{
			format{bg: standardColors[BG_BLUE], underline: true, bold: true},
			paramsFromInts([]int{BG_BLACK, UNDERLINE_ON, STRIKEOUT_ON}),
			format{bg: standardColors[BG_BLACK], bold: true, underline: true, strikeout: true},
		},
		{
			format{},
			paramsFromInts([]int{FG_BRIGHT_RED}),
			format{fg: standardColors[FG_BRIGHT_RED]},
		},
		{
			format{},
			paramsFromInts([]int{FG_BRIGHT_RED, BG_BLACK, UNDERLINE_ON, STRIKEOUT_ON}),
			format{fg: standardColors[FG_BRIGHT_RED], bg: standardColors[BG_BLACK], underline: true, strikeout: true},
		},
		{
			format{bg: standardColors[BG_BLUE]},
			paramsFromInts([]int{INTENSITY_BOLD, SET_FG, 2, 212, 219, 123, STRIKEOUT_ON, STRIKEOUT_OFF}),
			format{fg: newRGBColor([]int{212, 219, 123}), bold: true, bg: standardColors[BG_BLUE], strikeout: false},
		},
	}

	for i, c := range cases {
		if got := formatFromParams(c.initial, c.params); !c.want.equal(got) {
			t.Errorf("%d: Got\n\t%s, wanted\n\t%s after applying %v to %s", i, got.String(), c.want.String(), c.params, c.initial.String())
		}
	}
}

func TestDiff(t *testing.T) {
	cases := []struct {
		srcF, destF format
		want        []byte
	}{
		{
			format{fg: standardColors[FG_WHITE], underline: true},
			format{fg: standardColors[FG_WHITE], underline: true},
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
			format{bg: standardColors[BG_YELLOW]},
			[]byte(fmt.Sprintf("%c%c%d%c%c%c%d%c", ESC, ESC_CSI, FG_DEF, CSI_SGR, ESC, ESC_CSI, BG_YELLOW, CSI_SGR)),
		},
		{
			defFmt,
			format{fg: standardColors[FG_WHITE], bold: true},
			[]byte(fmt.Sprintf("%c%c%dm%c%c%d%c", ESC, ESC_CSI, FG_WHITE, ESC, ESC_CSI, INTENSITY_BOLD, CSI_SGR)),
		},
		{
			format{fg: standardColors[FG_WHITE], strikeout: true},
			format{bg: newAnsiColor(243), reversed: true},
			[]byte(fmt.Sprintf("%c%c%d%c%c%c%d;5;%d%c%c%c%d;%d%c", ESC, ESC_CSI, FG_DEF, CSI_SGR, ESC, ESC_CSI, SET_BG, 243, CSI_SGR, ESC, ESC_CSI, REVERSED_ON, STRIKEOUT_OFF, CSI_SGR)),
		},
		{
			format{fg: newRGBColor([]int{10, 20, 30}), bg: newRGBColor([]int{30, 20, 10})},
			format{fg: newRGBColor([]int{30, 20, 10}), bg: newRGBColor([]int{10, 20, 30})},
			[]byte(fmt.Sprintf("%c%c%d;2;%d;%d;%d%c%c%c%d;2;%d;%d;%d%c", ESC, ESC_CSI, SET_FG, 30, 20, 10, CSI_SGR, ESC, ESC_CSI, SET_BG, 10, 20, 30, CSI_SGR)),
		},
		{
			format{fg: newRGBColor([]int{10, 20, 30}), bg: newRGBColor([]int{30, 20, 10})},
			format{fg: standardColors[FG_BLUE], bg: newAnsiColor(124)},
			[]byte(fmt.Sprintf("%c%c%d%c%c%c%d;5;%d%c", ESC, ESC_CSI, FG_BLUE, CSI_SGR, ESC, ESC_CSI, SET_BG, 124, CSI_SGR)),
		},

		{
			format{fg: newRGBColor([]int{10, 20, 30}), bg: newRGBColor([]int{30, 20, 10})},
			format{fg: newRGBColor([]int{10, 20, 30}), bg: newAnsiColor(124)},
			[]byte(fmt.Sprintf("%c%c%d;5;%d%c", ESC, ESC_CSI, SET_BG, 124, CSI_SGR)),
		},
		{
			defFmt,
			format{underline: true},
			[]byte(fmt.Sprintf("%c%c%d%c", ESC, ESC_CSI, UNDERLINE_ON, CSI_SGR)),
		},
	}

	for i, c := range cases {
		if got := c.srcF.diff(c.destF); !slices.Equal(got, c.want) {
			t.Errorf("%d: Got\n\t%q, wanted\n\t%q\n\t%v\n\t%v", i, string(got), string(c.want), c.srcF, c.destF)
		}
	}
}
