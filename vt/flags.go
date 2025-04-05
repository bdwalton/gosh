package vt

import "fmt"

type privFlag struct {
	enabled bool
	code    int
}

func (p *privFlag) copy() *privFlag {
	return &privFlag{enabled: p.enabled, code: p.code}
}

func (p *privFlag) set(b bool) {
	p.enabled = b
}

func (p *privFlag) get() bool {
	return p.enabled
}

func (p *privFlag) getAnsiString() string {
	b := CSI_PRIV_DISABLE
	if p.enabled {
		b = CSI_PRIV_ENABLE
	}
	return fmt.Sprintf("%c%c%d%c", ESC, ESC_CSI, p.code, b)
}

func (p *privFlag) equal(other *privFlag) bool {
	return p.getAnsiString() == other.getAnsiString()
}

func newPrivFlag(code int) *privFlag {
	return &privFlag{code: code}
}
