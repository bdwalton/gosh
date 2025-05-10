package stm

import (
	"errors"
	"io"
	"log/slog"
	"net"
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

	remoteAgent net.Listener
	localAgent  net.Conn
	socketPath  string
	nextAuthID  uint32
	authMux     sync.Mutex
	agentConns  map[uint32]net.Conn

	smux                 sync.Mutex
	remState, localState time.Time
	lastSeenRem          time.Time
	states               map[time.Time]*vt.Terminal
}

func new(remote io.ReadWriter, t *vt.Terminal, st uint8) *stmObj {
	s := &stmObj{
		remote:     remote,
		st:         st,
		term:       t,
		frag:       fragmenter.New(MAX_PACKET_SIZE),
		states:     make(map[time.Time]*vt.Terminal),
		agentConns: make(map[uint32]net.Conn),
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

func NewClient(remote io.ReadWriter, t *vt.Terminal, sock net.Conn) *stmObj {
	s := new(remote, t, CLIENT)
	s.localAgent = sock
	s.socketPath = sock.RemoteAddr().String()
	return s
}

func NewServer(remote io.ReadWriter, t *vt.Terminal, sock net.Listener) *stmObj {
	s := new(remote, t, SERVER)
	s.remoteAgent = sock
	s.socketPath = sock.Addr().String()
	return s
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

func (s *stmObj) shouldSend() bool {
	// Only send if we've seen the remote side within the last
	// minute or we think local state and remote state match or
	// we've never seen them (zero time) which means we should
	// send initial state.
	return s.lastSeenRem.Add(1*time.Minute).After(time.Now()) || s.lastSeenRem.IsZero()
}

func (s *stmObj) Run() {
	// This goroutine is leaked
	go s.fragCleaner()

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

		if s.socketPath != "" {
			s.wg.Add(1)
			go func() {
				s.handleAuthSock()
				s.wg.Done()
			}()
		}

		go func() {
			// If the process in the pty dies, we need to
			// shut down.
			s.term.Wait()
			slog.Debug("terminal Wait() succeeded. shutting down")
			s.Shutdown()
			s.wg.Done()
		}()

		tick := time.NewTicker(10 * time.Millisecond)
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
				if s.remState.Before(ntm) && s.shouldSend() {
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
					}
				}
				s.smux.Unlock()
			}
		}
	}

	s.wg.Wait()
}

func (s *stmObj) Shutdown() {
	// Likely a race, but this is ok anyway, as the worst that
	// happens is errors trying to close closed objects, etc.
	if s.shutdown {
		slog.Debug("shutdown already in progress, ignoring")
		return
	}

	s.shutdown = true

	if s.socketPath != "" {
		switch s.st {
		case SERVER:
			if err := s.remoteAgent.Close(); err != nil {
				slog.Debug("error shutting down remote agent socket", "err", err)
			}
		case CLIENT:
			if err := s.localAgent.Close(); err != nil {
				slog.Debug("error shutting down local agent socket connection", "err", err)
			}
		}
	}

	s.sendPayload(s.buildPayload(goshpb.PayloadType_SHUTDOWN.Enum()))
	slog.Info("sending shutdown to remote peer")

	s.term.Stop()
}

func (s stmObj) handleAuthSock() {
	for {
		if s.shutdown {
			break
		}

		if c, err := s.remoteAgent.Accept(); err == nil {
			go s.handleAuthConn(c)
		} else {
			slog.Debug("error accepting auth sock connection", "err", err)
			break
		}
	}
}

func (s stmObj) handleAuthConn(c net.Conn) {
	s.authMux.Lock()
	id := s.nextAuthID
	s.nextAuthID += 1
	s.authMux.Unlock()
	s.agentConns[id] = c

	for {
		buf := make([]byte, 4096)
		nr, err := c.Read(buf)
		if err != nil {
			slog.Debug("error reading from auth sock connection", "err", err)
			break
		}

		pl := s.buildPayload(goshpb.PayloadType_SSH_AGENT_REQUEST.Enum())
		pl.SetData(buf[0:nr])

		pl.SetAuthid(id)
		s.sendPayload(pl)
	}

	s.authMux.Lock()
	delete(s.agentConns, id)
	c.Close()
	s.authMux.Unlock()
	slog.Debug("client closed connection", "id", id)
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
		at := msg.GetReceived().AsTime()
		slog.Debug("received ack", "time", at)
		rt := msg.GetReceived().AsTime()
		slog.Debug("received ack", "time", rt)
		s.smux.Lock()
		s.remState = rt
		for k := range s.states {
			if k.Before(rt) {
				delete(s.states, k)
				slog.Debug("removing state", "k", k)
			}
		}
		s.smux.Unlock()
	case goshpb.PayloadType_SHUTDOWN:
		slog.Debug("remote initiated shutdown")
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
	case goshpb.PayloadType_SSH_AGENT_RESPONSE:
		id := msg.GetAuthid()
		c, ok := s.agentConns[id]
		if !ok {
			slog.Debug("couldn't lookup auth sock connection", "id", id)
			return
		}
		d := msg.GetData()
		if _, err := c.Write(d); err != nil {
			slog.Debug("error writing ssh agent response to socket", "err", err)
			return
		} else {
			slog.Debug("wrote ssh agent response to local client", "id", id)
		}
	case goshpb.PayloadType_SSH_AGENT_REQUEST:
		d := msg.GetData()
		if _, err := s.localAgent.Write(d); err != nil {
			slog.Debug("error writing to local auth sock", "err", err)
			return
		}
		buf := make([]byte, 4096)
		nr, err := s.localAgent.Read(buf)
		if err != nil {
			slog.Debug("error reading from local auth socket", "err", err)
			return
		}

		pl := s.buildPayload(goshpb.PayloadType_SSH_AGENT_RESPONSE.Enum())
		pl.SetData(buf[0:nr])
		id := msg.GetAuthid()
		pl.SetAuthid(id)
		s.sendPayload(pl)
		slog.Debug("sent ssh agent response", "id", id)
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
	msg.SetReceived(tspb.New(t))
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

		s.lastSeenRem = time.Now()

		if s.frag.Store(&frag) {
			s.consumePayload(frag.GetId())
		}
	}
}
