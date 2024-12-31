package vt

import (
	"slices"
	"testing"
)

type dummyD struct {
	actions []pAction
}

func newDummy() *dummyD {
	return &dummyD{actions: make([]pAction, 0)}
}

func (d *dummyD) handle(act pAction, params []int, data []rune, lastbyte byte) {
}

func (d *dummyD) print(r rune) {
}

func TestFirstParamEmpty(t *testing.T) {
	cases := []struct {
		input      []byte
		wantParams []int
	}{
		{[]byte{ESC, ESC_CSI, ';'}, []int{0, 0}},
		{[]byte{C1_CSI, ';'}, []int{0, 0}},
		{[]byte{C1_CSI, ';', ';'}, []int{0, 0, 0}},
		{[]byte{C1_CSI, ';', '0', ';'}, []int{0, 0, 0}},
		{[]byte{C1_CSI, ';', '5', '0', ';'}, []int{0, 50, 0}},
		{[]byte{C1_CSI, '1', '0', ';', ';'}, []int{10, 0, 0}},
		{[]byte{C1_CSI, '1', '0', ';', ';'}, []int{10, 0, 0}},
		{[]byte{C1_CSI, '1', '0', ';', ';', '5'}, []int{10, 0, 5}},
	}

	for i, c := range cases {
		p := newParser(&dummyD{})
		for _, b := range c.input {
			p.ParseByte(b)
		}
		if len(p.params) != len(c.wantParams) || !slices.Equal(p.params, c.wantParams) {
			t.Errorf("%d: Got %v, want %v", i, p.params, c.wantParams)
		}
	}
}
