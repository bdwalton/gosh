package stm

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bdwalton/gosh/network"
	"github.com/bdwalton/gosh/protos/client"
	"golang.org/x/term"
	"google.golang.org/protobuf/proto"
)

type stmObj struct {
	gc *network.GConn
	os *term.State // original state of the client pty
	quitCh chan struct{}
}

func NewClient(gc *network.GConn) (*stmObj, error) {
	os, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, fmt.Errorf("couldn't make terminal raw: %v", err)
	}

	s := &stmObj{
		gc:     gc,
		os:     os,
		quitCh: make(chan struct{}),
	}

	return s, nil
}

func (s *stmObj) handleWinCh() {
	defer close(s.quitCh)

	sig := make(chan os.Signal, 10)
	signal.Notify(sig, syscall.SIGWINCH)

	for {
		select {
		case <-sig:
			w, h, err := term.GetSize(int(os.Stdin.Fd()))
			if err != nil {
				// TODO: Add error logging here
				continue
			}
			msg := client.ClientAction_builder{
				Size: client.Resize_builder{
					Width:  proto.Int32(int32(w)),
					Height: proto.Int32(int32(h)),
				}.Build(),
			}.Build()
			p, err := proto.Marshal(msg)
			if err != nil {
				// TODO: Log error messages
				continue
			}
			s.gc.WriteRemote(p)
		case <-s.quitCh:
			return
		}
	}
}

func (s *stmObj) clientShutdown() {
	// TODO: Send shutdown to remote

	if err := term.Restore(os.Stdin.Fd(), s.os); err != nil {
		// TODO: Log error messages
	}

	s.quitCh <- struct{}{}
}

func (s *stmObj) handleInput() {
	var inEsc bool

	char := make([]byte, 1)
	var msg *client.ClientAction

	for {
		_, err := os.Stdin.Read(char)
		if err != nil {
			fmt.Printf("Error on os.Stdin.Read(): %v\n", err)
			continue
		}

		if inEsc {
			switch char[0] {
			case '.':
				s.clientShutdown()
				return
			default:
				msg.SetKeys(append(msg.GetKeys(), char...))
				inEsc = false
			}
		} else {
			msg = client.ClientAction_builder{
				Keys: char,
			}.Build()

			switch char[0] {
			case '\x1e':
				inEsc = true
				continue // Don't immediately send this
			}
		}

		b, err := proto.Marshal(msg)
		if err != nil {
			// TODO: Log errors
			continue
		}

		s.gc.WriteRemote(b)
	}
}

func (s *stmObj) RunClient() {
	go s.handleWinCh()
	go s.handleInput()

	for {
		select {
		case <-s.quitCh:
			return
		}
	}
}
