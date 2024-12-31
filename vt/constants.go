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
	CSI_CUU = 'A' // cursor up
	CSI_CUD = 'B' // cursor down
	CSI_CUF = 'C' // cursor forward
	CSI_CUB = 'D' // cursor back
	CSI_CNL = 'E' // cursor next line
	CSI_CPL = 'F' // cursor previous line
	CSI_CHA = 'G' // cursor horizontal attribute
	CSI_CUP = 'H' // cursor position
	CSI_ED  = 'J' // erase in display
	CSI_EL  = 'K' // erase in line
	CSI_SU  = 'S' // scroll up
	CSI_SD  = 'T' // scroll down
	CSI_HVP = 'f' // horizontal vertical position
	CSI_SGR = 'm' // select graphic rendition
	CSI_DSR = 'n' // device status report
)
