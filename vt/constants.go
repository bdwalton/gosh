package vt

// Returned when as the version component of the DSC response when the
// vt is queried for XTERM_VERSION CSI > q. This is not representative
// of the overall gosh protocol version, but meant as an internal
// version for the vt emulation.
const GOSH_VT_VER = "0.8"

const (
	// Like it's 1975 baby!
	DEF_ROWS = 24
	DEF_COLS = 80
)

// Single character functions
const (
	BEL = 0x07 // ^G Bell
	BS  = 0x08 // ^H Backspace
	TAB = 0x09 // ^I Tab \t
	LF  = 0x0a // ^J Line feed \n
	VT  = 0x0b // ^K Vertical tab \v
	FF  = 0x0c // ^L Form feed \f
	CR  = 0x0d // ^M Carriage return \r
	SO  = 0x0e // ^N Switch to G1/alternate charset as default
	SI  = 0x0f // ^O Switch to G0 charset as default
)

// ESC sequence controls
const (
	ESC   = 0x1b
	DECSC = '7'  // DECSC - save cursor
	DECRC = '8'  // DECRC - restore cursor
	IND   = 'D'  // IND - index
	NEL   = 'E'  // NEL - newline
	HTS   = 'H'  // HTS - horizontal tab set
	RI    = 'M'  // RI - reverse index
	DCS   = 'P'  // Device control sequence
	RIS   = 'c'  // RIS - Full reset
	CSI   = 0x5b // ]; control sequence introducer
	OSC   = 0x5d // [; operating system command
	ST    = '\\' // string terminator
)

// Control codes
const (
	C1_HTS = 0x88
	C1_DCS = 0x8f
	C1_CSI = 0x9b
	C1_ST  = 0x9c
	C1_OSC = 0x9d
)

// CSI codes
const (
	CSI_ICH        = '@' // insert blank characters
	CSI_CUU        = 'A' // cursor up
	CSI_CUD        = 'B' // cursor down
	CSI_CUF        = 'C' // cursor forward
	CSI_CUB        = 'D' // cursor back
	CSI_CNL        = 'E' // cursor next line
	CSI_CPL        = 'F' // cursor previous line
	CSI_CHA        = 'G' // cursor horizontal attribute
	CSI_CUP        = 'H' // cursor position
	CSI_CHT        = 'I' // cursor forward tabulation
	CSI_ED         = 'J' // erase in display
	CSI_EL         = 'K' // erase in line
	CSI_DL         = 'M' // delete line(s)
	CSI_DCH        = 'P' // delete character(s)
	CSI_POS        = 'R' // cursor position report
	CSI_SU         = 'S' // scroll up
	CSI_SD         = 'T' // scroll down
	CSI_DECST8C    = 'W' // DEC reset tab stops, starting at col 9, every 8 columns
	CSI_ECH        = 'X' // erase characters
	CSI_CBT        = 'Z' // cursor backward tabulation
	CSI_HPA        = '`' // character position absolute (column), default [row,1]
	CSI_HPR        = 'a' // character position relative (column), default [row,col+1]
	CSI_DA         = 'c' // send (primary) device attributes
	CSI_VPA        = 'd' // line position absolute (row), default [1,col]
	CSI_VPR        = 'e' // line position relative (row), default [row+1,col]
	CSI_HVP        = 'f' // horizontal vertical position
	CSI_TBC        = 'g' // tab stop clear
	CSI_MODE_SET   = 'h' // h typically enables or activates something
	CSI_MODE_RESET = 'l' // l typically disables or deactivates something
	CSI_SGR        = 'm' // select graphic rendition
	CSI_DSR        = 'n' // device status report
	CSI_Q_MULTI    = 'q' // overloaded, common for returning xterm name and version
	CSI_DECSTBM    = 'r' // set top and bottom margin
	CSI_DECSLRM    = 's' // set left and right margin
	CSI_XTWINOPS   = 't' // window manipulation, xterm/dtterm stuff mostly
)

// CSI SGR Format codes
const (
	RESET            = 0
	INTENSITY_BOLD   = 1
	INTENSITY_FAINT  = 2
	UNDERLINE_ON     = 4
	BLINK_ON         = 5
	RAPID_BLINK_ON   = 6
	REVERSED_ON      = 7
	INVISIBLE_ON     = 8
	STRIKEOUT_ON     = 9
	PRIMARY_FONT     = 10
	INTENSITY_NORMAL = 22
	UNDERLINE_OFF    = 24
	BLINK_OFF        = 25
	REVERSED_OFF     = 27
	INVISIBLE_OFF    = 28
	STRIKEOUT_OFF    = 29
)

