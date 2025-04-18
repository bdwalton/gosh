package vt

import (
	"testing"
)

func TestModeGetAnsiString(t *testing.T) {
	cases := []struct {
		m    *mode
		want string
	}{
		{&mode{state: CSI_MODE_SET, private: true, code: 5}, "\x1b[?5h"},
		{&mode{state: CSI_MODE_RESET, private: true, code: 3}, "\x1b[?3l"},
		{&mode{state: CSI_MODE_SET, private: false, code: 4}, "\x1b[4h"},
		{&mode{state: CSI_MODE_RESET, private: false, code: 2}, "\x1b[2l"},
	}

	for i, c := range cases {
		if got := c.m.getAnsiString(); got != c.want {
			t.Errorf("%d: Got %q, wanted %q", i, got, c.want)
		}
	}
}
