package network

import (
	"encoding/binary"
	"fmt"
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
		panic(fmt.Sprintf("nonce pool exceeded %d", nce))
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
		panic("invalid nonce bytes passed in")
	}
	n := binary.LittleEndian.Uint64(b[4:])
	if n > uint64(math.MaxUint32) {
		panic(fmt.Sprintf("nonce pool exceeded %d", n))
	}
	return n, uint8(b[0])
}
