package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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

func Setup(logfile string) error {
	var l *slog.Logger

	if logfile != "" {
		f, err := os.OpenFile(logfile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0700)
		if err != nil {
			return fmt.Errorf("couldn't open logfile %q: %v", logfile, err)
		}

		l = slog.New(slog.NewTextHandler(f, nil))
	} else {
		l = slog.New(&discardHandler{})
	}

	slog.SetDefault(l)
	return nil
}
