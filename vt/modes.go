package vt

import "fmt"

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
	enabled, private bool
	code             int
}

func (m *mode) copy() *mode {
	return &mode{enabled: m.enabled, private: m.private, code: m.code}
}

func (m *mode) set(b bool) {
	m.enabled = b
}

func (m *mode) get() bool {
	return m.enabled
}

func (m *mode) getAnsiString() string {
	b := CSI_MODE_RESET
	if m.enabled {
		b = CSI_MODE_SET
	}
	return fmt.Sprintf("%c%c%d%c", ESC, ESC_CSI, m.code, b)
}

func (m *mode) equal(other *mode) bool {
	return m.getAnsiString() == other.getAnsiString()
}

func newPrivMode(code int, set bool) *mode {
	return &mode{code: code, private: true, enabled: set}
}
