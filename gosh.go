// Copyright (c) 2025, Ben Walton
// All rights reserved.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"github.com/bdwalton/gosh/vt"
	"golang.org/x/term"
)

var (
	agentForward = flag.Bool("ssh_agent_forwarding", false, "If true, listen on a socket to forward SSH agent requests")
	bindServer   = flag.String("bind_server", "any", "Can be ssh, any or a specific IP")
	debug        = flag.Bool("debug", false, "If true, enable DEBUG log level for verbose log output")
	dest         = flag.String("dest", "localhost", "The {username@}localhost to connect to.")
	goshClient   = flag.String("gosh_client", "gosh-client", "The path to the gosh-client executable on the local system.")
	goshSrv      = flag.String("gosh_server", "gosh-server", "The path to the gosh-server executable on the remote system.")
	logfile      = flag.String("logfile", "", "If set, client logs will be written to this file.")
	pprofFile    = flag.String("pprof_file", "", "If set, enable pprof capture to the provided file.")
	remLog       = flag.String("remote_logfile", "", "If set, the remote gosh-server will be asked to log to this file.")
	titlePfx     = flag.String("title_prefix", "[gosh] ", "The prefix applied to the title. Set to '' to disable.")
	useSystemd   = flag.Bool("use_systemd", true, "If true, execute the remote server under systemd so the detached process outlives the ssh connection.")
)

type connectData struct {
	port string
	key  string
}

func main() {
	flag.Parse()

	rows, cols := initialSize()
	connectData, err := runServer(rows, cols)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	runClient(connectData, rows, cols)
}

// runServer will ssh to the remote machine and setup a gosh-server
// process there. On success it will return the port to connect to and
// the session key for the encryption. It will return an error if it
// can't run the remote process or if the remote process doesn't
// return viable connection data.
func runServer(rows, cols int) (*connectData, error) {
	// dest is {username@}host, with username@ optional. feed this
	// to ssh as its natural target argument.
	args := []string{*dest}

	if *useSystemd {
		systemd := []string{"systemd-run", "--user", "--scope"}
		args = append(args, systemd...)
	}

	args = append(args, *goshSrv)
	if *remLog != "" {
		args = append(args, fmt.Sprintf("--logfile=%q", *remLog))
	}

	if *debug {
		args = append(args, "--debug")
	}

	if *agentForward {
		args = append(args, "--ssh_agent_forwarding")
	}

	if *pprofFile != "" {
		args = append(args, fmt.Sprintf("--pprof_file=%q", *pprofFile))
	}

	args = append(args, "--bind_server", *bindServer)
	args = append(args, fmt.Sprintf("--initial_rows=%d", rows))
	args = append(args, fmt.Sprintf("--initial_cols=%d", cols))
	args = append(args, fmt.Sprintf("--title_prefix=%q", *titlePfx))

	cmd := exec.Command("ssh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run %q: %w\n%s", cmd, err, out)
	}

	re := regexp.MustCompile("GOSH CONNECT (\\d+) ([^\\s]+).*")
	m := re.FindStringSubmatch(string(out))
	if len(m) != 3 {
		return nil, fmt.Errorf("couldn't extract port and key; got %v from %q", m, string(out))
	}

	return &connectData{port: m[1], key: m[2]}, nil
}

// runClient never returns. It execs gosh-client with the right args
// and environment.
func runClient(connD *connectData, rows, cols int) {
	args := []string{*goshClient, "--remote_port", connD.port, "--remote_host", hostFromDest(*dest)}
	if *logfile != "" {
		args = append(args, "--logfile", *logfile)
	}

	if *debug {
		args = append(args, "--debug")
	}

	if *agentForward {
		args = append(args, "--ssh_agent_forwarding")
	}

	args = append(args, fmt.Sprintf("--initial_rows=%d", rows))
	args = append(args, fmt.Sprintf("--initial_cols=%d", cols))

	envv := append(os.Environ(), fmt.Sprintf("GOSH_KEY=%s", connD.key))
	syscall.Exec(*goshClient, args, envv)
}

func hostFromDest(dest string) string {
	if strings.Contains(dest, "@") {
		return strings.SplitN(dest, "@", 2)[1]
	}
	return dest
}

func initialSize() (int, int) {
	cols, rows, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "couldn't get terminal size: %v", err)
		return vt.DEF_ROWS, vt.DEF_COLS
	}

	return rows, cols
}
