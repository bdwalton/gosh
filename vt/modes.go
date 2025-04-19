package vt

import (
	"fmt"
	"log/slog"
)

// Private modes here will be initialized, diff'd, copied, etc.
var modeToID = map[string]int{
	"DECCKM":               PRIV_DECCKM,
	"DECCOLM":              PRIV_DECCOLM,
	"SMOOTH_SCROLL":        PRIV_SMOOTH_SCROLL,
	"REV_VIDEO":            PRIV_REV_VIDEO,
	"ORIGIN_MODE":          PRIV_ORIGIN_MODE,
	"DECAWM":               PRIV_DECAWM,
	"AUTO_REPEAT":          PRIV_AUTO_REPEAT,
	"BLINK_CURSOR":         PRIV_BLINK_CURSOR,
	"LNM":                  PRIV_LNM,
	"SHOW_CURSOR":          PRIV_SHOW_CURSOR,
	"REVERSE_WRAP":         PRIV_REVERSE_WRAP,
	"XTERM_80_132_ALLOW":   PRIV_XTERM_80_132_ALLOW,
	"DISABLE_MOUSE_XY":     PRIV_DISABLE_MOUSE_XY,
	"DISABLE_MOUSE_HILITE": PRIV_DISABLE_MOUSE_HILITE,
	"DISABLE_MOUSE_MOTION": PRIV_DISABLE_MOUSE_MOTION,
	"DISABLE_MOUSE_ALL":    PRIV_DISABLE_MOUSE_ALL,
	"DISABLE_MOUSE_FOCUS":  PRIV_DISABLE_MOUSE_FOCUS,
	"DISABLE_MOUSE_UTF8":   PRIV_DISABLE_MOUSE_UTF8,
	"DISABLE_MOUSE_SGR":    PRIV_DISABLE_MOUSE_SGR,
	"BRACKET_PASTE":        PRIV_BRACKET_PASTE,
}

var privIDToName = map[int]string{
	PRIV_DECCKM:               "DECCKM",
	PRIV_DECCOLM:              "DECCOLM",
	PRIV_SMOOTH_SCROLL:        "SMOOTH_SCROLL",
	PRIV_REV_VIDEO:            "REV_VIDEO",
	PRIV_ORIGIN_MODE:          "ORIGIN_MODE",
	PRIV_DECAWM:               "DECAWM",
	PRIV_AUTO_REPEAT:          "AUTO_REPEAT",
	PRIV_BLINK_CURSOR:         "BLINK_CURSOR",
	PRIV_LNM:                  "LNM",
	PRIV_SHOW_CURSOR:          "SHOW_CURSOR",
	PRIV_REVERSE_WRAP:         "REVERSE_WRAP",
	PRIV_XTERM_80_132_ALLOW:   "XTERM_80_132_ALLOW",
	PRIV_DISABLE_MOUSE_XY:     "DISABLE_MOUSE_XY",
	PRIV_DISABLE_MOUSE_HILITE: "DISABLE_MOUSE_HILITE",
	PRIV_DISABLE_MOUSE_MOTION: "DISABLE_MOUSE_MOTION",
	PRIV_DISABLE_MOUSE_ALL:    "DISABLE_MOUSE_ALL",
	PRIV_DISABLE_MOUSE_FOCUS:  "DISABLE_MOUSE_FOCUS",
	PRIV_DISABLE_MOUSE_UTF8:   "DISABLE_MOUSE_UTF8",
	PRIV_DISABLE_MOUSE_SGR:    "DISABLE_MOUSE_SGR",
	PRIV_BRACKET_PASTE:        "BRACKET_PASTE",
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

func (m *mode) get() bool {
	return m.state == CSI_MODE_SET
}

func (m *mode) getAnsiString() string {
	if m.private {
		// Ensure we ship the ? for private modes
		return fmt.Sprintf("%c%c?%d%c", ESC, CSI, m.code, m.state)
	}

	return fmt.Sprintf("%c%c%d%c", ESC, CSI, m.code, m.state)
}

func (m *mode) equal(other *mode) bool {
	return m.getAnsiString() == other.getAnsiString()
}

func newPrivMode(code int) *mode {
	// Modes always start in the reset (off) state
	return &mode{code: code, private: true, state: CSI_MODE_RESET}
}
