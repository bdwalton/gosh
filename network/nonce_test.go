package network

import (
	"math"
	"slices"
	"testing"
)

func TestNextGCMNonceDifferentServerClient(t *testing.T) {
	cases := []uint64{
		0,
		uint64(math.MaxUint32 - 1),
	}

	for i, c := range cases {
		n1, n2 := &nonce{n: c}, &nonce{n: c}
		s1, s2 := n1.nextGCMNonce(CLIENT), n2.nextGCMNonce(SERVER)
		if slices.Equal(s1, s2) {
			t.Errorf("%d: %v = %v", i, s1, s2)
		}

		if n1.n != n2.n {
			t.Errorf("%d: Underlying nonces not the same: %d and %d", i, n1.n, n2.n)
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

	n := &nonce{n: uint64(math.MaxUint32)}
	// Should panic
	n.nextGCMNonce(CLIENT)

	t.Errorf("nextGCMNonce() didn't panic when rolling over 32 bits: %d", n.n)
}

func TestNonceFromBytesPanic32BitsClient(t *testing.T) {
	// Defer a function to recover from panic
	defer func() {
		if r := recover(); r != nil {
			// We successfully recovered from panic
			t.Log("Test passed, panic was caught!")
		}
	}()

	// Should panic
	n, dir := nonceFromBytes([]byte{0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0})

	t.Errorf("nonceFromBytes() didn't panic when over 32 bits: %d/%d", n, dir)
}

func TestNextGCMNoncePanicServer(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			// We successfully recovered from panic
			t.Log("Test passed, panic was caught!")
		}
	}()

	n := nonce{n: uint64(math.MaxUint32)}
	// Should panic
	n.nextGCMNonce(SERVER)

	t.Errorf("nextGCMNonce() didn't panic when rolling over 32 bits: %d", n.n)
}

func TestNonceFromBytesPanic32BitsServer(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			// We successfully recovered from panic
			t.Log("Test passed, panic was caught!")
		}
	}()

	// Should panic
	n, dir := nonceFromBytes([]byte{1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0})

	// If the panic was not caught, the test will fail
	t.Errorf("nonceFromBytes() didn't panic when over 32 bits: %d/%d", n, dir)
}

func TestNonceFromBytes(t *testing.T) {
	cases := []struct {
		bytes   []byte
		want    uint64
		wantDir uint8
	}{
		{[]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, 0, SERVER},
		{[]byte{0, 0, 0, 0, 255, 0, 0, 0, 0, 0, 0, 0}, 255, CLIENT},
	}

	for i, c := range cases {
		got, dir := nonceFromBytes(c.bytes)
		if got != c.want || dir != c.wantDir {
			t.Errorf("%d: Got %d/%d, wanted %d/%d", i, got, dir, c.want, c.wantDir)
		}
	}
}

func TestNonceIncrement(t *testing.T) {
	cases := []struct {
		n    *nonce
		want []uint64
	}{
		{&nonce{}, []uint64{1, 2, 3}},
		{&nonce{n: uint64(math.MaxUint32) - 1}, []uint64{uint64(math.MaxUint32)}},
		{&nonce{n: 255}, []uint64{256, 257}},
	}

	for i, c := range cases {
		for j, n := range c.want {
			c.n.nextGCMNonce(CLIENT)
			if c.n.n != n {
				t.Errorf("%d/%d: Got %d, wanted %d", i, j, c.n, n)
			}
		}
	}
}

func TestNonceFromBytesWithInvalidInputShort(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			// We successfully recovered from panic
			t.Log("Test passed, panic was caught!")
		}
	}()

	// Should panic
	nonceFromBytes([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

	t.Errorf("nonceFromBytes() didn't panic when given 11 bytes instead of 12.")
}

func TestNonceFromBytesWithInvalidInputLong(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			// We successfully recovered from panic
			t.Log("Test passed, panic was caught!")
		}
	}()

	// Should panic
	nonceFromBytes([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13})

	t.Errorf("nonceFromBytes() didn't panic when given 13 bytes instead of 12.")
}
