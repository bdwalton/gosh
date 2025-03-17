package fragmenter

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"log/slog"
	"math"
	"sync"

	"github.com/bdwalton/gosh/protos/goshpb"
	"google.golang.org/protobuf/proto"
)

// For now this is a magic number - we seem to save bytes when
// payload is 100 bytes or more.
const COMPRESS_THRESHOLD = 100

type Fragger struct {
	id    uint32 // Increment for each new batch
	size  int    // How much data we can include in each fragment
	idMux sync.Mutex
}

func (f *Fragger) getUniqueId() uint32 {
	f.idMux.Lock()
	defer f.idMux.Unlock()
	i := f.id
	f.id += 1
	return i
}

func New(size int) *Fragger {
	return &Fragger{size: size}
}

func compress(buf []byte) ([]byte, error) {
	var gbuf bytes.Buffer
	gz := gzip.NewWriter(&gbuf)

	n, err := gz.Write(buf)
	if err != nil || n != len(buf) {
		slog.Error("failed to compress data", "err", err, "n", n)
	}
	gz.Close()
	return gbuf.Bytes(), nil
}

func (f *Fragger) CreateFragments(buf []byte) ([]*goshpb.Fragment, error) {
	fcomp := len(buf) > COMPRESS_THRESHOLD

	var err error
	payload := buf
	if fcomp {
		payload, err = compress(buf)
		if err != nil {
			return nil, fmt.Errorf("couldn't compress payload: %v", err)
		}
	}

	frid := f.getUniqueId()
	total := int(math.Ceil(float64(len(payload)) / float64(f.size)))
	fragments := make([]*goshpb.Fragment, total)
	for i := 0; i < total; i++ {
		fragments[i] = goshpb.Fragment_builder{
			Id:         proto.Uint32(frid),
			ThisFrag:   proto.Uint32(uint32(i)),
			TotalFrags: proto.Uint32(uint32(total)),
			Compressed: proto.Bool(fcomp),
		}.Build()
		s, e := i*f.size, i*f.size+f.size
		if e > len(payload) {
			e = len(payload)
		}
		fragments[i].SetData(payload[s:e])
	}

	return fragments, nil
}
