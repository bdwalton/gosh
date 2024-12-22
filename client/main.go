package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bdwalton/gosh/network"
	"github.com/bdwalton/gosh/stm"
)

var (
	remotePort = flag.String("remote_port", "61000", "Port to dial on remote host")
	remoteHost = flag.String("remote_host", "", "Remote host to dial")
)

func main() {
	flag.Parse()

	gc, err := network.NewClient(*remoteHost+":"+*remotePort, os.Getenv("GOSH_KEY"))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	c, err := stm.NewClient(gc)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	c.Run()
}
