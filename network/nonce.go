package network

import (
	"encoding/binary"
	"log/slog"
	"math"
	"sync"
)

type nonce struct {
	n   uint64
	mux sync.Mutex
}

func (n *nonce) nextId() uint64 {
	n.mux.Lock()
	n.n += 1
	ret := n.n
	n.mux.Unlock()

	return ret
}

func (n *nonce) nextGCMNonce(dir uint8) []byte {
	nce := n.nextId()
	if nce > uint64(math.MaxUint32) {
		slog.Error("nonce pool exceeded", "nonce", nce)
		panic("nonce pool exceeded")
	}
	b := make([]byte, 12)
	b[0] = byte(dir)
	binary.LittleEndian.PutUint64(b[4:], uint64(nce))
	return b
}

// nonceFromBytes returns a nonce value and the "direction" of the
// nonce, which should match either CLIENT or SERVER
func nonceFromBytes(b []byte) (uint64, uint8) {
	if len(b) != 12 {
		slog.Error("invalid nonce bytes")
		panic("invalid nonce bytes")
	}
	n := binary.LittleEndian.Uint64(b[4:])
	if n > uint64(math.MaxUint32) {
		slog.Error("nonce pool exceeded", "nonce", n)
		panic("nonce pool exceeded")
	}
	return n, uint8(b[0])
}
