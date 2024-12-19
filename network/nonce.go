package network

import (
	"encoding/binary"
)

type nonce uint64

func (n nonce) toGCMNonce() []byte {
	b := make([]byte, 12)
	binary.LittleEndian.PutUint64(b[4:], uint64(n))
	return b
}

func nonceFromBytes(b []byte) nonce {
	return nonce(binary.LittleEndian.Uint64(b[4:]))
}
