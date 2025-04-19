package vt

import (
	"fmt"
	"testing"
)

func TestCursorMoveToAnsi(t *testing.T) {
	cases := []struct {
		cur  cursor
		want string
	}{
		{cursor{0, 0}, fmt.Sprintf("%c%c%c", ESC, CSI, CSI_CUP)},
		{cursor{1, 1}, fmt.Sprintf("%c%c2;2%c", ESC, CSI, CSI_CUP)},
		{cursor{30, 0}, fmt.Sprintf("%c%c31%c", ESC, CSI, CSI_CUP)},
		{cursor{0, 15}, fmt.Sprintf("%c%c;16%c", ESC, CSI, CSI_CUP)},
	}

	for i, c := range cases {
		if got := c.cur.getMoveToAnsi(); got != c.want {
			t.Errorf("%d: Got %q, wanted %q", i, got, c.want)
		}
	}
}
