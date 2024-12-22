package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bdwalton/gosh/network"
	"github.com/bdwalton/gosh/stm"
	// "github.com/bdwalton/gosh/terminal"
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

	s, err := stm.NewServer(gc)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Printf("GOSH CONNECT %d %s\n", gc.LocalPort(), gc.Base64Key())

	go func() {
		sigQuit := make(chan os.Signal, 1)
		signal.Notify(sigQuit, syscall.SIGINT, syscall.SIGTERM)

		for {
			select {
			case <-sigQuit:
				s.Shutdown()
			}
		}
	}()

	s.Run()
}
