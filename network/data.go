package network

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	KEY_BYTES = 16
	MTU       = 1024
)

type GConn struct {
	c        *net.UDPConn
	shutdown bool
	quitCh   chan struct{}
	wg       sync.WaitGroup
	key      []byte
	aead     cipher.AEAD
	ln, rn   nonce
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

	dkey, err := base64.StdEncoding.DecodeString(key + "==")
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
		quitCh: make(chan struct{}),
	}

	return gc, nil
}

// NewServer takes a port range "n:m" and returns a GConn object
// listening to a port in that range or an error if it can't listen.
func NewServer(prng string) (*GConn, error) {
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
		key:    key,
		quitCh: make(chan struct{}),
		aead:   aead,
	}

	ua := &net.UDPAddr{Port: 0}
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
	// For compatibility with Mosh, we trim the == here
	return strings.TrimRight(base64.StdEncoding.EncodeToString(gc.key), "==")
}

func (gc *GConn) LocalPort() int {
	return gc.c.LocalAddr().(*net.UDPAddr).Port
}

func (gc *GConn) Shutdown() {
	gc.quitCh <- struct{}{}
	<-gc.quitCh
}

func (gc *GConn) WriteRemote(msg []byte) error {
	nce := gc.ln.toGCMNonce()

	sealed := gc.aead.Seal(nil, nce, msg, nil)

	m := []byte(string(nce) + string(sealed))

	n, err := gc.c.Write(m)
	if n != len(msg) || err != nil {
		return fmt.Errorf("failed to write %d bytes: %v", len(msg), err)
	}

	gc.ln += 1

	return nil
}

func (gc *GConn) ReadRemote() {
	gc.wg.Add(1)
	defer gc.wg.Done()

	buf := make([]byte, MTU, MTU)
	for {
		gc.c.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remote, err := gc.c.ReadFromUDP(buf[:MTU])
		if err != nil {
			if e, ok := err.(net.Error); !ok || !e.Timeout() {
				// handle error, it's not a timeout
			}
		} else {
			nce := buf[0:12]
			m := buf[12:n]

			unsealed, err := gc.aead.Open(nil, nce, m, nil)
			if err != nil {
				continue
			}

			fmt.Println(remote, string(unsealed))
		}

		if gc.shutdown {
			return
		}
	}
}

func (gc *GConn) RunServer() {
	defer close(gc.quitCh)

	go gc.ReadRemote()

	<-gc.quitCh

	gc.shutdown = true

	gc.wg.Wait()

	gc.c.Close()
}
