// Copyright (c) 2025, Ben Walton
// All rights reserved.
//go:build linux

package vt

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
)

const utempter = "/usr/lib/x86_64-linux-gnu/utempter/utempter"

func addUtmp(f *os.File) {
	// We're not going to include real "remote" host info here as
	// the IP can change and that would required updating as that
	// happens. It's also not necessary. Instead, we'll just note
	// that we're Gosh and our PID.
	host := fmt.Sprintf("gosh[%d]", os.Getpid())
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
