package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/bdwalton/gosh/logging"
	"github.com/bdwalton/gosh/network"
	"github.com/bdwalton/gosh/stm"
	"github.com/bdwalton/gosh/vt"
	"golang.org/x/term"
)

var (
	debug      = flag.Bool("debug", false, "If true, enable DEBUG log level for verbose log output")
	logfile    = flag.String("logfile", "", "If set, logs will be written to this file.")
	remotePort = flag.String("remote_port", "61000", "Port to dial on remote host")
	remoteHost = flag.String("remote_host", "", "Remote host to dial")
)

func main() {
	flag.Parse()

	err := logging.Setup(*logfile, *debug)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	orig, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		slog.Error("couldn't make terminal raw", "err", err)
		os.Exit(1)
	}
	defer func(orig *term.State) {
		if err := term.Restore(int(os.Stdin.Fd()), orig); err != nil {
			slog.Error("couldn't restore terminal state", "err", err)
		}

		if err := os.Stdin.Close(); err != nil {
			slog.Error("error closing stdin", "err", err)
		}
	}(orig)

	gc, err := network.NewClient(*remoteHost+":"+*remotePort, os.Getenv("GOSH_KEY"))
	if err != nil {
		slog.Error("Couldn't setup network connection", "err", err)
		os.Exit(1)
	}

	t, err := vt.NewTerminal()
	if err != nil {
		slog.Error("Couldn't setup terminal", "err", err)
		os.Exit(1)
	}

	c := stm.NewClient(gc, t)
	c.Run()

	slog.Info("Shutting down")
}
