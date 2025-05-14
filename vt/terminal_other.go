//go:build !linux

package vt

import (
	"log/slog"
	"os"
)

func addUtmp(f *os.File) {
	slog.Debug("AddUtmp() not implemented on this platform")
}

func rmUtmp(f *os.File) {
	slog.Debug("RmUtmp() not implemented on this platform")
}
