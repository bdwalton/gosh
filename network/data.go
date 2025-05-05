package network

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	KEY_BYTES = 16
	MTU       = 1280
)

const (
	CLIENT = iota
	SERVER
)

type GConn struct {
	c        *net.UDPConn
	remote   *net.UDPAddr
	shutdown bool
	key      []byte
	aead     cipher.AEAD
	nce      *nonce // Our local nonce generation
	rnce     uint64 // Highest seen remote nonce value
	cType    uint8
}

func initAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher from key: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM AEAD: %v", err)
	}

	return gcm, nil
}

func NewClient(addr, key string) (*GConn, error) {
	a, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("couldn't resolve udp address %q: %v", addr, err)
	}

	c, err := net.DialUDP("udp", nil, a)
	if err != nil {
		return nil, fmt.Errorf("couldn't dial %q: %v", addr, err)
	}

	dkey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("couldn't base64 decode key: %v", err)
	}

	aead, err := initAEAD(dkey)
	if err != nil {
		return nil, fmt.Errorf("couldn't initialize AEAD: %v", err)
	}

	gc := &GConn{
		c:      c,
		key:    dkey,
		aead:   aead,
		cType:  CLIENT,
		remote: a,
		nce:    &nonce{},
	}

	return gc, nil
}

// NewServer takes a port range "n:m" and returns a GConn object
// listening to a port in that range or an error if it can't listen.
func NewServer(ip, prng string) (*GConn, error) {
	var pr [2]uint16
	for i, ns := range strings.SplitN(prng, ":", 2) {
		n, err := strconv.ParseUint(ns, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("couldn't convert port range %q (component: %s): %v", prng, ns, err)
		}
		pr[i] = uint16(n)
	}

	if pr[0] > pr[1] {
		return nil, fmt.Errorf("backwards port range %q - must have lower number first", prng)
	}

	key := make([]byte, KEY_BYTES)
	if n, err := rand.Read(key); err != nil || n != len(key) {
		return nil, fmt.Errorf("failed to generate server key: %v", err)
	}

	aead, err := initAEAD(key)
	if err != nil {
		return nil, fmt.Errorf("couldn't initialize AEAD: %v", err)
	}

	gc := &GConn{
		key:   key,
		aead:  aead,
		cType: SERVER,
		nce:   &nonce{},
	}

	ua := &net.UDPAddr{Port: 0, IP: net.ParseIP(ip)}
	for i := pr[0]; i <= pr[1]; i++ {
		ua.Port = int(i)
		if c, err := net.ListenUDP("udp", ua); err == nil {
			gc.c = c
			return gc, nil
		}
	}

	return nil, fmt.Errorf("couldn't bind a port in the port range %q", prng)
}

func (gc *GConn) Base64Key() string {
	return base64.StdEncoding.EncodeToString(gc.key)
}

func (gc *GConn) LocalPort() int {
	return gc.c.LocalAddr().(*net.UDPAddr).Port
}

func (gc *GConn) Close() error {
	return gc.c.Close()
}

func (gc *GConn) Write(msg []byte) (int, error) {
	// panics if we overflow 32bits of nonce usage
	nce := gc.nce.nextGCMNonce(gc.cType)

	sealed := gc.aead.Seal(nil, nce, msg, nil)

	m := []byte(string(nce) + string(sealed))

	var n int
	var err error

	switch gc.cType {
	case CLIENT:
		n, err = gc.c.Write(m)
	case SERVER:
		n, err = gc.c.WriteToUDP(m, gc.remote)
	}

	if n != len(m) || err != nil {
		return 0, fmt.Errorf("wrote %d of %d bytes: %v", n, len(m), err)
	}

	return n, err
}

func (gc *GConn) Connected() bool {
	return gc.remote != nil
}

func (gc *GConn) Read(extbuf []byte) (int, error) {
	buf := make([]byte, MTU, MTU)

	gc.c.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	n, remote, err := gc.c.ReadFromUDP(buf[:MTU])
	if err != nil {
		if e, ok := err.(net.Error); !ok || !e.Timeout() {
			slog.Error("non-timeout error reading from remote", "err", err)
		}
		return 0, fmt.Errorf("failed to ReadFromUDP(): %v", err)
	} else {
		nce := buf[0:12]
		// Will panic if the nonce exceeds a 32-bit uint
		rn, dir := nonceFromBytes(nce)
		if dir == gc.cType {
			slog.Error("received nonce with our own 'direction'")
			return 0, errors.New("invalid nonce received - bad directionality")
		}

		m := buf[12:n]

		unsealed, err := gc.aead.Open(nil, nce, m, nil)
		if err != nil {
			return 0, fmt.Errorf("failed to unseal data: %v", err)
		}

		// Only update our remote if the nonce sequence has
		// increased from our last known good remote nonce.
		if ors, rs := gc.remote.String(), remote.String(); rs != ors && rn > gc.rnce {
			slog.Info("Updating remote peer", "remote", remote.String())
			gc.remote = remote
		}

		if rn > gc.rnce {
			gc.rnce = rn
		}

		n := copy(extbuf, unsealed)
		if n != len(unsealed) {
			return 0, fmt.Errorf("couldn't copy buffers (%d, %d): %v", n, len(unsealed), err)
		}

		return n, nil
	}
}
