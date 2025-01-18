package stm

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	"github.com/bdwalton/gosh/vt"
	"github.com/creack/pty"
	"golang.org/x/term"
	"google.golang.org/protobuf/proto"
	tspb "google.golang.org/protobuf/types/known/timestamppb"
)

const (
	CLIENT = iota
	SERVER
)

const (
	// Like it's 1975 baby!
	DEF_ROWS = 24
	DEF_COLS = 80
)

type stmObj struct {
	gc     *network.GConn
	origSz *term.State // original state of the client pty

	ctx       context.Context
	ptmx      *os.File
	ptyIO     *io.PipeWriter
	cmd       *exec.Cmd
	cancelPty context.CancelFunc

	term *vt.Terminal

	st         uint8
	shutdown   bool
	ptyRunning bool
	wg         sync.WaitGroup
}

func NewClient(gc *network.GConn) (*stmObj, error) {
	orig, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, fmt.Errorf("couldn't make terminal raw: %v", err)
	}

	// On the client end, we will read from the network and ship
	// the diff into the locally running terminal. To do that,
	// we'll tell the terminal we create that it's pty is the pr
	// end of the pipe we'll write into.
	pr, pw := io.Pipe()

	s := &stmObj{
		gc:     gc,
		origSz: orig,
		st:     CLIENT,
		term:   vt.NewTerminal(pr, DEF_ROWS, DEF_COLS),
		ptyIO:  pw,
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
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: DEF_ROWS, Cols: DEF_COLS})
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
		term:      vt.NewTerminal(ptmx, DEF_ROWS, DEF_COLS),
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
		go func() {
			s.wg.Add(1)
			s.handleWinCh()
			s.wg.Done()
		}()

		// Don't put this in a goroutine or count it as we'll
		// just let it get torn down by the runtime.
		go s.handleInput()
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
		if err := term.Restore(int(os.Stdin.Fd()), s.origSz); err != nil {
			slog.Error("couldn't restore terminal mode", "err", err)
		}

		if err := os.Stdin.Close(); err != nil {
			slog.Debug("error closing stdin", "err", err)
		}
	case SERVER:
		s.ptyIO.CloseWithError(io.EOF)
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

	char := make([]byte, 1024)

	for {
		if s.shutdown {
			return
		}

		msg := s.buildPayload(goshpb.PayloadType_CLIENT_INPUT.Enum())
		n, err := os.Stdin.Read(char)
		if err != nil {
			if errors.Is(err, io.EOF) {
				slog.Debug("os.stdin eof. shutting down")
				s.Shutdown()
				return
			}

			slog.Debug("stdin readbyte error", "err", err)
			continue
		}

		if inEsc {
			switch char[0] {
			case '.':
				s.Shutdown()
				return
			default:
				msg.SetData(char[:n])
				inEsc = false
			}
		} else {

			switch char[0] {
			case '\x1e':
				inEsc = true
				continue // Don't immediately send this
			default:
				msg.SetData(char[:n])
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
			if errors.Is(err, io.EOF) {
				slog.Debug("EOF from remote, so shutting down")
				s.Shutdown()
				return
			}
			// TODO Log this if not a timeout
			// slog.Error("error reading remote", "err", err)
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
					s.term.Run()
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
			n, err := s.ptyIO.Write(o)
			if err != nil || n != len(o) {
				slog.Error("couldn't write to stdout", "err", err)
				break
			}
		}
	}
}
