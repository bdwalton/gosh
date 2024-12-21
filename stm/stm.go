package stm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	"github.com/bdwalton/gosh/network"
	"github.com/bdwalton/gosh/protos/client"
	"github.com/creack/pty"
	"golang.org/x/term"
	"google.golang.org/protobuf/proto"
)

type stmObj struct {
	gc *network.GConn
	os *term.State // original state of the client pty

	ctx       context.Context
	ptmx      *os.File
	cancelPty context.CancelFunc

	shutdown bool
	wg       sync.WaitGroup
	quitCh   chan struct{}
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

func NewServer(gc *network.GConn) (*stmObj, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Start a login shell with a pty.
	// TODO: Use environmet when we're through testing
	shell := "/bin/bash" /* os.Getenv("SHELL") */
	ptmx, err := pty.Start(exec.CommandContext(ctx, shell, "-l"))
	if err != nil {
		return nil, fmt.Errorf("couldn't start pty: %v", err)
	}

	if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
		return nil, fmt.Errorf("couldn't inherit window size: %v", err)
	}

	s := &stmObj{
		gc:        gc,
		ptmx:      ptmx,
		cancelPty: cancel,
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
			_, err = s.gc.Write(p)
			if err != nil {
				// TODO: Log error messages
				fmt.Println(err)
			}
		case <-s.quitCh:
			return
		}
	}
}

func (s *stmObj) ServerShutdown() {
	s.shutdown = true
	s.wg.Wait()
	s.ptmx.Close()
	s.gc.Close()
}

func (s *stmObj) clientShutdown() {
	// TODO: Send shutdown to remote

	if err := term.Restore(int(os.Stdin.Fd()), s.os); err != nil {
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
		if _, err = s.gc.Write(b); err != nil {
			// TODO log errors
			continue
		}

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

func (s *stmObj) handleRemoteInput() {
	s.wg.Add(1)
	defer s.wg.Done()
	for {
		if s.shutdown {
			return
		}
		buf := make([]byte, 1024)
		n, err := s.gc.Read(buf)
		if err != nil {
			// TODO: Log this
			continue
		}

		var msg client.ClientAction
		if err = proto.Unmarshal(buf[:n], &msg); err != nil {
			// TODO log this
			fmt.Println(err)
			continue
		}

		if msg.HasSize() {
			sz := msg.GetSize()
			pty.Setsize(s.ptmx, &pty.Winsize{Rows: uint16(sz.GetHeight()), Cols: uint16(sz.GetWidth())})
		}

		if msg.HasKeys() {
			keys := msg.GetKeys()
			if n, err := s.ptmx.Write(keys); err != nil || n != len(keys) {
				// TODO log this
			}
		}
	}
}

func (s *stmObj) RunServer() {
	go s.handleRemoteInput()

	for {
		select {
		case <-s.quitCh:
			return
		}
	}
}
