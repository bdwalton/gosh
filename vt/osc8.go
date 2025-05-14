package vt

import "fmt"

const cancelHyperlink = "8;;"

// These will be common across many cells as we instantiate it
// everywhere by default.
var defOSC8 = newHyperlink(cancelHyperlink)

// osc8 objects store the OSC byte sequence for the uri
type osc8 struct {
	data string
}

func newHyperlink(data string) *osc8 {
	return &osc8{data: data}
}

func (o *osc8) equal(other *osc8) bool {
	return o.data == other.data
}

func (o *osc8) ansiString() string {
	return fmt.Sprintf("%c%c%s%c%c", ESC, OSC, o.data, ESC, ST)
}
