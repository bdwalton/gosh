package stm

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bdwalton/gosh/network"
	"github.com/bdwalton/gosh/protos/transport"
	"github.com/creack/pty"
	"golang.org/x/term"
	"google.golang.org/protobuf/proto"
	tspb "google.golang.org/protobuf/types/known/timestamppb"
	"os/exec"
)

const (
	CLIENT = iota
	SERVER
)

type stmObj struct {
	gc *network.GConn
	os *term.State // original state of the client pty

	ctx       context.Context
	ptmx      *os.File
	cancelPty context.CancelFunc

	st         uint8
	shutdown   bool
	ptyRunning bool
	wg         sync.WaitGroup
}

func NewClient(gc *network.GConn) (*stmObj, error) {
	fd := int(os.Stdin.Fd())
	os, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("couldn't make terminal raw: %v", err)
	}

	// Any use of Fd() will set the descriptor non-blocking, so we
	// need to change that here.NewClient
	if err := syscall.SetNonblock(fd, true); err != nil {
		return nil, fmt.Errorf("couldn't set ptmx non-blocking: %v", err)
	}

	s := &stmObj{
		gc: gc,
		os: os,
		st: CLIENT,
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

	// Any use of Fd(), including in the InheritSize call above,
	// will set the descriptor non-blocking, so we need to change
	// that here.
	pfd := int(ptmx.Fd())
	if err := syscall.SetNonblock(pfd, true); err != nil {
		return nil, fmt.Errorf("couldn't set ptmx non-blocking: %v", err)
	}

	s := &stmObj{
		gc:        gc,
		ptmx:      ptmx,
		cancelPty: cancel,
		st:        SERVER,
	}

	return s, nil
}

func (s *stmObj) sendPayload(msg *transport.Payload) {
	p, err := proto.Marshal(msg)
	if err != nil {
		// TODO: Log error messages
		return
	}
	_, err = s.gc.Write(p)
	if err != nil {
		// TODO: Log error messages
	}
}

func (s *stmObj) ping() {
	s.sendPayload(s.buildPayload(transport.PayloadType_PING.Enum()))
}

func (s *stmObj) Run() {
	s.wg.Add(1)
	go func() {
		s.handleRemote()
		s.wg.Done()
	}()

	switch s.st {
	case CLIENT:
		s.ping()
		s.wg.Add(2)
		go func() {
			s.handleWinCh()
			s.wg.Done()
		}()
		go func() {
			s.handleInput()
			s.wg.Done()
		}()
	case SERVER:
		// We defer handlePtyOutput until we're pinged by the
		// client, so this is just a placeholder for now.
	}

	s.wg.Wait()
}

func (s *stmObj) Shutdown() {
	s.shutdown = true

	switch s.st {
	case CLIENT:
		s.sendPayload(s.buildPayload(transport.PayloadType_SHUTDOWN.Enum()))
		if err := term.Restore(int(os.Stdin.Fd()), s.os); err != nil {
			// TODO: Log error messages
		}
	}

	go func() {
		s.wg.Add(1)
		s.gc.Close()
		s.wg.Done()
	}()
}

func (s *stmObj) handleWinCh() {
	sig := make(chan os.Signal, 10)
	signal.Notify(sig, syscall.SIGWINCH)

	t := time.NewTicker(100 * time.Millisecond)

	for {
		if s.shutdown {
			return
		}

		select {
		case <-sig:
			w, h, err := term.GetSize(int(os.Stdin.Fd()))
			if err != nil {
				// TODO: Add error logging here
				continue
			}

			sz := transport.Resize_builder{
				Width:  proto.Int32(int32(w)),
				Height: proto.Int32(int32(h)),
			}.Build()
			msg := s.buildPayload(transport.PayloadType_WINDOW_RESIZE.Enum())
			msg.SetSize(sz)

			s.sendPayload(msg)
		case <-t.C:
			// Just a catch to ensure we don't block
			// forever on the WINCH signal and get a
			// chance to terminate cleanly if we're in
			// shutdown mode.
		}
	}
}

func (s *stmObj) handleInput() {
	var inEsc bool

	char := make([]byte, 1)

	msg := s.buildPayload(transport.PayloadType_CLIENT_INPUT.Enum())

	for {
		_, err := os.Stdin.Read(char)
		if err != nil {
			// TODO: Log this?
			continue
		}

		if inEsc {
			switch char[0] {
			case '.':
				s.Shutdown()
				return
			default:
				msg.SetInput(append(msg.GetInput(), char...))
				inEsc = false
			}
		} else {
			msg.SetInput(char)
			switch char[0] {
			case '\x1e':
				inEsc = true
				continue // Don't immediately send this
			}
		}
		s.sendPayload(msg)
	}
}

// buildPayload returns a transport.Payload object populated with
// various basic fields. The actual payload should be added by the
// caller to make the message complete
func (s *stmObj) buildPayload(t *transport.PayloadType) *transport.Payload {
	// TODO: Make id and ack fields useful
	return transport.Payload_builder{
		Sent: tspb.New(time.Now()),
		Id:   proto.Int32(1),
		Ack:  proto.Int32(1),
		Type: t,
	}.Build()
}

func (s *stmObj) handleRemote() {
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

		var msg transport.Payload
		if err = proto.Unmarshal(buf[:n], &msg); err != nil {
			// TODO log this
			continue
		}

		switch msg.GetType() {
		case transport.PayloadType_PING:
			if s.st == SERVER && !s.ptyRunning {
				s.wg.Add(1)
				go func() {
					s.handlePtyOutput()
					s.wg.Done()
				}()
				s.ptyRunning = true
			}
		case transport.PayloadType_SHUTDOWN:
			s.Shutdown()
		case transport.PayloadType_CLIENT_INPUT:
			keys := msg.GetInput()
			if n, err := s.ptmx.Write(keys); err != nil || n != len(keys) {
				// TODO log this
			}
		case transport.PayloadType_WINDOW_RESIZE:
			sz := msg.GetSize()
			pts := &pty.Winsize{
				Rows: uint16(sz.GetHeight()),
				Cols: uint16(sz.GetWidth()),
			}
			pty.Setsize(s.ptmx, pts)
			// Any use of Fd(), including in the InheritSize call above,
			// will set the descriptor non-blocking, so we need to change
			// that here.
			pfd := int(s.ptmx.Fd())
			if err := syscall.SetNonblock(pfd, true); err != nil {
				// TODO log this
			}
		case transport.PayloadType_SERVER_OUTPUT:
			o := msg.GetPtyOutput()
			l := len(o)
			for {
				n, err := os.Stdout.Write(o)
				if err != nil {
					// TODO: Log this
					break
				}
				l -= n

				if l == 0 {
					break
				}
			}
		}
	}
}

func (s *stmObj) handlePtyOutput() {
	defer s.cancelPty()

	for {
		if s.shutdown {
			break
		}

		// if !s.gc.Connected() {
		// 	continue
		// }

		buf := make([]byte, 1024)
		s.ptmx.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := s.ptmx.Read(buf)
		if err != nil {
			// TODO: Log this
			// Handle timeout errors here
			// if e,ok := err.(io.Er) !ok || !e.Timeout() {
			// 	// handle error, it's not a timeout
			// }
			continue
		}

		msg := s.buildPayload(transport.PayloadType_SERVER_OUTPUT.Enum())
		msg.SetPtyOutput(buf[:n])
		s.sendPayload(msg)
	}
}
