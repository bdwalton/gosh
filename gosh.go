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

func main() {
	flag.Parse()

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
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't run remote server: %v", err)
		os.Exit(1)
	}

	re := regexp.MustCompile("GOSH CONNECT (\\d+) ([^\\s]+).*")
	m := re.FindStringSubmatch(string(out))
	if len(m) != 3 {
		fmt.Fprintf(os.Stderr, "Couldn't extract port and key. Got %v from %q.", m, string(out))
		os.Exit(1)
	}

	args = []string{*goshClient, "--remote_port", m[1], "--remote_host", *host}
	if *logfile != "" {
		args = append(args, "--logfile", *logfile)
	}
	envv := append(os.Environ(), fmt.Sprintf("GOSH_KEY=%s", m[2]))
	syscall.Exec(*goshClient, args, envv)
}
