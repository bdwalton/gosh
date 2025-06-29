// Copyright (c) 2025, Ben Walton
// All rights reserved.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"syscall"

	"github.com/bdwalton/gosh/logging"
	"github.com/bdwalton/gosh/network"
	"github.com/bdwalton/gosh/stm"
	"github.com/bdwalton/gosh/vt"
)

var (
	agentForward = flag.Bool("ssh_agent_forwarding", false, "If true, listen on a socket to forward SSH agent requests")
	bindServer   = flag.String("bind_server", "any", "Can be ssh, any or a specific IP")
	debug        = flag.Bool("debug", false, "If true, enable DEBUG log level for verbose log output")
	defTerm      = flag.String("default_terminal", "xterm-256color", "Default TERM value if not set by remote environment")
	detached     = flag.Bool("detached", false, "For use gosh-server to setup a detached version")
	initCols     = flag.Int("initial_cols", vt.DEF_COLS, "Numer of columns to start the terminal with")
	initRows     = flag.Int("initial_rows", vt.DEF_ROWS, "Numer of rows to start the terminal with")
	logfile      = flag.String("logfile", "", "If set, logs will be written to this file.")
	portRange    = flag.String("port_range", "60000:61000", "Port range")
	pprofFile    = flag.String("pprof_file", "", "If set, enable pprof capture to the provided file.")
	titlePfx     = flag.String("title_prefix", "[gosh] ", "The prefix applied to the title. Set to '' to disable.")
)

func die(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, msg, args...)
	os.Exit(1)
}

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
			die("error detaching: %v", err)
		}

		os.Exit(0)
	}

	err := logging.Setup(*logfile, *debug)
	if err != nil {
		die("failed to setup logging: %v", err)
	}

	if *pprofFile != "" {
		cp, err := os.Create(*pprofFile)
		if err != nil {
			slog.Error("couldn't create pprof file", "err", err)
		} else {
			defer cp.Close()
			if err = pprof.StartCPUProfile(cp); err != nil {
				slog.Error("couldn't start profiling", "err", err)
			} else {
				defer pprof.StopCPUProfile()
			}
		}
		slog.Debug("enabled cpu profiling", "output", *pprofFile)
	}

	gc, err := network.NewServer(getIP(*bindServer), *portRange)
	if err != nil {
		die("couldn't setup network layer: %v", err)
	}
	defer func() {
		if err := gc.Close(); err != nil {
			slog.Error("error closing gosh conn", "err", err)
		}
	}()

	var sock net.Listener
	if *agentForward {
		sock, err = openAuthSock()
		if err != nil {
			die("couldn't open auth socket: %v", err)
		}
		defer sock.Close()
		// Do this before we start the terminal and run the command
		os.Setenv("SSH_AUTH_SOCK", sock.Addr().String())
	}

	cmd, cancel := getCmd()
	t, err := vt.NewTerminalWithPty(*initRows, *initCols, cmd, cancel)
	if err != nil {
		die("couldn't setup terminal: %v", err)
	}
	t.SetTitlePrefix(*titlePfx)

	s := stm.NewServer(gc, t, sock)

	port, pid := gc.LocalPort(), os.Getpid()
	slog.Info("Running", "port", port)

	// TODO: Maybe we spit out a protocol version identifier here
	// in the future?
	fmt.Printf("GOSH CONNECT %d %s (pid=%d)\n", port, gc.Base64Key(), pid)

	os.Stdin.Close()
	os.Stdout.Close()
	os.Stderr.Close()

	s.Run()

	slog.Info("Shutting down")
}

func runDetached() error {
	env := os.Environ()
	if os.Getenv("TERM") == "" {
		env = append(env, "TERM="+*defTerm)
	}
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
	cmd.Env = env

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

func remIP() string {
	if sshC := os.Getenv("SSH_CONNECTION"); sshC != "" {
		parts := strings.SplitN(sshC, " ", 4)
		return parts[0]
	}
	return ""
}

func localIP() string {
	if sshC := os.Getenv("SSH_CONNECTION"); sshC != "" {
		parts := strings.SplitN(sshC, " ", 4)
		return parts[2]
	}
	return ""
}

func getIP(flagv string) string {
	switch flagv {
	case "any":
		return "" // clients will just join this to ":<port>"
	case "ssh":
		return localIP()
	default:
		return flagv
	}
}

func openAuthSock() (net.Listener, error) {
	// net.Listen doesn't allow specifying file mode for
	// the socket, so work around that by tightening the
	// umask. restore it after as we don't want to
	// interfere with user intent.
	om := syscall.Umask(0177)
	sockPath := filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), fmt.Sprintf("ssh-agent-gosh.%d.socket", os.Getpid()))
	l, err := net.Listen("unix", sockPath)
	syscall.Umask(om)
	if err != nil {
		slog.Debug("failed to open unix socket", "path", sockPath, "err", err)
		return nil, err
	}

	os.Setenv("SSH_AUTH_SOCK", sockPath)
	os.Setenv("GOSH_AUTH_SOCK", sockPath)
	slog.Debug("returning auth socket", "addr", l.Addr())
	return l, nil
}
