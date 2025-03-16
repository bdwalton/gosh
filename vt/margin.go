package vt

import (
	"fmt"
	"log/slog"
)

type margin struct {
	val1, val2 int
	set        bool
}

func newMargin(val1, val2 int) margin {
	if val1 >= val2 {
		slog.Error("invalid margin creation request val1 must be < val2", "val1", val1, "val2", val2)
		return margin{}
	}
	return margin{val1: val1, val2: val2, set: true}
}

func (m margin) contains(v int) bool {
	if !m.isSet() || (m.getMin() <= v && v <= m.getMax()) {
		return true
	}
	return false
}

func (m margin) isSet() bool {
	return m.set
}

func (m margin) getMin() int {
	return m.val1
}

func (m margin) getMax() int {
	return m.val2
}

func (m margin) equal(other margin) bool {
	if m.isSet() != other.isSet() || m.getMin() != other.getMin() || m.getMax() != other.getMax() {
		return false
	}

	return true
}

func (m margin) getAnsi(csi rune) string {
	// +1 because we're zero based internally
	return fmt.Sprintf("%c%c%d;%d%c", ESC, ESC_CSI, m.val1+1, m.val2+1, csi)
}

func (m margin) String() string {
	return fmt.Sprintf("(%d,%d)/%t", m.val1, m.val2, m.set)
}
