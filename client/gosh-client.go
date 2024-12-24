package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/bdwalton/gosh/logging"
	"github.com/bdwalton/gosh/network"
	"github.com/bdwalton/gosh/stm"
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

	gc, err := network.NewClient(*remoteHost+":"+*remotePort, os.Getenv("GOSH_KEY"))
	if err != nil {
		slog.Error("Couldn't setup network connection", "err", err)
		os.Exit(1)
	}

	c, err := stm.NewClient(gc)
	if err != nil {
		slog.Error("Couldn't setup STM client", "err", err)
		os.Exit(1)
	}

	c.Run()

	slog.Info("Shutting down")
}
