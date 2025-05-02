package fragmenter

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/bdwalton/gosh/protos/goshpb"
	"google.golang.org/protobuf/proto"
)

// For now this is a magic number - we seem to save bytes when
// payload is 100 bytes or more.
const COMPRESS_THRESHOLD = 100

type fragSet struct {
	first, last   time.Time // Used for GC on stale fragments we've stored
	cnt, expected uint32
	frags         []*goshpb.Fragment
}

func newFragSet(size uint32) *fragSet {
	t := time.Now()

	return &fragSet{
		first:    t,
		last:     t,
		expected: size,
		frags:    make([]*goshpb.Fragment, int(size), (size)),
	}
}

func (f *fragSet) add(frag *goshpb.Fragment) {
	f.last = time.Now()
	f.frags[frag.GetThisFrag()] = frag
	f.cnt += 1
}

func (f *fragSet) complete() bool {
	return f.cnt == f.expected
}

type Fragger struct {
	id           uint32 // Increment for each new batch
	size         int    // How much data we can include in each fragment
	idMux, asMux sync.Mutex

	asmbl map[uint32]*fragSet
	last  map[uint32]time.Time
}

func (f *Fragger) getUniqueId() uint32 {
	f.idMux.Lock()
	defer f.idMux.Unlock()
	i := f.id
	f.id += 1
	return i
}

func New(size int) *Fragger {
	return &Fragger{
		size:  size,
		asmbl: make(map[uint32]*fragSet),
		last:  make(map[uint32]time.Time),
	}
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

func decompress(buf []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(buf))
	if err != nil {
		return nil, err
	}

	var obuf bytes.Buffer
	if _, err := io.Copy(&obuf, gz); err != nil {
		return nil, err
	}

	return obuf.Bytes(), nil
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

// Store accepts a fragment and returns a bool indicating whether we
// have all fragments to complete the set for the fragment's id.
func (f *Fragger) Store(frag *goshpb.Fragment) bool {
	id := frag.GetId()

	f.asMux.Lock()
	fset, ok := f.asmbl[id]
	if !ok {
		fset = newFragSet(frag.GetTotalFrags())
		f.asmbl[id] = fset
	}
	f.last[id] = time.Now()
	f.asMux.Unlock()

	fset.add(frag)

	return fset.complete()
}

var unknownFragSet = errors.New("unknown fragement set")
var incompleteFragSet = errors.New("incomplete fragement set")

// Assemble will consume a fragment set and uncompress it as required,
// returning the bytes of the fragments in the expected order. An
// error is returned if the id is unknown or incomplete or if the data
// is compressed and decompression fails.
func (f *Fragger) Assemble(id uint32) ([]byte, error) {
	var err error
	var ret []byte

	f.asMux.Lock()
	defer f.asMux.Unlock()

	fset, ok := f.asmbl[id]
	if !ok {
		return nil, unknownFragSet
	}

	if !fset.complete() {
		return nil, incompleteFragSet
	}

	data := make([][]byte, fset.expected, fset.expected)

	for i := 0; i < int(fset.cnt); i++ {
		data[i] = fset.frags[uint32(i)].GetData()
	}

	d := slices.Concat(data...)
	if fset.frags[0].GetCompressed() {
		ret, err = decompress(d)
		if err != nil {
			return nil, err
		}
	} else {
		ret = d
	}

	delete(f.asmbl, id)
	delete(f.last, id)

	return ret, nil
}

func (f *Fragger) Clean() {
	f.asMux.Lock()
	defer f.asMux.Unlock()

	for id, t := range f.last {
		// Look for fragsets older than 1m. If we haven't seen
		// a full set in that interval, discard the set.
		if t.Add(1 * time.Minute).Before(time.Now()) {
			slog.Debug("expiring old fragset", "id", id, "last", t)
			delete(f.asmbl, id)
			delete(f.last, id)
		}
	}
}
