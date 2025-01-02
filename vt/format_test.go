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
