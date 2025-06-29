// Copyright (c) 2025, Ben Walton
// All rights reserved.
package vt

import (
	"fmt"
	"log/slog"
	"strings"
)

const (
	MAX_EXPECTED_INTERMEDIATE = 10
	MAX_EXPECTED_PARAMS       = 16
)

type transition uint8

func newTransition(a pAction, s pState) transition {
	return transition(uint8(a<<4) | uint8(s))
}

func (t transition) state() pState {
	return pState(t & 0x0F)
}

func (t transition) action() pAction {
	return pAction(t >> 4)
}

type parameters struct {
	num   int
	items []int
}

func newParams() *parameters {
	return &parameters{items: make([]int, 0, MAX_EXPECTED_PARAMS)}
}

func (p *parameters) copy() *parameters {
	ni := make([]int, len(p.items))
	copy(ni, p.items)
	return &parameters{num: p.num, items: ni}
}

func (p *parameters) String() string {
	var sb strings.Builder
	for i := 0; i < p.numItems(); i++ {
		if i > 0 {
			sb.WriteByte(';')
		}
		sb.WriteString(fmt.Sprintf("%d", p.item(i, 0)))
	}
	return sb.String()
}

func (p *parameters) addItem(item int) {
	p.items = append(p.items, item)
	p.num += 1
}

func (p *parameters) alterItem(val int) {
	p.items[p.num-1] = val
}

func (p *parameters) reset() {
	p.items = p.items[:0]
	p.num = 0
}

func (p *parameters) numItems() int {
	return p.num
}

func (p *parameters) item(item, def int) int {
	if p.num == 0 || p.num <= item {
		return def
	}
	return p.items[item]
}

func (p *parameters) itemDefaultOneIfZero(item, def int) int {
	n := p.item(item, def)
	if n == 0 {
		return 1
	}
	return n
}

func (p *parameters) lastItem() int {
	if p.num == 0 {
		return 0
	}
	return p.items[p.num-1]
}

func (p *parameters) consumeItem() (int, bool) {
	if p.num < 1 {
		return 0, false
	}
	n := p.items[0]
	p.num -= 1
	p.items = p.items[1:]
	return n, true
}

type parser struct {
	state        pState
	intermediate []rune
	params       *parameters
}

func newParser() *parser {
	return &parser{
		state:        STATE_GROUND,
		params:       newParams(),
		intermediate: make([]rune, 0, MAX_EXPECTED_INTERMEDIATE),
	}
}

func (p *parser) copy() *parser {
	in := make([]rune, len(p.intermediate))
	copy(in, p.intermediate)
	return &parser{
		state:        p.state,
		params:       p.params.copy(),
		intermediate: in,
	}
}

func (p *parser) parse(r rune) []*action {
	trans, ok := STATE_TABLE[p.state][r]
	if !ok {
		switch p.state {
		case STATE_GROUND:
			return []*action{p.action(ACTION_PRINT, r)}
		case STATE_OSC_STRING:
			return []*action{p.action(ACTION_OSC_PUT, r)}
		default:
			slog.Debug("unhandled state for failed rune lookup", "state", STATE_NAME[p.state], "r", r)
		}

	}

	return p.stateChange(trans, r)
}

// action is what we'll return to our clients to udpate and manage
// state as we parse the input.
type action struct {
	act    pAction
	params *parameters
	data   []rune
	cmd    rune
}

func (p *parser) action(act pAction, r rune) *action {
	switch act {
	case ACTION_PRINT, ACTION_EXECUTE, ACTION_HOOK, ACTION_PUT, ACTION_OSC_START, ACTION_OSC_PUT, ACTION_OSC_END, ACTION_UNHOOK, ACTION_CSI_DISPATCH, ACTION_ESC_DISPATCH:
		return &action{act, p.params, p.intermediate, r}
	case ACTION_IGNORE:
		// Do nothing
	case ACTION_COLLECT:
		p.intermediate = append(p.intermediate, r)
	case ACTION_PARAM:
		switch r {
		// : is used for some CSI sequences like:
		// CSI 32 : 2 : Pi : Pr : Pg : Pb m to set true colors
		case ';', ':':
			if p.params.numItems() == 0 {
				p.params.addItem(0)
			}
			p.params.addItem(0)
		default:
			switch p.params.numItems() {
			case 0:
				p.params.addItem(int(r - '0'))
			default:
				p.params.alterItem(p.params.lastItem()*10 + int(r-'0'))
			}
		}
	case ACTION_CLEAR:
		p.intermediate = p.intermediate[:0]
		p.params.reset()
	default:
		return &action{ACTION_ERROR, nil, nil, 0}
	}

	return nil
}

func (p *parser) stateChange(t transition, r rune) []*action {
	newState := t.state()
	act := t.action()

	ret := []*action{}

	if newState != STATE_NONE {
		exit := EXIT_ACTIONS[p.state]
		enter := ENTRY_ACTIONS[newState]

		if exit != ACTION_NOP {
			if a := p.action(exit, r); a != nil {
				ret = append(ret, a)
			}
		}

		if act != ACTION_NOP {
			if a := p.action(act, r); a != nil {
				ret = append(ret, a)
			}
		}

		if enter != ACTION_NOP {
			if a := p.action(enter, r); a != nil {
				ret = append(ret, a)
			}
		}

		p.state = newState
	} else {
		if a := p.action(act, r); a != nil {
			ret = append(ret, a)
		}
	}

	return ret
}
