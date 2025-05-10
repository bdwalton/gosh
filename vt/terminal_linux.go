//go:build linux

package vt

import (
	"log/slog"
	"os"
	"os/exec"
)

const utempter = "/usr/lib/x86_64-linux-gnu/utempter/utempter"

func addUtmp(f *os.File, host string) {
	cmd := exec.Command(utempter, "add", host)
	cmd.Stdin = f
	if err := cmd.Run(); err != nil {
		slog.Debug("addUtmp error", "err", err)
	} else {
		slog.Debug("addUtmp", "host", host)
	}
}

func rmUtmp(f *os.File) {
	cmd := exec.Command(utempter, "del")
	cmd.Stdin = f
	if err := cmd.Run(); err != nil {
		slog.Debug("rmUtmp error", "err", err)
	} else {
		slog.Debug("rmUtmp")
	}
}
