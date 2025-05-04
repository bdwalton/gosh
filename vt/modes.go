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
	"4":     newMode("IRM", IRM, true, CSI_MODE_RESET),
	"20":    newMode("LNM", LNM, true, CSI_MODE_RESET),
	"?1":    newMode("DECCKM", DECCKM, false, CSI_MODE_RESET),
	"?3":    newMode("DECCOLM", DECCOLM, false, CSI_MODE_RESET),
	"?4":    newMode("SMOOTH_SCROLL", SMOOTH_SCROLL, false, CSI_MODE_RESET),
	"?5":    newMode("REV_VIDEO", REV_VIDEO, false, CSI_MODE_RESET),
	"?6":    newMode("DECOM", DECOM, false, CSI_MODE_RESET),
	"?7":    newMode("DECAWM", DECAWM, false, CSI_MODE_SET),
	"?8":    newMode("AUTO_REPEAT", AUTO_REPEAT, false, CSI_MODE_RESET),
	"?9":    newMode("MOUSE_XY_PRESS", MOUSE_XY_PRESS, false, CSI_MODE_RESET),
	"?12":   newMode("BLINK_CURSOR", BLINK_CURSOR, false, CSI_MODE_RESET),
	"?25":   newMode("SHOW_CURSOR", SHOW_CURSOR, false, CSI_MODE_SET),
	"?40":   newMode("XTERM_80_132", XTERM_80_132, false, CSI_MODE_RESET),
	"?45":   newMode("REV_WRAP", REV_WRAP, false, CSI_MODE_RESET),
	"?1000": newMode("MOUSE_XY_PRESS_RELEASE", MOUSE_XY_PRESS_RELEASE, false, CSI_MODE_RESET),
	"?1001": newMode("MOUSE_HILITE", MOUSE_HILIGHT, false, CSI_MODE_RESET),
	"?1002": newMode("MOUSE_MOTION", MOUSE_MOTION, false, CSI_MODE_RESET),
	"?1003": newMode("MOUSE_ALL", MOUSE_ALL, false, CSI_MODE_RESET),
	"?1004": newMode("MOUSE_FOCUS", MOUSE_FOCUS, false, CSI_MODE_RESET),
	"?1005": newMode("MOUSE_UTF8", MOUSE_UTF8, false, CSI_MODE_RESET),
	"?1006": newMode("MOUSE_SGR", MOUSE_SGR, false, CSI_MODE_RESET),
	"?1007": newMode("MOUSE_ALT", MOUSE_ALT, false, CSI_MODE_RESET),
	"?2004": newMode("BRACKET_PASTE", BRACKET_PASTE, false, CSI_MODE_RESET),
}

var modeNameToID = map[string]string{
	"IRM":                    "4",
	"LNM":                    "20",
	"DECCKM":                 "?1", // Application cursor keys
	"DECCOLM":                "?3", // 132 vs 80 column toggle
	"SMOOTH_SCROLL":          "?4",
	"REV_VIDEO":              "?5",
	"DECOM":                  "?6", // DEC origin mode
	"DECAWM":                 "?7", // Auto wrap mode
	"AUTO_REPEAT":            "?8",
	"MOUSE_XY_PRESS":         "?9",
	"BLINK_CURSOR":           "?12",
	"SHOW_CURSOR":            "?25",
	"XTERM_80_132":           "?40",
	"REV_WRAP":               "?45",
	"MOUSE_XY_PRESS_RELEASE": "?1000",
	"MOUSE_HILITE":           "?1001",
	"MOUSE_MOTION":           "?1002",
	"MOUSE_ALL":              "?1003",
	"MOUSE_FOCUS":            "?1004",
	"MOUSE_UTF8":             "?1005",
	"MOUSE_SGR":              "?1006",
	"MOUSE_ALT":              "?1007",
	"BRACKET_PASTE":          "?2004",
}

// Modes in this list will be transported to the client. All other
// modes are used for local purposes only.
//
// Keep it sorted - or at least stable - as tests will depend on
// output order for their validation in some cases.
var transportModes = []string{
	"BLINK_CURSOR",
	"DECCKM",
	"REV_VIDEO",
	"SHOW_CURSOR",
	"XTERM_80_132",
	"MOUSE_XY_PRESS",
	"MOUSE_XY_PRESS_RELEASE",
	"MOUSE_MOTION",
	"MOUSE_ALL",
	"MOUSE_FOCUS",
	"MOUSE_UTF8",
	"MOUSE_SGR",
	"MOUSE_ALT",
}

// For convenience in logging state changes
var modeStateNames = map[rune]string{
	CSI_MODE_RESET: "RESET",
	CSI_MODE_SET:   "SET",
}

type mode struct {
	state  rune // CSI_MODE_SET/h or CSI_MODE_RESET/l
	public bool // This is an ansi mode, if true, DEC private if false
	code   int  // The numeric id for the code that gets placed in params
	name   string
}

func (m *mode) copy() *mode {
	return &mode{
		state:  m.state,
		public: m.public,
		code:   m.code,
		name:   m.name,
	}
}

// r should be either CSI_MODE_SET or CSI_MODE_RESET
func (m *mode) setState(state rune) {
	if state != CSI_MODE_RESET && state != CSI_MODE_SET {
		slog.Debug("mode setState called with invalid state", "state", state)
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

func newMode(name string, code int, public bool, state rune) *mode {
	return &mode{name: name, code: code, public: public, state: state}
}
