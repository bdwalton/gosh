package vt

const (
	ESC     = 0x1b
	ESC_DCS = 'P'
	C1_DCS  = 0x8f
	ESC_CSI = 0x5b // ]
	ESC_OSC = 0x5d // [
	ESC_ST  = '\\'
	C1_CSI  = 0x9b
	C1_ST   = 0x9c
	C1_OSC  = 0x9d
)

// Control codes
const (
	CTRL_BEL = 0x07 // ^G Bell
	CTRL_BS  = 0x08 // ^H Backspace
	CTRL_TAB = 0x09 // ^I Tab
	CTRL_LF  = 0x0a // ^J Line feed
	CTRL_FF  = 0x0c // ^L Form feed
	CTRL_CR  = 0x0d // ^M Carriage return
	CTRL_HTS = 0x88 // Horizontal tab stop
	CTRL_ST  = 0x9c // ST string terminator
)

// CSI codes
const (
	CSI_CUU          = 'A' // cursor up
	CSI_CUD          = 'B' // cursor down
	CSI_CUF          = 'C' // cursor forward
	CSI_CUB          = 'D' // cursor back
	CSI_CNL          = 'E' // cursor next line
	CSI_CPL          = 'F' // cursor previous line
	CSI_CHA          = 'G' // cursor horizontal attribute
	CSI_CUP          = 'H' // cursor position
	CSI_ED           = 'J' // erase in display
	CSI_EL           = 'K' // erase in line
	CSI_SU           = 'S' // scroll up
	CSI_SD           = 'T' // scroll down
	CSI_HVP          = 'f' // horizontal vertical position
	CSI_SGR          = 'm' // select graphic rendition
	CSI_DSR          = 'n' // device status report
	CSI_PRIV_ENABLE  = 'h' // h typically enables or activates something
	CSI_PRIV_DISABLE = 'l' // l typically disables or deactivates something
	CSI_DECSTBM      = 'r' // set top and bottom margin
	CSI_DECSLRM      = 's' // set left and right margin
)

// CSI SGR Format codes
const (
	RESET = iota
	INTENSITY_BOLD
	INTENSITY_DIM
	ITALIC_ON
	UNDERLINE_ON
	BLINK_ON
	REVERSED_ON
	INVISIBLE_ON
	STRIKEOUT_ON
	PRIMARY_FONT
	ALT_FONT_1
	ALT_FONT_2
	ALT_FONT_3
	ALT_FONT_4
	ALT_FONT_5
	ALT_FONT_6
	ALT_FONT_7
	ALT_FONT_8
	ALT_FONT_9
	FRAKTUR
	DBL_UNDERLINE
	INTENSITY_NORMAL
	ITALIC_OFF
	UNDERLINE_OFF
	BLINK_OFF
	UNUSED_PROPORTIONAL_SPACING
	REVERSED_OFF
	INVISIBLE_OFF
	STRIKEOUT_OFF
)

// font intensities
const (
	FONT_NORMAL = iota
	FONT_BOLD
	FONT_DIM
)

// underline styles
const (
	UNDERLINE_NONE = iota
	UNDERLINE_SINGLE
	UNDERLINE_DOUBLE
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

// CSI private mode parameter codes
const (
	PRIV_CSI_DECAWM = 7  // DEC autowrap mode, default reset
	PRIV_CSI_LNM    = 20 // Line Feed/New Line Mode, default reset
)
