// Copyright (c) 2025, Ben Walton
// All rights reserved.
package vt

import (
	"testing"
)

func TestOSC8Equal(t *testing.T) {
	cases := []struct {
		o1, o2 *osc8
		want   bool
	}{
		{defOSC8, defOSC8, true},
		{defOSC8, newHyperlink("8;;http://foo"), false},
		{newHyperlink("8;;http://foo"), newHyperlink("8;;http://foo"), true},
		{newHyperlink("8;;http://foo"), newHyperlink("8;;http://bar"), false},
		{newHyperlink("8;;http://foo"), defOSC8, false},
		{newHyperlink("8;;"), newHyperlink("8;;http://foo"), false},
	}

	for i, c := range cases {
		if got := c.o1.equal(c.o2); got != c.want {
			t.Errorf("%d: Got %t, wanted %t from %v == %v", i, got, c.want, c.o1, c.o2)
		}
	}
}

func TestOSC8AnsiString(t *testing.T) {
	cases := []struct {
		o    *osc8
		want string
	}{
		{defOSC8, "\x1b]8;;\x1b\\"},
		{newHyperlink("8;;http://foo"), "\x1b]8;;http://foo\x1b\\"},
		{newHyperlink("8;;http://example.com"), "\x1b]8;;http://example.com\x1b\\"},
		{newHyperlink("8;id=0;http://example.com"), "\x1b]8;id=0;http://example.com\x1b\\"},
		{newHyperlink(cancelHyperlink), "\x1b]8;;\x1b\\"},
	}

	for i, c := range cases {
		if got := c.o.ansiString(); got != c.want {
			t.Errorf("%d: Got %q, wanted %q from %v", i, got, c.want, c.o)
		}
	}
}
