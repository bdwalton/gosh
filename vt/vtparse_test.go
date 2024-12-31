package vt

import (
	"slices"
	"testing"
)

type dummyD struct {
	actions      []pAction
	params       []int
	intermediate []rune
	lastbyte     byte
}

func newDummy() *dummyD {
	return &dummyD{actions: make([]pAction, 0)}
}

func (d *dummyD) handle(act pAction, params []int, intermediate []rune, lastbyte byte) {
	d.actions = append(d.actions, act)
	d.params = params
	d.intermediate = intermediate
	d.lastbyte = lastbyte
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
		p := newParser(newDummy())
		for _, b := range c.input {
			p.ParseByte(b)
		}
		if len(p.params) != len(c.wantParams) || !slices.Equal(p.params, c.wantParams) {
			t.Errorf("%d: Got %v, want %v", i, p.params, c.wantParams)
		}
	}
}

func TestCSIParsing(t *testing.T) {
	cases := []struct {
		input            []byte
		wantActions      []pAction
		wantParams       []int
		wantIntermediate []rune
		wantLast         byte
	}{
		{
			[]byte{C1_CSI, ';', 'm'},
			[]pAction{VTPARSE_ACTION_CSI_DISPATCH},
			[]int{0, 0},
			[]rune{},
			CSI_SGR,
		},
		{
			[]byte{C1_CSI, 'm'},
			[]pAction{VTPARSE_ACTION_CSI_DISPATCH},
			[]int{},
			[]rune{},
			CSI_SGR,
		},
		{
			[]byte{C1_CSI, '1', '0', 'A'},
			[]pAction{VTPARSE_ACTION_CSI_DISPATCH},
			[]int{10},
			[]rune{},
			CSI_CUU,
		},
		{
			[]byte{C1_CSI, '1', '0', ';', '3', 'H'},
			[]pAction{VTPARSE_ACTION_CSI_DISPATCH},
			[]int{10, 3},
			[]rune{},
			CSI_CUP,
		},
		{
			[]byte{C1_CSI, '6', 'n'},
			[]pAction{VTPARSE_ACTION_CSI_DISPATCH},
			[]int{6},
			[]rune{},
			CSI_DSR,
		},
		{
			[]byte{C1_CSI, '?', '2', '0', '0', '4', 'l'},
			[]pAction{VTPARSE_ACTION_CSI_DISPATCH},
			[]int{2004},
			[]rune{'?'},
			CSI_PRIV_DISABLE,
		},
	}

	for i, c := range cases {
		d := newDummy()
		p := newParser(d)
		for _, b := range c.input {
			p.ParseByte(b)
		}

		if !slices.Equal(d.actions, c.wantActions) {
			t.Errorf("%d: Invalid actions called. Got %v, want %v", i, d.actions, c.wantActions)
		}
		if !slices.Equal(d.params, c.wantParams) {
			t.Errorf("%d: Invalid params. Got %v, want %v", i, d.params, c.wantParams)
		}
		if !slices.Equal(d.intermediate, c.wantIntermediate) {
			t.Errorf("%d: Invalid params. Got %v, want %v", i, d.intermediate, c.wantIntermediate)
		}
		if d.lastbyte != c.wantLast {
			t.Errorf("%d: Invalid last byte. Got %02x, want %02x", i, d.lastbyte, c.wantLast)
		}
	}
}
