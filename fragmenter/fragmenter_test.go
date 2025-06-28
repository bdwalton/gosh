// Copyright (c) 2025, Ben Walton
// All rights reserved.
package fragmenter

import (
	"crypto/rand"
	"slices"
	"testing"

	"github.com/bdwalton/gosh/protos/goshpb"
	"google.golang.org/protobuf/proto"
)

func TestCreateFragments(t *testing.T) {
	f1 := New(10)
	f2 := New(2)
	f3 := New(10)

	rbytes := make([]byte, COMPRESS_THRESHOLD+1)
	rand.Read(rbytes)
	cases := []struct {
		f         *Fragger
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

func newFragment(id, this, total uint32, data []byte, compressed bool) *goshpb.Fragment {
	f := goshpb.Fragment_builder{
		Id:         proto.Uint32(id),
		ThisFrag:   proto.Uint32(this),
		TotalFrags: proto.Uint32(total),
		Compressed: proto.Bool(compressed),
	}.Build()

	f.SetData(data)

	return f
}

func makeFrag(id, this, total uint32, data []byte) *goshpb.Fragment {
	f := goshpb.Fragment_builder{
		Id:         proto.Uint32(id),
		ThisFrag:   proto.Uint32(this),
		TotalFrags: proto.Uint32(total),
	}.Build()
	f.SetData(data)
	return f
}

func makeFragSet(frags []*goshpb.Fragment) *fragSet {
	tot := frags[0].GetTotalFrags()
	fs := &fragSet{
		expected: tot,
		frags:    make([]*goshpb.Fragment, tot, tot),
	}
	for _, f := range frags {
		fs.add(f)
	}

	return fs
}

func TestStore(t *testing.T) {
	cases := []struct {
		frags []*goshpb.Fragment
		want  bool
	}{
		{[]*goshpb.Fragment{makeFrag(1, 0, 1, []byte("123"))}, true},
		{[]*goshpb.Fragment{makeFrag(2, 0, 2, []byte("123"))}, false},
		{
			[]*goshpb.Fragment{
				makeFrag(3, 0, 3, []byte("123")),
				makeFrag(3, 1, 3, []byte("456")),
			},
			false,
		},
		{
			[]*goshpb.Fragment{
				makeFrag(4, 0, 3, []byte("123")),
				makeFrag(4, 1, 3, []byte("456")),
			},
			false,
		},
		{
			[]*goshpb.Fragment{
				makeFrag(5, 0, 3, []byte("123")),
				makeFrag(5, 1, 3, []byte("456")),
				makeFrag(5, 2, 3, []byte("789")),
			},
			true,
		},
		{
			[]*goshpb.Fragment{
				makeFrag(6, 0, 3, []byte("123")),
				makeFrag(6, 1, 3, []byte("456")),
				makeFrag(7, 2, 3, []byte("789")),
			},
			false,
		},
	}

	for i, c := range cases {
		f := New(3)
		var got bool
		for _, frag := range c.frags {
			got = f.Store(frag)
		}

		if got != c.want {
			t.Errorf("%d: Got %t, wanted %t", i, got, c.want)
		}
	}
}

func TestAssemble(t *testing.T) {
	buf := make([]byte, COMPRESS_THRESHOLD+1)
	rand.Read(buf)
	f := New(50)
	frags, _ := f.CreateFragments(buf)

	cases := []struct {
		id      uint32
		frags   []*goshpb.Fragment
		want    []byte
		wantErr error
	}{
		{
			0,
			[]*goshpb.Fragment{makeFrag(1, 0, 1, []byte("123"))},
			nil,
			unknownFragSet,
		},
		{
			1,
			[]*goshpb.Fragment{makeFrag(1, 0, 2, []byte("123"))},
			nil,
			incompleteFragSet,
		},
		{
			1,
			[]*goshpb.Fragment{makeFrag(1, 0, 1, []byte("123"))},
			[]byte("123"),
			nil,
		},
		{
			1,
			[]*goshpb.Fragment{
				makeFrag(1, 2, 3, []byte("987")),
				makeFrag(1, 0, 3, []byte("123")),
				makeFrag(1, 1, 3, []byte("abc")),
			},
			[]byte("123abc987"),
			nil,
		},
		{
			frags[0].GetId(), frags, buf, nil,
		},
	}

	for i, c := range cases {
		f := New(3)
		f.asmbl[c.frags[0].GetId()] = makeFragSet(c.frags)
		if got, err := f.Assemble(c.id); !slices.Equal(got, c.want) || (err == nil && c.wantErr != nil) || (err != nil && c.wantErr == nil) {
			t.Errorf("%d: Got %q (err: %v), wanted %q (err: %v)", i, string(got), err, string(c.want), c.wantErr)
		}
	}
}
