package network

import (
	"math"
	"slices"
	"sync/atomic"
	"testing"
)

func TestNextGCMNonceDifferentServerClient(t *testing.T) {
	cases := []uint64{
		0,
		uint64(math.MaxUint32 - 1),
	}

	for i, c := range cases {
		var nc atomic.Uint64
		nc.Store(c)
		n1, n2 := &nonce{v: nc}, &nonce{v: nc}
		s1, s2 := n1.get(CLIENT), n2.get(SERVER)
		if slices.Equal(s1, s2) {
			t.Errorf("%d: %v = %v", i, s1, s2)
		}

		if n1.v != n2.v {
			t.Errorf("%d: Underlying nonces not the same: %d and %d", i, n1.v, n2.v)
		}
	}

}

func TestNextGCMNoncePanicClient(t *testing.T) {
	// Defer a function to recover from panic
	defer func() {
		if r := recover(); r != nil {
			// We successfully recovered from panic
			t.Log("Test passed, panic was caught!")
		}
	}()

	var nc atomic.Uint64
	nc.Store(MAX_NONCE_VAL - 1)
	n := &nonce{v: nc}
	// Should panic
	n.get(CLIENT)

	t.Errorf("nextGCMNonce() didn't panic when reaching max 64-bit int: %d", n.v)
}

func TestExtractNoncePanic64BitsClient(t *testing.T) {
	// Defer a function to recover from panic
	defer func() {
		if r := recover(); r != nil {
			// We successfully recovered from panic
			t.Log("Test passed, panic was caught!")
		}
	}()

	// Should panic
	n, dir := extractNonce([]byte{0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40})
	t.Errorf("nonceFromBytes() didn't panic when over 32 bits: %d/%d", n, dir)
}

func TestNextGCMNoncePanicServer(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			// We successfully recovered from panic
			t.Log("Test passed, panic was caught!")
		}
	}()

	var nc atomic.Uint64
	nc.Store(MAX_NONCE_VAL - 1)
	n := nonce{v: nc}
	// Should panic
	n.get(SERVER)
	t.Errorf("nextGCMNonce() didn't panic when reaching max 64-bit int: %d", n.v)
}

func TestExtractNoncePanic64BitsServer(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			// We successfully recovered from panic
			t.Log("Test passed, panic was caught!")
		}
	}()

	// Should panic
	n, dir := extractNonce([]byte{1, 0, 0, 0, 255, 255, 255, 255, 255, 255, 255, 255})
	t.Errorf("nonceFromBytes() didn't panic when over 32 bits: %d/%d", n, dir)
}

func TestExtractNonce(t *testing.T) {
	cases := []struct {
		bytes   []byte
		want    uint64
		wantDir uint8
	}{
		{[]byte{SERVER, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, 0, SERVER},
		{[]byte{CLIENT, 0, 0, 0, 255, 0, 0, 0, 0, 0, 0, 0}, 255, CLIENT},
	}

	for i, c := range cases {
		got, dir := extractNonce(c.bytes)
		if got != c.want || dir != c.wantDir {
			t.Errorf("%d: Got %d/%d, wanted %d/%d", i, got, dir, c.want, c.wantDir)
		}
	}
}

func TestNonceIncrement(t *testing.T) {
	var v1, v2 atomic.Uint64
	v1.Store(math.MaxUint64 - 2)
	v2.Store(255)
	cases := []struct {
		n    *nonce
		want []uint64
	}{
		{&nonce{}, []uint64{1, 2, 3}},
		{&nonce{v: v1}, []uint64{math.MaxUint64 - 1}},
		{&nonce{v: v2}, []uint64{256, 257}},
	}

	for i, c := range cases {
		for j, n := range c.want {
			c.n.get(CLIENT)
			if got := c.n.v.Load(); got != n {
				t.Errorf("%d/%d: Got %d, wanted %d", i, j, c.n, n)
			}
		}
	}
}

func TestExtractNonceWithInvalidInput(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			// We successfully recovered from panic
			t.Log("Test passed, panic was caught!")
		}
	}()

	// Should panic
	extractNonce([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

	t.Errorf("nonceFromBytes() didn't panic when given 11 bytes instead of 12.")
}

func TestExtractNonceWithInvalidInputLong(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			// We successfully recovered from panic
			t.Log("Test passed, panic was caught!")
		}
	}()

	// Should panic
	extractNonce([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13})

	t.Errorf("nonceFromBytes() didn't panic when given 13 bytes instead of 12.")
}
