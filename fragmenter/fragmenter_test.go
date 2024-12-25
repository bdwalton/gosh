package fragmenter

import (
	"math/rand"
	"slices"
	"testing"
)

func TestCreateFragments(t *testing.T) {
	f1 := New(10)
	f2 := New(2)
	f3 := New(10)

	rbytes := make([]byte, COMPRESS_THRESHOLD+1)
	rand.Read(rbytes)
	cases := []struct {
		f         *fragmenter
		input     []byte
		wantId    uint32
		wantTotal int
		wantComp  bool
		wantFrags [][]byte
	}{
		{f1, []byte{1, 2, 3}, 0, 1, false, [][]byte{[]byte{1, 2, 3}}},
		{f2, []byte{1, 2, 3}, 0, 2, false, [][]byte{[]byte{1, 2}, []byte{3}}},
		{f2, []byte{1, 2, 3}, 1, 2, false, [][]byte{[]byte{1, 2}, []byte{3}}},
		{f2, []byte{1, 2, 3, 4, 5}, 2, 2, false, [][]byte{[]byte{1, 2}, []byte{3, 4}, []byte{5}}},
		{f3, rbytes, 0, 1, true, [][]byte{} /*unused for compress test*/},
	}

	for i, c := range cases {
		// We don't test error cases as we only ever error in
		// gzip which isn't something we should test for here.
		got, _ := c.f.CreateFragments(c.input)
		// Ingore this check for compressed data. TODO: Add
		// round tripping to verify compression.
		if !c.wantComp && len(got) != len(c.wantFrags) {
			t.Errorf("%d: %d frags, wanted %d", i, len(got), len(c.wantFrags))
		}

		if id := got[0].GetId(); id != c.wantId {
			t.Errorf("%d: Got id %d, wanted %d", i, id, c.wantId)
		}

		if comp := got[0].GetCompressed(); comp != c.wantComp {
			t.Errorf("%d: Got compressiong %t, wanted %t", i, comp, c.wantComp)
		}

		if !c.wantComp {
			for j, s := range got {
				if d := s.GetData(); !slices.Equal(d, c.wantFrags[j]) {
					t.Errorf("%d/%d: Bytes: %v; Got %v, wanted %v", i, j, c.input, d, c.wantFrags[j])
				}
			}
		}
	}
}
