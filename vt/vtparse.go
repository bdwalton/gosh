package vt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"unicode/utf8"
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

type dispatcher interface {
	Print(*parser, rune)
	Handle(*parser, pAction, byte)
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
		p.d.Print(p, r)
	}
}

func (p *parser) ParseByte(b byte) {
	p.stateChange(STATE_TABLE[p.state][b], b)
}

func (p *parser) action(a pAction, b byte) {
	switch a {
	case VTPARSE_ACTION_PRINT:
		p.d.Print(p, rune(b))
	case VTPARSE_ACTION_EXECUTE, VTPARSE_ACTION_HOOK, VTPARSE_ACTION_PUT, VTPARSE_ACTION_OSC_START, VTPARSE_ACTION_OSC_PUT, VTPARSE_ACTION_OSC_END, VTPARSE_ACTION_UNHOOK, VTPARSE_ACTION_CSI_DISPATCH, VTPARSE_ACTION_ESC_DISPATCH:
		p.d.Handle(p, a, b)
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
		p.d.Handle(p, VTPARSE_ACTION_ERROR, 0)
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

type disp struct {
}

func (d *disp) Print(p *parser, r rune) {
	fmt.Println("Received action PRINT")
	if r != 0 {
		fmt.Printf("Char: 0x%02x ('%c')\n", r, r)
	}
	if l := len(p.intermediate_chars); l > 0 {
		fmt.Printf("%d Intermediate chars:\n", l)
		for _, ch := range p.intermediate_chars {
			fmt.Printf("  0x%02x ('%c')\n", ch, ch)
		}
	}
	if l := len(p.params); l > 0 {
		fmt.Printf("%d Parameters:\n", l)
		for _, n := range p.params {
			fmt.Printf("\t%d\n", n)
		}
	}
	fmt.Println()
}

func (d *disp) Handle(p *parser, a pAction, b byte) {
	fmt.Println("Received action", ACTION_NAMES[a])
	if b != 0 {
		fmt.Printf("Char: 0x%02x ('%c')\n", b, b)
	}
	if l := len(p.intermediate_chars); l > 0 {
		fmt.Printf("%d Intermediate chars:\n", l)
		for _, ch := range p.intermediate_chars {
			fmt.Printf("  0x%02x ('%c')\n", ch, ch)
		}
	}
	if l := len(p.params); l > 0 {
		fmt.Printf("%d Parameters:\n", l)
		for _, n := range p.params {
			fmt.Printf("\t%d\n", n)
		}
	}
	fmt.Println()
}

func main() {
	vtp := newParser(&disp{})
	reader := bufio.NewReader(os.Stdin)

	for {
		// To support UTF-8, we need to handle some of the
		// signal bytes that ANSI codes may look for (0x9b,
		// 0x9c, etc) while also preferring to conume mutiple
		// bytes as needed for UTF-8. The only time that we
		// might find a byte that is a valid ANSI bytes with
		// the high bit set is when it would be in byte 2, 3
		// or 4 in a valid multi-byte character. That should
		// make it difficult to infer the wrong intent here,
		// so we can default to looking for runes and fall
		// back to bytes.
		r, sz, err := reader.ReadRune()
		switch {
		case sz == 0 || err != nil:
			if err == io.EOF {
				os.Exit(0)
			}
			os.Exit(1)
		case r == utf8.RuneError:
			if err := reader.UnreadRune(); err != nil {
				break
			}
			b, err := reader.ReadByte()
			if err != nil {
				break
			}
			vtp.ParseByte(b)
		case sz == 1:
			// Send single byte runes through parsebyte so
			// they get the full treatment of the lookup
			// tables. Only mutli-byte runes will be
			// diverted separately.
			vtp.ParseByte(byte(r))
		default:
			// Multi-byte runes won't by viable in our
			// state transntion lookup table, so we'll
			// handle them specially.
			vtp.ParseRune(r)
		}
	}
}
