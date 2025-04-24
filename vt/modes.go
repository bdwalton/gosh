package vt

import (
	"fmt"
	"log/slog"
)

// Map of the concatenation of "?" (DEC private) or "" (ansi) and mode
// number (eg: "?4" or "4") to a mode definition, with the defaults
// for that mode specified. These defaults are copied for new
// terminals and applied when the terminal is reset.
var modeDefaults = map[string]*mode{
	"4":     newMode("IRM", IRM, true, CSI_MODE_RESET, false),
	"20":    newMode("LNM", LNM, true, CSI_MODE_RESET, false),
	"?1":    newMode("DECCKM", DECCKM, false, CSI_MODE_RESET, false),
	"?3":    newMode("DECCOLM", DECCOLM, false, CSI_MODE_RESET, false),
	"?4":    newMode("SMOOTH_SCROLL", SMOOTH_SCROLL, false, CSI_MODE_RESET, false),
	"?5":    newMode("REV_VIDEO", REV_VIDEO, false, CSI_MODE_RESET, true),
	"?6":    newMode("DECOM", DECOM, false, CSI_MODE_RESET, false),
	"?7":    newMode("DECAWM", DECAWM, false, CSI_MODE_RESET, false),
	"?8":    newMode("AUTO_REPEAT", AUTO_REPEAT, false, CSI_MODE_RESET, false),
	"?12":   newMode("BLINK_CURSOR", BLINK_CURSOR, false, CSI_MODE_RESET, true),
	"?25":   newMode("SHOW_CURSOR", SHOW_CURSOR, false, CSI_MODE_SET, true),
	"?40":   newMode("XTERM_80_132", XTERM_80_132, false, CSI_MODE_RESET, true),
	"?45":   newMode("REV_WRAP", REV_WRAP, false, CSI_MODE_RESET, false),
	"?1000": newMode("DISABLE_MOUSE_XY", DISABLE_MOUSE_XY, false, CSI_MODE_RESET, false),
	"?1001": newMode("DISABLE_MOUSE_HILITE", DISABLE_MOUSE_HILITE, false, CSI_MODE_RESET, false),
	"?1002": newMode("DISABLE_MOUSE_MOTION", DISABLE_MOUSE_MOTION, false, CSI_MODE_RESET, false),
	"?1003": newMode("DISABLE_MOUSE_ALL", DISABLE_MOUSE_ALL, false, CSI_MODE_RESET, false),
	"?1004": newMode("DISABLE_MOUSE_FOCUS", DISABLE_MOUSE_FOCUS, false, CSI_MODE_RESET, false),
	"?1005": newMode("DISABLE_MOUSE_UTF8", DISABLE_MOUSE_UTF8, false, CSI_MODE_RESET, false),
	"?1006": newMode("DISABLE_MOUSE_SGR", DISABLE_MOUSE_SGR, false, CSI_MODE_RESET, false),
	"?2004": newMode("BRACKET_PASTE", BRACKET_PASTE, false, CSI_MODE_RESET, false),
}

var modeNameToID = map[string]string{
	"IRM":                  "4",
	"LNM":                  "20",
	"DECCKM":               "?1", // Application cursor keys
	"DECCOLM":              "?3", // 132 vs 80 column toggle
	"SMOOTH_SCROLL":        "?4",
	"REV_VIDEO":            "?5",
	"DECOM":                "?6", // DEC origin mode
	"DECAWM":               "?7", // Auto wrap mode
	"AUTO_REPEAT":          "?8",
	"BLINK_CURSOR":         "?12",
	"SHOW_CURSOR":          "?25",
	"XTERM_80_132":         "?40",
	"REV_WRAP":             "?45",
	"DISABLE_MOUSE_XY":     "?1000",
	"DISABLE_MOUSE_HILITE": "?1001",
	"DISABLE_MOUSE_MOTION": "?1002",
	"DISABLE_MOUSE_ALL":    "?1003",
	"DISABLE_MOUSE_FOCUS":  "?1004",
	"DISABLE_MOUSE_UTF8":   "?1005",
	"DISABLE_MOUSE_SGR":    "?1006",
	"BRACKET_PASTE":        "?2004",
}

type mode struct {
	state     rune // CSI_MODE_SET/h or CSI_MODE_RESET/l
	public    bool // This is an ansi mode, if true, DEC private if false
	code      int  // The numeric id for the code that gets placed in params
	transport bool // If true, include in diff output for transport
	name      string
}

func (m *mode) copy() *mode {
	return &mode{
		state:     m.state,
		public:    m.public,
		code:      m.code,
		transport: m.transport,
		name:      m.name,
	}
}

func (m *mode) shouldTransport() bool {
	return m.transport
}

// r should be either CSI_MODE_SET or CSI_MODE_RESET
func (m *mode) setState(state rune) {
	if state != CSI_MODE_RESET && state != CSI_MODE_SET {
		slog.Debug("mode setstate called with invalid state", "state", state)
		return
	}
	m.state = state
}

func (m *mode) enabled() bool {
	return m.state == CSI_MODE_SET
}

func (m *mode) ansiString() string {
	if m.public {
		return fmt.Sprintf("%c%c%d%c", ESC, CSI, m.code, m.state)

	}
	// Ensure we ship the ? for private modes
	return fmt.Sprintf("%c%c?%d%c", ESC, CSI, m.code, m.state)

}

func (m *mode) equal(other *mode) bool {
	return m.code == other.code && m.public == other.public && m.state == other.state
}

func newMode(name string, code int, public bool, state rune, transport bool) *mode {
	return &mode{name: name, code: code, public: public, state: state, transport: transport}
}