// CSI SGR Color codes
const (
	FG_BLACK          = 30
	FG_RED            = 31
	FG_GREEN          = 32
	FG_YELLOW         = 33
	FG_BLUE           = 34
	FG_MAGENTA        = 35
	FG_CYAN           = 36
	FG_WHITE          = 37
	SET_FG            = 38
	FG_DEF            = 39
	BG_BLACK          = 40
	BG_RED            = 41
	BG_GREEN          = 42
	BG_YELLOW         = 43
	BG_BLUE           = 44
	BG_MAGENTA        = 45
	BG_CYAN           = 46
	BG_WHITE          = 47
	SET_BG            = 48
	BG_DEF            = 49
	FG_BRIGHT_BLACK   = 90
	FG_BRIGHT_RED     = 91
	FG_BRIGHT_GREEN   = 92
	FG_BRIGHT_YELLOW  = 93
	FG_BRIGHT_BLUE    = 94
	FG_BRIGHT_MAGENTA = 95
	FG_BRIGHT_CYAN    = 96
	FG_BRIGHT_WHITE   = 97
	BG_BRIGHT_BLACK   = 100
	BG_BRIGHT_RED     = 101
	BG_BRIGHT_GREEN   = 102
	BG_BRIGHT_YELLOW  = 103
	BG_BRIGHT_BLUE    = 104
	BG_BRIGHT_MAGENTA = 105
	BG_BRIGHT_CYAN    = 106
	BG_BRIGHT_WHITE   = 107
)

// CSI mode parameter codes
const (
	IRM = 4  // Insert mode
	LNM = 20 // Automatic New Line Mode
)

// CSI private mode parameter codes
const (
	DECCKM                 = 1    // DEC application cursor keys
	DECCOLM                = 3    // DEC 80 (l) / 132 (h) mode DECCOLM
	SMOOTH_SCROLL          = 4    // Smooth scroll DECSCLM
	REV_VIDEO              = 5    // Reverse video DECSCNM
	DECOM                  = 6    // Origin Mode DECOM
	DECAWM                 = 7    // DEC autowrap mode, default reset
	AUTO_REPEAT            = 8    // Auto-repeat keys DECARM
	MOUSE_XY_PRESS         = 9    // Mouse X & Y on button press
	BLINK_CURSOR           = 12   // Start blinking cursor
	SHOW_CURSOR            = 25   // Show cursor DECTCEM
	XTERM_80_132           = 40   // Xterm specific to enable/disable 80/132 col reset
	REV_WRAP               = 45   // Xterm's reverse-wraparound mode
	MOUSE_XY_PRESS_RELEASE = 1000 // Mouse X & Y on button press and release
	MOUSE_HILIGHT          = 1001 // Hilight Mouse Tracking
	MOUSE_MOTION           = 1002 // Cell Motion Mouse Tracking
	MOUSE_ALL              = 1003 // All Motion Mouse Tracking
	MOUSE_FOCUS            = 1004 // FocusIn/FocusOut events
	MOUSE_UTF8             = 1005 // UTF-8 Mouse Mode
	MOUSE_SGR              = 1006 // SGR Mouse Mode
	MOUSE_ALT              = 1007 // Alternate scroll mode
	BRACKET_PASTE          = 2004 // Bracketed paste, ala xterm
)

// OSC actions
const (
	OSC_ICON_TITLE = "0"
	OSC_ICON       = "1"
	OSC_TITLE      = "2"
	OSC_SETSIZE    = "X" // Gosh-specific
)

// Modes for CSI_TBC
const (
	TBC_CUR = 0 // clear current tab stop
	TBC_ALL = 3 // clear all tab stops
)

// For erase in display and erase line
const (
	ERASE_FROM_CUR = 0 // from cursor to end of line/screen (includes cursor pos)
	ERASE_TO_CUR   = 1 // from start of line/screen to cursor (includes cursor pos)
	ERASE_ALL      = 2 // entire line/screen
)

const (
	XTWINOPS_SAVE    = 22
	XTWINOPS_RESTORE = 23
)
