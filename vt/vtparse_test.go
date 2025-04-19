package vt

import (
	"slices"
	"testing"
)

type dummyD struct {
	actions      []pAction
	params       *parameters
	intermediate []rune
	last         rune
	oscTemp      []rune
	oscString    []rune
}

func newDummy() *dummyD {
	return &dummyD{
		actions:   make([]pAction, 0),
		oscTemp:   make([]rune, 0),
		oscString: make([]rune, 0),
	}
}

func (d *dummyD) getActions() []string {
	l := len(d.actions)
	acts := make([]string, l, l)
	for i := range d.actions {
		acts[i] = ACTION_NAMES[d.actions[i]]
	}
	return acts
}

func (d *dummyD) handle(act pAction, params *parameters, intermediate []rune, last rune) {
	d.actions = append(d.actions, act)
	switch act {
	case VTPARSE_ACTION_OSC_PUT:
		d.oscTemp = append(d.oscTemp, last)
	case VTPARSE_ACTION_OSC_END:
		d.oscString = d.oscTemp
	default:
		d.params = params
		d.intermediate = intermediate
		d.last = last
	}
}

func (d *dummyD) print(r rune) {
}

func TestFirstParamEmpty(t *testing.T) {
	cases := []struct {
		input      []byte
		wantParams *parameters
	}{
		{[]byte{ESC, CSI, ';'}, paramsFromInts([]int{0, 0})},
		{[]byte{C1_CSI, ';'}, paramsFromInts([]int{0, 0})},
		{[]byte{C1_CSI, ';', ';'}, paramsFromInts([]int{0, 0, 0})},
		{[]byte{C1_CSI, ';', '0', ';'}, paramsFromInts([]int{0, 0, 0})},
		{[]byte{C1_CSI, ';', '5', '0', ';'}, paramsFromInts([]int{0, 50, 0})},
		{[]byte{C1_CSI, '1', '0', ';', ';'}, paramsFromInts([]int{10, 0, 0})},
		{[]byte{C1_CSI, '1', '0', ';', ';'}, paramsFromInts([]int{10, 0, 0})},
		{[]byte{C1_CSI, '1', '0', ';', ';', '5'}, paramsFromInts([]int{10, 0, 5})},
	}

	for i, c := range cases {
		p := newParser()
		for _, b := range c.input {
			p.parse(rune(b))
		}
		if p.params.numItems() != c.wantParams.numItems() || !slices.Equal(p.params.items, c.wantParams.items) {
			t.Errorf("%d: Got %v, want %v", i, p.params, c.wantParams)
		}
	}
}

func TestCSIParsing(t *testing.T) {
	cases := []struct {
		input            []rune
		wantActions      []pAction
		wantParams       *parameters
		wantIntermediate []rune
		wantLast         rune
	}{
		{
			[]rune{C1_CSI, ';', 'm'},
			[]pAction{VTPARSE_ACTION_CSI_DISPATCH},
			paramsFromInts([]int{0, 0}),
			[]rune{},
			CSI_SGR,
		},
		{
			[]rune{C1_CSI, 'm'},
			[]pAction{VTPARSE_ACTION_CSI_DISPATCH},
			paramsFromInts([]int{}),
			[]rune{},
			CSI_SGR,
		},
		{
			[]rune{C1_CSI, '1', '0', 'A'},
			[]pAction{VTPARSE_ACTION_CSI_DISPATCH},
			paramsFromInts([]int{10}),
			[]rune{},
			CSI_CUU,
		},
		{
			[]rune{C1_CSI, '1', '0', ';', '3', 'H'},
			[]pAction{VTPARSE_ACTION_CSI_DISPATCH},
			paramsFromInts([]int{10, 3}),
			[]rune{},
			CSI_CUP,
		},
		{
			[]rune{C1_CSI, '6', 'n'},
			[]pAction{VTPARSE_ACTION_CSI_DISPATCH},
			paramsFromInts([]int{6}),
			[]rune{},
			CSI_DSR,
		},
		{
			[]rune{C1_CSI, '?', '2', '0', '0', '4', 'l'},
			[]pAction{VTPARSE_ACTION_CSI_DISPATCH},
			paramsFromInts([]int{2004}),
			[]rune{'?'},
			CSI_MODE_RESET,
		},
	}

	for i, c := range cases {
		p := newParser()
		d := newDummy()
		for _, r := range c.input {
			for _, a := range p.parse(r) {
				d.handle(a.act, a.params, a.data, a.r)
			}
		}

		if !slices.Equal(d.actions, c.wantActions) {
			t.Errorf("%d: Invalid actions called. Got %v, want %v", i, d.actions, c.wantActions)
		}
		if !slices.Equal(d.params.items, c.wantParams.items) {
			t.Errorf("%d: Invalid params. Got %v, want %v", i, p.params, c.wantParams)
		}
		if !slices.Equal(d.intermediate, c.wantIntermediate) {
			t.Errorf("%d: Invalid params. Got %v, want %v", i, p.intermediate, c.wantIntermediate)
		}
		if d.last != c.wantLast {
			t.Errorf("%d: Invalid last byte. Got %02x, want %02x", i, d.last, c.wantLast)
		}
	}
}

func TestOSCString(t *testing.T) {
	cases := []struct {
		input   []rune
		wantOSC string
	}{
		{[]rune{C1_OSC, '0', ';', 'i', 'c', 'o', 'n', 't', 'i', 't', 'l', 'e', BEL}, "0;icontitle"},
		{[]rune{C1_OSC, '1', ';', 'i', 'c', 'o', 'n', C1_ST}, "1;icon"},
		{[]rune{ESC, OSC, '2', ';', 't', 'i', 't', 'l', 'e', C1_ST}, "2;title"},
		{[]rune{ESC, OSC, '2', ';', 't', 'i', 't', 'l', 'e', ESC, ST}, "2;title"},
		{[]rune{ESC, OSC, '3', ';', 'F', 'o', 'O', BEL}, "3;FoO"},
	}

	for i, c := range cases {
		d := newDummy()
		p := newParser()
		for _, r := range c.input {
			for _, a := range p.parse(r) {
				d.handle(a.act, a.params, a.data, a.r)
			}
		}
		if string(d.oscString) != c.wantOSC {
			t.Errorf("%d: Got %q, want: %q (actions: %v)", i, string(d.oscString), c.wantOSC, d.getActions())
		}

	}
}
