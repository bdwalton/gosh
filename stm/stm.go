package stm

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bdwalton/gosh/protos/goshpb"
	"github.com/bdwalton/gosh/vt"
	"golang.org/x/term"
	"google.golang.org/protobuf/proto"
	tspb "google.golang.org/protobuf/types/known/timestamppb"
)

const (
	CLIENT = iota
	SERVER
)

type stmObj struct {
	remote io.ReadWriteCloser

	term *vt.Terminal

	st       uint8 // stm type (client or server)
	shutdown bool
	wg       sync.WaitGroup
}

func NewClient(remote io.ReadWriteCloser, t *vt.Terminal) *stmObj {
	return &stmObj{
		remote: remote,
		st:     CLIENT,
		term:   t,
	}
}

func NewServer(remote io.ReadWriteCloser, t *vt.Terminal) *stmObj {
	return &stmObj{
		remote: remote,
		st:     SERVER,
		term:   t,
	}
}

func (s *stmObj) sendPayload(msg *goshpb.Payload) {
	p, err := proto.Marshal(msg)
	if err != nil {
		slog.Error("couldn't marshal message")
		return
	}
	_, err = s.remote.Write(p)
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

	s.wg.Add(1)
	go func() {
		s.term.Run()
		s.wg.Done()
	}()

	switch s.st {
	case CLIENT:
		s.ping()
		s.wg.Add(1)
		go func() {
			s.handleWinCh()
			s.wg.Done()
		}()

		// We don't try to gracefully shut this one down
		// because it'll be blocked on a Read() and using
		// non-blocking is very cpu intensive.
		go s.handleInput()
	case SERVER:
		lastT := s.term.Copy()

		s.wg.Add(1)
		go func() {
			// If the process in the pty dies, we need to
			// shut down.
			s.term.Wait()
			s.Shutdown()
			s.wg.Done()
		}()

		tick := time.NewTicker(100 * time.Millisecond)
		for {
			if s.shutdown {
				break
			}

			select {
			case <-tick.C:
				nowT := s.term.Copy()
				diff := lastT.Diff(nowT)
				if len(diff) > 0 {
					msg := s.buildPayload(goshpb.PayloadType_SERVER_OUTPUT.Enum())
					msg.SetData(diff)
					s.sendPayload(msg)
					lastT = nowT
				}
			}
		}
	}

	s.wg.Wait()
}

func (s *stmObj) Shutdown() {
	s.shutdown = true

	s.sendPayload(s.buildPayload(goshpb.PayloadType_SHUTDOWN.Enum()))
	slog.Info("sending shutdown to remote peer")

	s.term.Stop()

	go func() {
		s.wg.Add(1)
		if err := s.remote.Close(); err != nil {
			slog.Error("Error closing remote", "err", err)
		}
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
			slog.Debug("exiting SIGWINCH watcher")
			return
		}

		select {
		case <-sig:
			cols, rows, err := term.GetSize(int(os.Stdin.Fd()))
			if err != nil {
				slog.Error("couldn't get terminal size", "err", err)
				continue
			}

			sz := goshpb.Resize_builder{
				Cols: proto.Int32(int32(cols)),
				Rows: proto.Int32(int32(rows)),
			}.Build()
			msg := s.buildPayload(goshpb.PayloadType_WINDOW_RESIZE.Enum())
			msg.SetSize(sz)

			s.sendPayload(msg)
			slog.Info("change window size", "rows", sz.GetRows(), "cols", sz.GetCols())
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
				slog.Debug("os.stdin eof, shutting down and exiting input handler")
				s.Shutdown()
				return
			}

			slog.Debug("stdin Read() error", "err", err)
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

		n, err := s.remote.Read(buf)
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
			// TODO: Update a last seen timestamp here
		case goshpb.PayloadType_SHUTDOWN:
			s.Shutdown()
		case goshpb.PayloadType_CLIENT_INPUT:
			keys := msg.GetData()
			if n, err := s.term.Write(keys); err != nil || n != len(keys) {
				slog.Error("couldn't write to terminal", "n", n, "len(keys)", len(keys), "err", err)
			}
		case goshpb.PayloadType_WINDOW_RESIZE:
			sz := msg.GetSize()
			rows, cols := sz.GetRows(), sz.GetCols()
			s.term.Resize(int(rows), int(cols))
		case goshpb.PayloadType_SERVER_OUTPUT:
			o := msg.GetData()
			n, err := s.term.Write(o)
			if err != nil || n != len(o) {
				slog.Error("couldn't write to terminal", "err", err)
				break
			}
			os.Stdout.Write(o)
		}
	}
}
