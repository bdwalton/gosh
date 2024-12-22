package network

import (
	"encoding/binary"
	"math"
)

type nonce uint64

func (n *nonce) nextGCMNonce() []byte {
	*n += 1
	if uint64(*n) > uint64(math.MaxUint32) {
		panic("nonce pool exceeded")
	}
	b := make([]byte, 12)
	binary.LittleEndian.PutUint64(b[4:], uint64(*n))
	return b
}

func nonceFromBytes(b []byte) nonce {
	n := binary.LittleEndian.Uint64(b[4:])
	if n > uint64(math.MaxUint32) {
		panic("nonce pool exceeded")
	}
	return nonce(n)
}
