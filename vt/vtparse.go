package vt

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

type dispatcher interface {
	print(rune)
	// action, params, intermediate_chars, last byte
	handle(pAction, []int, []rune, byte)
}

type parser struct {
	state              pState
	d                  dispatcher
	intermediate_chars []rune
	num_intermediate   int
	params             []int
}

func newParser(d dispatcher) *parser {
	return &parser{
		state:              VTPARSE_STATE_GROUND,
		d:                  d,
		params:             make([]int, 0, MAX_EXPECTED_PARAMS),
		intermediate_chars: make([]rune, 0, MAX_EXPECTED_INTERMEDIATE),
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
		p.d.handle(act, p.params, p.intermediate_chars, b)
	case VTPARSE_ACTION_IGNORE:
		// Do nothing
	case VTPARSE_ACTION_COLLECT:
		p.intermediate_chars = append(p.intermediate_chars, rune(b))
	case VTPARSE_ACTION_PARAM:
		if b == ';' || b == ':' { // The ; is more common, but : is allowed
			p.params = append(p.params, 0)

		} else {
			if len(p.params) == 0 {
				p.params = append(p.params, 0)
			}

			cp := len(p.params) - 1
			p.params[cp] = (p.params[cp]*10 + int(b-'0'))
		}
	case VTPARSE_ACTION_CLEAR:
		p.intermediate_chars = p.intermediate_chars[:0]
		p.params = p.params[:0]
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
