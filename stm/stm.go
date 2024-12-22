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
	"golang.org/x/term"
	"google.golang.org/protobuf/proto"
	tspb "google.golang.org/protobuf/types/known/timestamppb"
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
		gc:     gc,
		os:     os,
		quitCh: make(chan struct{}),
	}

	return s, nil
}

func (s *stmObj) RunClient() {
	go s.handleWinCh()
	go s.handleInput()
	go s.handleRemotePty()

	for {
		select {
		case <-s.quitCh:
			return
		}
	}
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

			sz := transport.Resize_builder{
				Width:  proto.Int32(int32(w)),
				Height: proto.Int32(int32(h)),
			}.Build()
			msg := s.buildPayload(transport.PayloadType_WINDOW_RESIZE.Enum())
			msg.SetSize(sz)

			p, err := proto.Marshal(msg)
			if err != nil {
				// TODO: Log error messages
				continue
			}
			_, err = s.gc.Write(p)
			if err != nil {
				// TODO: Log error messages
			}
		case <-s.quitCh:
			return
		}
	}
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
				s.clientShutdown()
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

func (s *stmObj) handleRemotePty() {
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
			fmt.Println(err)
			continue
		}

		if msg.HasPtyOutput() {
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
