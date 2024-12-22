package stm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/bdwalton/gosh/network"
	"github.com/bdwalton/gosh/protos/transport"
	"github.com/creack/pty"
	"google.golang.org/protobuf/proto"
	"syscall"
)

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
	}

	return s, nil
}

func (s *stmObj) ServerShutdown() {
	s.shutdown = true
	s.wg.Wait()
	s.ptmx.Close()
	s.gc.Close()
}

func (s *stmObj) handlePtyOutput() {
	s.wg.Add(1)
	defer s.wg.Done()

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

		b, err := proto.Marshal(msg)
		if err != nil {
			// TODO: Log this
			continue
		}

		if m, err := s.gc.Write(b); m != n || err != nil {
			// TODO: Log error.
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

		var msg transport.Payload
		if err = proto.Unmarshal(buf[:n], &msg); err != nil {
			// TODO log this
			continue
		}

		switch msg.GetType() {
		case transport.PayloadType_CLIENT_INPUT:
			keys := msg.GetInput()
			if n, err := s.ptmx.Write(keys); err != nil || n != len(keys) {
				// TODO log this
			}
		case transport.PayloadType_WINDOW_RESIZE:
			sz := msg.GetSize()
			pty.Setsize(s.ptmx, &pty.Winsize{Rows: uint16(sz.GetHeight()), Cols: uint16(sz.GetWidth())})
		}
	}
}

func (s *stmObj) RunServer() {
	go s.handleRemoteInput()
	go s.handlePtyOutput()

	for {
		select {
		case <-s.quitCh:
			return
		}
	}
}
