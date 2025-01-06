package vt

import "log/slog"

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

func (p *parameters) getItem(item, def int) (int, bool) {
	if p.num == 0 || p.num <= item {
		return def, false
	}
	return p.items[item], true
}

func (p *parameters) lastItem() int {
	if p.num == 0 {
		return 0
	}
	return p.items[p.num-1]
}

func (p *parameters) consumeItem() (int, bool) {
	if p.num < 1 {
		slog.Debug("consumed from empty params")
		return 0, false
	}
	n := p.items[0]
	p.num -= 1
	p.items = p.items[1:]
	return n, true
}

type dispatcher interface {
	print(rune)
	// action, params, intermediate, last byte
	handle(pAction, *parameters, []rune, byte)
}

type parser struct {
	state        pState
	d            dispatcher
	intermediate []rune
	params       *parameters
}

func newParser(d dispatcher) *parser {
	return &parser{
		state:        VTPARSE_STATE_GROUND,
		d:            d,
		params:       newParams(),
		intermediate: make([]rune, 0, MAX_EXPECTED_INTERMEDIATE),
	}
}

func (p *parser) ParseRune(r rune) {
	switch p.state {
	case VTPARSE_STATE_GROUND:
		p.d.print(r)
	}
}

func (p *parser) ParseByte(b byte) {
	p.stateChange(STATE_TABLE[p.state][b], b)
}

func (p *parser) action(act pAction, b byte) {
	switch act {
	case VTPARSE_ACTION_PRINT:
		p.d.print(rune(b))
	case VTPARSE_ACTION_EXECUTE, VTPARSE_ACTION_HOOK, VTPARSE_ACTION_PUT, VTPARSE_ACTION_OSC_START, VTPARSE_ACTION_OSC_PUT, VTPARSE_ACTION_OSC_END, VTPARSE_ACTION_UNHOOK, VTPARSE_ACTION_CSI_DISPATCH, VTPARSE_ACTION_ESC_DISPATCH:
		p.d.handle(act, p.params, p.intermediate, b)
	case VTPARSE_ACTION_IGNORE:
		// Do nothing
	case VTPARSE_ACTION_COLLECT:
		p.intermediate = append(p.intermediate, rune(b))
	case VTPARSE_ACTION_PARAM:
		// State table only covers ; for param separator, but
		// : should be allowed.
		// TODO: Add : support later when we get to vttest level.
		if b == ';' {
			if p.params.numItems() == 0 {
				p.params.addItem(0)
			}
			p.params.addItem(0)
		} else {
			switch p.params.numItems() {
			case 0:
				p.params.addItem(int(b - '0'))
			default:
				p.params.alterItem(p.params.lastItem()*10 + int(b-'0'))
			}
		}
	case VTPARSE_ACTION_CLEAR:
		p.intermediate = p.intermediate[:0]
		p.params.reset()
	default:
		p.d.handle(VTPARSE_ACTION_ERROR, nil, nil, 0)
	}
}

func (p *parser) stateChange(t transition, b byte) {
	newState := t.state()
	act := t.action()

	if newState != VTPARSE_STATE_NONE {
		exit := EXIT_ACTIONS[p.state]
		enter := ENTRY_ACTIONS[newState]

		if exit != VTPARSE_ACTION_NOP {
			p.action(exit, b)
		}

		if act != VTPARSE_ACTION_NOP {
			p.action(act, b)
		}

		if enter != VTPARSE_ACTION_NOP {
			p.action(enter, b)
		}

		p.state = newState
	} else {
		p.action(act, b)
	}
}
