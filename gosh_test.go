package main

import (
	"testing"
)

func TestHostFromDest(t *testing.T) {
	cases := []struct {
		dest, want string
	}{
		{"hostname", "hostname"},
		{"username@hostname", "hostname"},
		{"username@hostname@something", "hostname@something"},
		{"username@hostname@something@whatwereyouthinking", "hostname@something@whatwereyouthinking"},
	}

	for i, c := range cases {
		if got := hostFromDest(c.dest); got != c.want {
			t.Errorf("%d: Got %q, wanted %q; from %q", i, got, c.want, c.dest)
		}
	}
}
