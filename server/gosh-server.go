package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/bdwalton/gosh/logging"
	"github.com/bdwalton/gosh/network"
	"github.com/bdwalton/gosh/stm"
	"github.com/bdwalton/gosh/vt"
)

var (
	debug     = flag.Bool("debug", false, "If true, enable DEBUG log level for verbose log output")
	detached  = flag.Bool("detached", false, "For use gosh-server to setup a detached version")
	portRange = flag.String("port_range", "61000:61999", "Port range")
	logfile   = flag.String("logfile", "", "If set, logs will be written to this file.")
)

func main() {
	flag.Parse()

	// The remotely instantiated server process will have a
	// controlling tty, etc, but we want to have that process
	// detach from the terminal after the connect string is
	// dumped. This way, the initiating ssh session can be
	// terminated, leaving only the securely connectable gosh
	// server in the background. On modern systems with systemd,
	// or other container management in place, the initiating ssh
	// command may need to invoke something like:
	// `systemd-run --user --scope /path/to/gosh-server arg1`
	if !*detached {
		if err := runDetached(); err != nil {
			fmt.Println("Error detaching:", err)
			os.Exit(1)
		}

		os.Exit(0)
	}

	err := logging.Setup(*logfile, *debug)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	gc, err := network.NewServer(*portRange)
	if err != nil {
		slog.Error("Couldn't setup network connection", "err", err)
		os.Exit(1)
	}
	defer func() {
		if err := gc.Close(); err != nil {
			slog.Error("error closing gosh conn", "err", err)
		}
	}()

	cmd, cancel := getCmd()
	t, err := vt.NewTerminalWithPty(cmd, cancel)
	if err != nil {
		slog.Error("Couldn't setup terminal", "err", err)
		os.Exit(1)
	}

	s := stm.NewServer(gc, t)

	port, pid := gc.LocalPort(), os.Getpid()
	slog.Info("Running", "port", port)
	fmt.Println("GOSH CONNECT", port, gc.Base64Key(), "pid =", pid)

	os.Stdin.Close()
	os.Stdout.Close()
	os.Stderr.Close()

	s.Run()

	slog.Info("Shutting down")
}

func runDetached() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	args := append(os.Args, "--detached")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = cwd
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Ctty: 0,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("couldn't start detached server: %v", err)
	}

	return cmd.Process.Release()
}

func getCmd() (*exec.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	// Start a login shell with a pty.
	shell := os.Getenv("SHELL")
	lshell := "-" + filepath.Base(shell)
	cmd := exec.CommandContext(ctx, shell)
	cmd.Args = []string{lshell}
	// TODO: We should probably clean this a bit, but for now,
	// just pass it all through.
	cmd.Env = os.Environ()

	return cmd, cancel
}
