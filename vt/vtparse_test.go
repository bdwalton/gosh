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
	case ACTION_OSC_PUT:
		d.oscTemp = append(d.oscTemp, last)
	case ACTION_OSC_END:
		d.oscString = d.oscTemp
	default:
		d.params = params
		d.intermediate = intermediate
		d.last = last
	}
}

func (d *dummyD) print(r rune) {
}

func TestColonParamSeparator(t *testing.T) {
	cases := []struct {
		input      []byte
		wantParams *parameters
	}{
		{[]byte{ESC, CSI, ':'}, paramsFromInts([]int{0, 0})},
		{[]byte{ESC, CSI, ':', ':'}, paramsFromInts([]int{0, 0, 0})},
		{[]byte{ESC, CSI, ':', '0', ':'}, paramsFromInts([]int{0, 0, 0})},
		{[]byte{ESC, CSI, ':', '5', '0', ':'}, paramsFromInts([]int{0, 50, 0})},
		{[]byte{ESC, CSI, '1', '0', ':', ':'}, paramsFromInts([]int{10, 0, 0})},
		{[]byte{ESC, CSI, '1', '0', ':', ':'}, paramsFromInts([]int{10, 0, 0})},
		{[]byte{ESC, CSI, '3', '8', ':', '2', ':', '5', ':', '1', ':', '6'}, paramsFromInts([]int{38, 2, 5, 1, 6})},
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

func TestFirstParamEmpty(t *testing.T) {
	cases := []struct {
		input      []byte
		wantParams *parameters
	}{
		{[]byte{ESC, CSI, ';'}, paramsFromInts([]int{0, 0})},
		{[]byte{ESC, CSI, ';'}, paramsFromInts([]int{0, 0})},
		{[]byte{ESC, CSI, ';', ';'}, paramsFromInts([]int{0, 0, 0})},
		{[]byte{ESC, CSI, ';', '0', ';'}, paramsFromInts([]int{0, 0, 0})},
		{[]byte{ESC, CSI, ';', '5', '0', ';'}, paramsFromInts([]int{0, 50, 0})},
		{[]byte{ESC, CSI, '1', '0', ';', ';'}, paramsFromInts([]int{10, 0, 0})},
		{[]byte{ESC, CSI, '1', '0', ';', ';'}, paramsFromInts([]int{10, 0, 0})},
		{[]byte{ESC, CSI, '1', '0', ';', ';', '5'}, paramsFromInts([]int{10, 0, 5})},
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
			[]rune{ESC, CSI, ';', 'm'},
			[]pAction{ACTION_CSI_DISPATCH},
			paramsFromInts([]int{0, 0}),
			[]rune{},
			CSI_SGR,
		},
		{
			[]rune{ESC, CSI, 'm'},
			[]pAction{ACTION_CSI_DISPATCH},
			paramsFromInts([]int{}),
			[]rune{},
			CSI_SGR,
		},
		{
			[]rune{ESC, CSI, '1', '0', 'A'},
			[]pAction{ACTION_CSI_DISPATCH},
			paramsFromInts([]int{10}),
			[]rune{},
			CSI_CUU,
		},
		{
			[]rune{ESC, CSI, '1', '0', ';', '3', 'H'},
			[]pAction{ACTION_CSI_DISPATCH},
			paramsFromInts([]int{10, 3}),
			[]rune{},
			CSI_CUP,
		},
		{
			[]rune{ESC, CSI, '6', 'n'},
			[]pAction{ACTION_CSI_DISPATCH},
			paramsFromInts([]int{6}),
			[]rune{},
			CSI_DSR,
		},
		{
			[]rune{ESC, CSI, '?', '2', '0', '0', '4', 'l'},
			[]pAction{ACTION_CSI_DISPATCH},
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
				d.handle(a.act, a.params, a.data, a.cmd)
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
		{[]rune{ESC, OSC, '0', ';', 'i', 'c', 'o', 'n', 't', 'i', 't', 'l', 'e', BEL}, "0;icontitle"},
		{[]rune{ESC, OSC, '1', ';', 'i', 'c', 'o', 'n', ESC, ST}, "1;icon"},
		{[]rune{ESC, OSC, '2', ';', 't', 'i', 't', 'l', 'e', ESC, ST}, "2;title"},
		{[]rune{ESC, OSC, '2', ';', 't', 'i', 't', 'l', 'e', ESC, ST}, "2;title"},
		{[]rune{ESC, OSC, '3', ';', 'F', 'o', 'O', BEL}, "3;FoO"},
	}

	for i, c := range cases {
		d := newDummy()
		p := newParser()
		for _, r := range c.input {
			for _, a := range p.parse(r) {
				d.handle(a.act, a.params, a.data, a.cmd)
			}
		}
		if string(d.oscString) != c.wantOSC {
			t.Errorf("%d: Got %q, want: %q (actions: %v)", i, string(d.oscString), c.wantOSC, d.getActions())
		}

	}
}
