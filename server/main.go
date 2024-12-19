package main

import (
	"flag"
	"fmt"
	"github.com/bdwalton/gosh/network"
	"os"
	"os/signal"
	"syscall"
)

var (
	portRange = flag.String("port_range", "61000:61999", "Port range")
)

func main() {
	flag.Parse()

	gc, err := network.NewServer(*portRange)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("GOSH CONNECT %d %s\n", gc.LocalPort(), gc.Base64Key())

	go gc.RunServer()

	sigQuit := make(chan os.Signal, 1)
	signal.Notify(sigQuit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sigQuit:
			gc.Shutdown()
			os.Exit(0)
		}
	}
}
