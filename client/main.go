package main

import (
	"flag"
	"fmt"
	"github.com/bdwalton/gosh/network"
	"os"
	"time"
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

	var i int
	for {
		gc.WriteRemote([]byte(fmt.Sprintf("hello %d", i)))
		i += 1
		time.Sleep(time.Duration(1 * time.Second))
	}
}
