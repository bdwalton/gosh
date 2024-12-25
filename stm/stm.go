package stm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/bdwalton/gosh/network"
	"github.com/bdwalton/gosh/protos/goshpb"
	"github.com/creack/pty"
	"golang.org/x/term"
	"google.golang.org/protobuf/proto"
	tspb "google.golang.org/protobuf/types/known/timestamppb"
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
	cmd       *exec.Cmd
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
	shell := os.Getenv("SHELL")
	lshell := "-" + filepath.Base(shell)
	cmd := exec.CommandContext(ctx, shell)
	cmd.Args = []string{lshell}
	// TODO: We should probably clean this a bit, but for now,
	// just pass it all through.
	cmd.Env = os.Environ()
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		return nil, fmt.Errorf("couldn't start pty: %v", err)
	}

	// Any use of Fd(), including indirectly via the Setsize call
	// above, will set the descriptor non-blocking, so we need to
	// change that here.
	pfd := int(ptmx.Fd())
	if err := syscall.SetNonblock(pfd, true); err != nil {
		return nil, fmt.Errorf("couldn't set ptmx non-blocking: %v", err)
	}

	s := &stmObj{
		gc:        gc,
		ptmx:      ptmx,
		cancelPty: cancel,
		st:        SERVER,
		cmd:       cmd,
	}

	return s, nil
}

func (s *stmObj) sendPayload(msg *goshpb.Payload) {
	p, err := proto.Marshal(msg)
	if err != nil {
		slog.Error("couldn't marshal message")
		return
	}
	_, err = s.gc.Write(p)
	if err != nil {
		slog.Error("couldn't write to network", "err", err)
	}
}

func (s *stmObj) ping() {
	s.sendPayload(s.buildPayload(goshpb.PayloadType_PING.Enum()))
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
		s.wg.Add(1)
		go func() {
			// If the process in the pty dies, we need to
			// shut down.
			s.cmd.Wait()
			s.Shutdown()
			s.wg.Done()
		}()
	}

	s.wg.Wait()
}

func (s *stmObj) Shutdown() {
	s.shutdown = true

	s.sendPayload(s.buildPayload(goshpb.PayloadType_SHUTDOWN.Enum()))
	slog.Info("sending shutdown to remote peer")

	switch s.st {
	case CLIENT:
		if err := term.Restore(int(os.Stdin.Fd()), s.os); err != nil {
			slog.Error("couldn't restore terminal mode", "err", err)
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

	// won't block as we have buffer, done to ensure we always
	// send the peer our current size at startup.
	sig <- syscall.SIGWINCH

	t := time.NewTicker(100 * time.Millisecond)

	for {
		if s.shutdown {
			return
		}

		select {
		case <-sig:
			w, h, err := term.GetSize(int(os.Stdin.Fd()))
			if err != nil {
				slog.Error("couldn't get terminal size", "err", err)
				continue
			}

			sz := goshpb.Resize_builder{
				Width:  proto.Int32(int32(w)),
				Height: proto.Int32(int32(h)),
			}.Build()
			msg := s.buildPayload(goshpb.PayloadType_WINDOW_RESIZE.Enum())
			msg.SetSize(sz)

			s.sendPayload(msg)
			slog.Info("change window size", "rows", sz.GetHeight(), "cols", sz.GetWidth())
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

	for {
		if s.shutdown {
			return
		}

		msg := s.buildPayload(goshpb.PayloadType_CLIENT_INPUT.Enum())
		_, err := os.Stdin.Read(char)
		if err != nil {
			// This is a constant stream as Read returns
			// EAGAIN.  Figure out a nicer approach
			// here. Can we use syscall.RawConn elegantly
			// for this somehow?
			continue
		}

		if inEsc {
			switch char[0] {
			case '.':
				s.Shutdown()
				return
			default:
				msg.SetData(append(msg.GetData(), char...))
				inEsc = false
			}
		} else {
			msg.SetData(char)
			switch char[0] {
			case '\x1e':
				inEsc = true
				continue // Don't immediately send this
			}
		}
		s.sendPayload(msg)
	}
}

// buildPayload returns a goshpb.Payload object populated with
// various basic fields. The actual payload should be added by the
// caller to make the message complete
func (s *stmObj) buildPayload(t *goshpb.PayloadType) *goshpb.Payload {
	// TODO: Make id and ack fields useful
	return goshpb.Payload_builder{
		Sent: tspb.New(time.Now()),
		Id:   proto.Int32(1),
		Ack:  proto.Int32(1),
		Type: t,
	}.Build()
}

func (s *stmObj) handleRemote() {
	buf := make([]byte, 2048)
	for {
		if s.shutdown {
			return
		}

		n, err := s.gc.Read(buf)
		if err != nil {
			// TODO: Log this
			continue
		}

		var msg goshpb.Payload
		if err = proto.Unmarshal(buf[:n], &msg); err != nil {
			slog.Error("couldn't unmarshal proto", "err", err)
			continue
		}

		switch msg.GetType() {
		case goshpb.PayloadType_PING:
			if s.st == SERVER && !s.ptyRunning {
				s.wg.Add(1)
				go func() {
					s.handlePtyOutput()
					s.wg.Done()
				}()
				s.ptyRunning = true
			}
		case goshpb.PayloadType_SHUTDOWN:
			s.Shutdown()
		case goshpb.PayloadType_CLIENT_INPUT:
			keys := msg.GetData()
			if n, err := s.ptmx.Write(keys); err != nil || n != len(keys) {
				slog.Error("couldn't write to pty", "n", n, "len(keys)", len(keys), "err", err)
			}
		case goshpb.PayloadType_WINDOW_RESIZE:
			sz := msg.GetSize()
			h, w := sz.GetHeight(), sz.GetWidth()
			pts := &pty.Winsize{
				Rows: uint16(h),
				Cols: uint16(w),
			}
			if err := pty.Setsize(s.ptmx, pts); err != nil {
				slog.Error("couldn't set size on pty", "err", err)
			}
			// Any use of Fd(), including in the InheritSize call above,
			// will set the descriptor non-blocking, so we need to change
			// that here.
			pfd := int(s.ptmx.Fd())
			if err := syscall.SetNonblock(pfd, true); err != nil {
				slog.Error("couldn't set pty to nonblocking", "err", err)
			}
			slog.Info("changed window size", "rows", h, "cols", w)
		case goshpb.PayloadType_SERVER_OUTPUT:
			o := msg.GetData()
			n, err := os.Stdout.Write(o)
			if err != nil || n != len(o) {
				slog.Error("couldn't write to stdout", "err", err)
				break
			}
		}
	}
}

func (s *stmObj) handlePtyOutput() {
	defer s.cancelPty()

	buf := make([]byte, 1024)

	for {
		if s.shutdown {
			break
		}

		if err := s.ptmx.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			slog.Error("failed to set ptmx read deadline", "err", err)
		}
		n, err := s.ptmx.Read(buf)
		if err != nil {
			if !errors.Is(err, os.ErrDeadlineExceeded) {
				slog.Error("ptmx read", "n", n, "err", err)
			}
			continue
		}

		msg := s.buildPayload(goshpb.PayloadType_SERVER_OUTPUT.Enum())
		msg.SetData(buf[:n])
		s.sendPayload(msg)
	}
}
