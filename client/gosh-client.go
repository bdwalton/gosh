package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/bdwalton/gosh/logging"
	"github.com/bdwalton/gosh/network"
	"github.com/bdwalton/gosh/stm"
	"github.com/bdwalton/gosh/vt"
	"golang.org/x/term"
	"zgo.at/termfo"
	"zgo.at/termfo/caps"
)

var (
	agentForward = flag.Bool("ssh_agent_forwarding", false, "If true, listen on a socket to forward SSH agent requests")
	debug        = flag.Bool("debug", false, "If true, enable DEBUG log level for verbose log output")
	initCols     = flag.Int("initial_cols", vt.DEF_COLS, "Numer of columns to start the terminal with")
	initRows     = flag.Int("initial_rows", vt.DEF_ROWS, "Numer of rows to start the terminal with")
	logfile      = flag.String("logfile", "", "If set, logs will be written to this file.")
	remotePort   = flag.String("remote_port", "61000", "Port to dial on remote host")
	remoteHost   = flag.String("remote_host", "", "Remote host to dial")
)

func die(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, msg, args...)
	os.Exit(1)
}

func main() {
	flag.Parse()

	err := logging.Setup(*logfile, *debug)
	if err != nil {
		die("failed to setup logging: %v", err)
	}

	orig, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		die("couldn't make terminal raw: %v", err)
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
		die("couldn't setup network layer: %v", err)
	}
	defer func() {
		if err := gc.Close(); err != nil {
			slog.Error("error closing gosh conn", "err", err)
		}
	}()

	undoAlt := maybeAltScreen()
	defer undoAlt()

	t, err := vt.NewTerminal(*initRows, *initCols)
	if err != nil {
		die("couldn't setup terminal: %v", err)
	}

	var sock net.Conn
	if *agentForward {
		sock, err = openAuthSock()
		if err != nil {
			die("couldn't open auth socket: %v", err)
		}
		defer sock.Close()
	}
	c := stm.NewClient(gc.RemoteAddr(), gc, t, sock)
	c.Run()

	slog.Info("Shutting down")

}

func maybeAltScreen() func() {
	if ti, err := termfo.New(""); err == nil {
		s, ok := ti.Strings[caps.EnterCaMode]
		if ok {
			os.Stdout.Write([]byte(s))
		}

		return func() {
			s, ok := ti.Strings[caps.ExitCaMode]
			if ok {
				os.Stdout.Write([]byte(s))
			}
		}
	} else {
		slog.Warn("error determining terminfo, proceeding without", "err", err)
	}

	return func() {}
}

func openAuthSock() (net.Conn, error) {
	sockPath := os.Getenv("SSH_AUTH_SOCK")
	if sockPath == "" {
		return nil, errors.New("no SSH_AUTH_SOCK set, ignoring agent forwarding")
	}
	return net.Dial("unix", sockPath)
}
