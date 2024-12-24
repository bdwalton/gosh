package network

import (
	"encoding/binary"
	"fmt"
	"math"
)

type nonce uint64

func (n *nonce) nextGCMNonce(dir uint8) []byte {
	*n += 1
	if uint64(*n) > uint64(math.MaxUint32) {
		panic(fmt.Sprintf("nonce pool exceeded %d", *n))
	}
	b := make([]byte, 12)

	b[0] = byte(dir)
	binary.LittleEndian.PutUint64(b[4:], uint64(*n))
	return b
}

// nonceFromBytes returns a nonce value and the "direction" of the
// nonce, which should match either CLIENT or SERVER
func nonceFromBytes(b []byte) (nonce, uint8) {
	if len(b) != 12 {
		panic("invalid nonce bytes passed in")
	}
	n := binary.LittleEndian.Uint64(b[4:])
	if n > uint64(math.MaxUint32) {
		panic(fmt.Sprintf("nonce pool exceeded %d", n))
	}
	return nonce(n), uint8(b[0])
}
