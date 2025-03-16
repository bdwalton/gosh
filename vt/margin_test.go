package vt

import (
	"testing"
)

func TestMarginEqual(t *testing.T) {
	cases := []struct {
		m1, m2 margin
		want   bool
	}{
		{margin{}, margin{}, true},
		{newMargin(0, 1), margin{}, false}, // one set specifically
		{margin{}, newMargin(0, 1), false}, // one set specifically
		{newMargin(1, 2), newMargin(1, 2), true},
		{newMargin(1, 2), newMargin(1, 3), false},
	}

	for i, c := range cases {
		if got := c.m1.equal(c.m2); got != c.want {
			t.Errorf("%d: Got %t, wanted %t when comparing m1:%s and m2:%s", i, got, c.want, c.m1, c.m2)
		}
	}
}

func TestMarginContains(t *testing.T) {
	cases := []struct {
		m    margin
		v    int
		want bool
	}{
		{margin{}, 10, true}, // isSet == false, so everything contained
		{newMargin(5, 7), 10, false},
		{newMargin(5, 7), 6, true},
	}

	for i, c := range cases {
		if got := c.m.contains(c.v); got != c.want {
			t.Errorf("%d: Got %t, wanted %t when calling %q.contains(%d)", i, got, c.want, c.m, c.v)
		}
	}
}
