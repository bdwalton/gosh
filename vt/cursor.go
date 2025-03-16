package vt

import (
	"fmt"
	"strings"
)

type cursor struct {
	row, col int
}

func (c cursor) Copy() cursor {
	return cursor{row: c.row, col: c.col}
}

func (c cursor) getMoveToAnsi() string {
	var sb strings.Builder
	sb.Write([]byte{ESC, ESC_CSI})
	if c.row != 0 {
		sb.WriteString(fmt.Sprintf("%d", c.row+1))
	}
	sb.WriteByte(';')
	if c.col != 0 {
		sb.WriteString(fmt.Sprintf("%d", c.col+1))
	}
	sb.WriteByte(CSI_CUP)
	return sb.String()
}

func (c cursor) equal(other cursor) bool {
	return c.row == other.row && c.col == other.col
}

func (c cursor) String() string {
	return fmt.Sprintf("(%d, %d)", c.row, c.col)
}
