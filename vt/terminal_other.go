//go:build !linux

package vt

import (
	"log/slog"
	"os"
)

func AddUtmp(f *os.File, host string) {
	slog.Debug("AddUtmp() not implemented on this platform")
}

func RmUtmp(f *os.File) {
	slog.Debug("RmUtmp() not implemented on this platform")
}
