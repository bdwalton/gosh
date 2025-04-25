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

	"github.com/bdwalton/gosh/fragmenter"
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

const (
	// 1280 is as much data as we want to send, so we allow room
	// for some overheads around packet size, proto message data,
	// etc. This is conservative for now.
	//
	// TODO: Make this more
	// dynamic and based somehow on the underlying remote
	// capabilities.
	MAX_PACKET_SIZE = 1100
)

type stmObj struct {
	remote io.ReadWriter
	term   *vt.Terminal
	frag   *fragmenter.Fragger

	st       uint8 // stm type (client or server)
	shutdown bool
	wg       sync.WaitGroup
}

func new(remote io.ReadWriter, t *vt.Terminal, st uint8) *stmObj {
	return &stmObj{
		remote: remote,
		st:     st,
		term:   t,
		frag:   fragmenter.New(MAX_PACKET_SIZE),
	}
}

func NewClient(remote io.ReadWriter, t *vt.Terminal) *stmObj {
	return new(remote, t, CLIENT)
}

func NewServer(remote io.ReadWriter, t *vt.Terminal) *stmObj {
	return new(remote, t, SERVER)
}

func (s *stmObj) sendPayload(msg *goshpb.Payload) {
	p, err := proto.Marshal(msg)
	if err != nil {
		slog.Error("couldn't marshal message", "err", err)
		return
	}

	frags, err := s.frag.CreateFragments(p)
	if err != nil {
		slog.Error("couldn't fragment payload", "err", err)
		return
	}

	for i, f := range frags {
		pf, err := proto.Marshal(f)
		if err != nil {
			slog.Error("couldn't marshal fragment", "i", i, "err", err)
			return
		}

		if n, err := s.remote.Write(pf); err != nil || n < len(pf) {
			slog.Error("failed or parial write to remote", "n", n, "err", err)
		}
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
					slog.Debug("sending diff", "diff", string(diff))
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
				s.Shutdown()
				return
			}

			slog.Error("stdin Read() error", "err", err)
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

func (s *stmObj) consumePayload(id uint32) {
	buf, err := s.frag.Assemble(id)
	if err != nil {
		slog.Error("couldn't assemble fragmented payload", "err", err)
		return
	}

	var msg goshpb.Payload
	if err = proto.Unmarshal(buf, &msg); err != nil {
		slog.Error("couldn't unmarshal proto", "err", err)
		return
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

		var frag goshpb.Fragment
		if err = proto.Unmarshal(buf[:n], &frag); err != nil {
			slog.Error("couldn't unmarshal fragment proto", "err", err)
			continue
		}

		if s.frag.Store(&frag) {
			s.consumePayload(frag.GetId())
		}
	}
}
