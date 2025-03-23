package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"syscall"
)

var (
	debug      = flag.Bool("debug", false, "If true, enable DEBUG log level for verbose log output")
	goshClient = flag.String("gosh_client", "gosh-client", "The path to the gosh-client executable on the local system.")
	goshSrv    = flag.String("gosh_server", "gosh-server", "The path to the gosh-server executable on the remote system.")
	logfile    = flag.String("logfile", "", "If set, client logs will be written to this file.")
	host       = flag.String("remote_host", "localhost", "The host to connect to.")
	remLog     = flag.String("remote_logfile", "", "If set, the remote gosh-server will be asked to log to this file.")
	useSystemd = flag.Bool("use_systemd", true, "If true, execute the remote server under systemd so the detached process outlives the ssh connection.")
)

type connectData struct {
	port string
	key  string
}

func main() {
	flag.Parse()

	connectData, err := runServer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running server: %v", err)
		os.Exit(1)
	}

	runClient(connectData)
}

// runServer will ssh to the remote machine and setup a gosh-server
// process there. On success it will return the port to connect to and
// the session key for the encryption. It will return an error if it
// can't run the remote process or if the remote process doesn't
// return viable connection data.
func runServer() (*connectData, error) {
	args := []string{*host}

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

	cmd := exec.Command("ssh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cmd, err)
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
func runClient(connD *connectData) {
	args := []string{*goshClient, "--remote_port", connD.port, "--remote_host", *host}
	if *logfile != "" {
		args = append(args, "--logfile", *logfile)
	}
	if *debug {
		args = append(args, "--debug")
	}
	envv := append(os.Environ(), fmt.Sprintf("GOSH_KEY=%s", connD.key))
	syscall.Exec(*goshClient, args, envv)
}
