package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"

	"github.com/bdwalton/gosh/logging"
	"github.com/bdwalton/gosh/network"
	"github.com/bdwalton/gosh/stm"
)

var (
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

	err := logging.Setup(*logfile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	gc, err := network.NewServer(*portRange)
	if err != nil {
		slog.Error(fmt.Sprintf("Couldn't setup network connection", "err", err))
		os.Exit(1)
	}

	s, err := stm.NewServer(gc)
	if err != nil {
		slog.Error(fmt.Sprintf("Couldn't setup STM server", "err", err))
		os.Exit(1)
	}

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
