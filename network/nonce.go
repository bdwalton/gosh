package network

import (
	"encoding/binary"
	"log/slog"
	"sync"
)

const (
	NONCE_BYTES = 12
	// With GCM, we can safely use 64-bits of a counter because
	// there is no chance of a collision. We're sending messages
	// in both directions here, so give a very ample 2^62 messages
	// per side. Client and server always distinguish themselves
	// by adding a 0 or a 1 to the nonce value so even as they
	// re-use this counter int, the actual nonce will always be
	// distinct and thus still safe.
	MAX_NONCE_VAL = 1 << 62
)

type nonce struct {
	v   uint64
	mux sync.Mutex
}

func (n *nonce) nextVal() uint64 {
	n.mux.Lock()
	n.v += 1
	ret := n.v
	n.mux.Unlock()

	return ret
}

func (n *nonce) get(dir uint8) []byte {
	nce := n.nextVal()
	if nce == MAX_NONCE_VAL {
		slog.Error("nonce pool exceeded", "nonce", nce)
		panic("nonce pool exceeded")
	}
	b := make([]byte, NONCE_BYTES)
	b[0] = byte(dir)
	binary.LittleEndian.PutUint64(b[4:], uint64(nce))
	return b
}

// extractNonce returns a nonce value and the "direction" of the
// nonce, which should match either CLIENT or SERVER
func extractNonce(b []byte) (uint64, uint8) {
	if len(b) != NONCE_BYTES {
		slog.Error("invalid nonce bytes")
		panic("invalid nonce bytes")
	}
	n := binary.LittleEndian.Uint64(b[4:])
	if n >= MAX_NONCE_VAL {
		slog.Error("nonce pool exceeded", "nonce", n)
		panic("nonce pool exceeded")
	}
	return n, uint8(b[0])
}
