package vt

import (
	"fmt"
	"log/slog"
)

// Public modes here will be initialized, diff'd, copied, etc.
var pubModeToID = map[string]int{
	"IRM": IRM,
	"LNM": LNM,
}

// Public modes
var pubIDToName = map[int]string{
	IRM: "IRM",
	LNM: "LNM",
}

// Private modes here will be initialized, diff'd, copied, etc.
var privModeToID = map[string]int{
	"DECCKM":               DECCKM,
	"DECCOLM":              DECCOLM,
	"SMOOTH_SCROLL":        SMOOTH_SCROLL,
	"REV_VIDEO":            REV_VIDEO,
	"DECOM":                DECOM,
	"DECAWM":               DECAWM,
	"AUTO_REPEAT":          AUTO_REPEAT,
	"BLINK_CURSOR":         BLINK_CURSOR,
	"LNM":                  LNM,
	"SHOW_CURSOR":          SHOW_CURSOR,
	"REVERSE_WRAP":         REV_WRAP,
	"XTERM_80_132_ALLOW":   XTERM_80_132,
	"DISABLE_MOUSE_XY":     DISABLE_MOUSE_XY,
	"DISABLE_MOUSE_HILITE": DISABLE_MOUSE_HILITE,
	"DISABLE_MOUSE_MOTION": DISABLE_MOUSE_MOTION,
	"DISABLE_MOUSE_ALL":    DISABLE_MOUSE_ALL,
	"DISABLE_MOUSE_FOCUS":  DISABLE_MOUSE_FOCUS,
	"DISABLE_MOUSE_UTF8":   DISABLE_MOUSE_UTF8,
	"DISABLE_MOUSE_SGR":    DISABLE_MOUSE_SGR,
	"BRACKET_PASTE":        BRACKET_PASTE,
}

// Private modes
var privIDToName = map[int]string{
	DECCKM:               "DECCKM",
	DECCOLM:              "DECCOLM",
	SMOOTH_SCROLL:        "SMOOTH_SCROLL",
	REV_VIDEO:            "REV_VIDEO",
	DECOM:                "DECOM",
	DECAWM:               "DECAWM",
	AUTO_REPEAT:          "AUTO_REPEAT",
	BLINK_CURSOR:         "BLINK_CURSOR",
	LNM:                  "LNM",
	SHOW_CURSOR:          "SHOW_CURSOR",
	REV_WRAP:             "REVERSE_WRAP",
	XTERM_80_132:         "XTERM_80_132",
	DISABLE_MOUSE_XY:     "DISABLE_MOUSE_XY",
	DISABLE_MOUSE_HILITE: "DISABLE_MOUSE_HILITE",
	DISABLE_MOUSE_MOTION: "DISABLE_MOUSE_MOTION",
	DISABLE_MOUSE_ALL:    "DISABLE_MOUSE_ALL",
	DISABLE_MOUSE_FOCUS:  "DISABLE_MOUSE_FOCUS",
	DISABLE_MOUSE_UTF8:   "DISABLE_MOUSE_UTF8",
	DISABLE_MOUSE_SGR:    "DISABLE_MOUSE_SGR",
	BRACKET_PASTE:        "BRACKET_PASTE",
}

type mode struct {
	state   rune // CSI_MODE_SET/h or CSI_MODE_RESET/l
	private bool
	code    int
}

func (m *mode) copy() *mode {
	return &mode{state: m.state, private: m.private, code: m.code}
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
	if m.private {
		// Ensure we ship the ? for private modes
		return fmt.Sprintf("%c%c?%d%c", ESC, CSI, m.code, m.state)
	}

	return fmt.Sprintf("%c%c%d%c", ESC, CSI, m.code, m.state)
}

func (m *mode) equal(other *mode) bool {
	return m.code == other.code && m.private == other.private && m.state == other.state
}

func newMode(code int) *mode {
	// Modes always start in the reset (off) state
	return &mode{code: code, state: CSI_MODE_RESET}
}

func newPrivMode(code int) *mode {
	m := newMode(code)
	m.private = true
	return m
}
