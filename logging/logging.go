// Copyright (c) 2025, Ben Walton
// All rights reserved.
package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// See https://github.com/golang/go/issues/62005 for details about why
// we have this. When that issue is closed, we should be able to use
// slog's built in discard handler.
type discardHandler struct {
	slog.JSONHandler
}

func (d *discardHandler) Enabled(context.Context, slog.Level) bool {
	return false
}

func Setup(logfile string, debug bool) error {
	var l *slog.Logger

	if logfile != "" {
		f, err := os.OpenFile(logfile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("couldn't open logfile %q: %w", logfile, err)
		}

		opts := &slog.HandlerOptions{}
		if debug {
			opts.Level = slog.LevelDebug
		}
		h := slog.NewTextHandler(f, opts)
		attrs := []slog.Attr{
			slog.Any("pid", os.Getpid()),
			slog.Any("binary", filepath.Base(os.Args[0])),
		}
		l = slog.New(h.WithAttrs(attrs))
	} else {
		l = slog.New(&discardHandler{})
	}

	slog.SetDefault(l)
	return nil
}
