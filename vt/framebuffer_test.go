package vt

import (
	"testing"
)

func TestResetRows(t *testing.T) {
	cases := []struct {
		fb *framebuffer
	}{
		{newFramebuffer(2, 2)},
	}

	for i, c := range cases {
		c.fb.resetRows(0, 0, defFmt)
		if c.fb == nil {
			t.Errorf("%d: resetRows() failed", i)
		}
	}
}
