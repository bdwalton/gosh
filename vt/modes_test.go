package vt

import (
	"testing"
)

func TestModeAnsiString(t *testing.T) {
	cases := []struct {
		m    *mode
		want string
	}{
		{&mode{state: CSI_MODE_SET, code: REV_VIDEO}, "\x1b[?5h"},
		{&mode{state: CSI_MODE_RESET, code: DECCOLM}, "\x1b[?3l"},
		{&mode{state: CSI_MODE_SET, public: true, code: IRM}, "\x1b[4h"},
		{&mode{state: CSI_MODE_RESET, public: true, code: LNM}, "\x1b[20l"},
	}

	for i, c := range cases {
		if got := c.m.ansiString(); got != c.want {
			t.Errorf("%d: Got %q, wanted %q", i, got, c.want)
		}
	}
}
