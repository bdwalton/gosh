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

	smux                 sync.Mutex
	remState, localState time.Time
	states               map[time.Time]*vt.Terminal
}

func new(remote io.ReadWriter, t *vt.Terminal, st uint8) *stmObj {
	s := &stmObj{
		remote: remote,
		st:     st,
		term:   t,
		frag:   fragmenter.New(MAX_PACKET_SIZE),
		states: make(map[time.Time]*vt.Terminal),
	}

	// snag a copy of the newly initialized Terminal. The last
	// change time should be the zero value every time, so that
	// gives us a coordinated time for the client and server to
	// base everything from.
	baseT := s.term.Copy()
	tm := baseT.LastChange()
	s.states[tm] = baseT
	s.remState = tm

	return s
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

func (s *stmObj) fragCleaner() {
	tick := time.NewTicker(1 * time.Second)
	for {
		if s.shutdown {
			break
		}

		select {
		case <-tick.C:
			s.frag.Clean()
		}
	}
}

func (s *stmObj) Run() {
	s.wg.Add(1)
	go func() {
		s.fragCleaner()
		s.wg.Done()
	}()

	s.wg.Add(1)
	go func() {
		s.handleRemote()
		s.wg.Done()
	}()

	switch s.st {
	case CLIENT:
		s.ack(s.localState) // Force first contact with the zero time state
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
		s.wg.Add(2)
		go func() {
			s.term.Run()
			s.wg.Done()
		}()

		go func() {
			// If the process in the pty dies, we need to
			// shut down.
			s.term.Wait()
			s.Shutdown()
			s.wg.Done()
		}()

		tick := time.NewTicker(50 * time.Millisecond)
		for {
			if s.shutdown {
				break
			}

			select {
			case <-tick.C:
				// We always source from the most
				// recent known remote state for now.
				// We can get into p-retransmission
				// later.
				//
				// https://github.com/mobile-shell/mosh/issues/1087#issuecomment-641801909
				nowT := s.term.Copy()
				ntm := nowT.LastChange()
				s.smux.Lock()
				if s.remState.Before(ntm) {
					// If the timestamp in the
					// latest snapshot is newer,
					// we assume there will be a
					// non-zero diff.
					prevT, ok := s.states[s.remState]
					if !ok {
						slog.Error("couldn't retrieve expected state", "remState", s.remState)
					} else {
						diff := prevT.Diff(nowT)
						msg := s.buildPayload(goshpb.PayloadType_SERVER_OUTPUT.Enum())

						msg.SetSource(tspb.New(s.remState))
						msg.SetTarget(tspb.New(ntm))
						msg.SetRetire(tspb.New(s.remState))
						msg.SetData(diff)
						s.sendPayload(msg)
						s.states[ntm] = nowT
						slog.Debug("sending diff", "diff", string(diff), "source", s.remState, "target", ntm)
					}
				}
				s.smux.Unlock()
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
	case goshpb.PayloadType_ACK:
		at := msg.GetAck().AsTime()
		slog.Debug("received ack", "time", at)
		s.smux.Lock()
		s.remState = at
		for k := range s.states {
			if k.Before(at) {
				delete(s.states, k)
				slog.Debug("removing state", "k", k)
			}
		}
		s.smux.Unlock()
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
		s.applyState(&msg)
	}
}

func (s *stmObj) applyState(msg *goshpb.Payload) {
	src := msg.GetSource().AsTime()
	targ := msg.GetTarget().AsTime()

	if targ.Before(s.localState) {
		slog.Debug("received an older state; ignoreing", "lastAck", s.localState, "target", targ, "source", src)
		return
	}

	srcT, ok := s.states[src]
	if !ok {
		slog.Debug("unknown source state", "src", src)
		return
	}

	diff := msg.GetData()
	slog.Debug("received diff", "src", src, "targ", targ, "diff", string(diff))
	// We never want to mutate a stored state because we
	// might need to use it in the future if we recieve
	// additional diffs that build on this one. This can
	// happen because of a dropped ACK or because the
	// server decides to make a diff to an older state
	// instead of a more recent one, etc.
	targT := srcT.Copy()
	n, err := targT.Write(diff)
	if err != nil || n != len(diff) {
		slog.Error("error applying diff", "err", err, "n", n)
		return
	}

	// In case we are applying the server side diff to a
	// state we consider older than our current state, to
	// bring it up to the current server side, we need to
	// diff current with target and write that to stdout
	// for the final display.
	if s.localState.After(src) {
		stdDiff := s.term.Diff(targT)
		slog.Debug("writing additional relative diff to stdout", "stdDiff", string(stdDiff))
		os.Stdout.Write(stdDiff)
	} else {
		os.Stdout.Write(diff)
	}

	s.states[targ] = targT
	s.term.Replace(targT)
	s.ack(targ)

	// we may turn this into a goroutine in the future, so there
	// is a standing periodic cleanup. for now, we just do it
	// inline every time we ack a message.
	for k := range s.states {
		if k.Before(msg.GetRetire().AsTime()) {
			slog.Debug("dropping old state", "k", k)
			delete(s.states, k)
		}
	}
}

func (s *stmObj) ack(t time.Time) {
	s.localState = t
	msg := s.buildPayload(goshpb.PayloadType_ACK.Enum())
	msg.SetAck(tspb.New(t))
	s.sendPayload(msg)
	slog.Debug("sent ack", "time", t)
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
