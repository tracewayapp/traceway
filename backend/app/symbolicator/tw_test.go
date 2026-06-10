package symbolicator

import (
	"encoding/binary"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap"
)

func TestTWRoundTrip(t *testing.T) {
	for _, tc := range parityCases {
		t.Run(tc.name, func(t *testing.T) {
			mapBytes := mustRead(t, fixture(t, tc.mapPath...))
			bundle := readIfSet(t, tc.minifiedPath)

			original, err := NewResolver(mapBytes, bundle)
			if err != nil {
				t.Fatalf("NewResolver: %v", err)
			}

			reopened, err := OpenTW(original.MarshalTW())
			if err != nil {
				t.Fatalf("OpenTW: %v", err)
			}

			parsed, err := sourcemap.Parse(mapBytes)
			if err != nil {
				t.Fatalf("parsing source map: %v", err)
			}

			mismatches := 0
			for _, token := range parsed.Tokens {
				oFrame, oOk := original.Lookup(token.GenLine, token.GenCol)
				rFrame, rOk := reopened.Lookup(token.GenLine, token.GenCol)
				if oOk != rOk || oFrame != rFrame {
					mismatches++
					if mismatches <= 10 {
						t.Errorf("lookup(%d,%d): original=(%+v,%v) reopened=(%+v,%v)", token.GenLine, token.GenCol, oFrame, oOk, rFrame, rOk)
					}
				}
			}
			if mismatches > 10 {
				t.Errorf("%d total mismatches", mismatches)
			}

			for _, exp := range tc.expectations {
				frame, ok := reopened.Lookup(exp.line, exp.col)
				if exp.wantNone {
					if ok {
						t.Errorf("lookup(%d,%d): got %+v, want no mapping", exp.line, exp.col, frame)
					}
					continue
				}
				if !ok {
					t.Errorf("lookup(%d,%d): got no mapping, want a result", exp.line, exp.col)
					continue
				}
				if exp.file != "" && frame.File != exp.file {
					t.Errorf("lookup(%d,%d) file: got %q, want %q", exp.line, exp.col, frame.File, exp.file)
				}
				if frame.Line != exp.srcLine+1 || frame.Col != exp.srcCol+1 {
					t.Errorf("lookup(%d,%d) pos: got %d:%d, want %d:%d", exp.line, exp.col, frame.Line, frame.Col, exp.srcLine+1, exp.srcCol+1)
				}
				if frame.Fn != exp.fn {
					t.Errorf("lookup(%d,%d) fn: got %q, want %q", exp.line, exp.col, frame.Fn, exp.fn)
				}
			}
		})
	}
}

func TestOpenTWRejectsInvalid(t *testing.T) {
	cases := map[string][]byte{
		"empty":         nil,
		"short":         []byte("TWSM"),
		"bad_magic":     append([]byte("XXXX"), make([]byte, 20)...),
		"bad_version":   append([]byte("TWSM\xff\x00\x00\x00"), make([]byte, 16)...),
		"truncated":     func() []byte { r := &Resolver{}; return r.MarshalTW()[:twHeaderSize-1] }(),
		"token_overrun": append([]byte("TWSM\x01\x00\x00\x00\xff\xff\xff\x00"), make([]byte, 12)...),
	}
	for name, data := range cases {
		if _, err := OpenTW(data); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func buildTW(t *testing.T, tokens [][6]uint32, fileOffsets []uint32, fileBlob []byte, fnOffsets []uint32, fnBlob []byte) []byte {
	t.Helper()
	out := []byte("TWSM")
	out = binary.LittleEndian.AppendUint32(out, 1)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(tokens)))
	out = binary.LittleEndian.AppendUint32(out, uint32(len(fileOffsets)-1))
	out = binary.LittleEndian.AppendUint32(out, uint32(len(fnOffsets)-1))
	out = binary.LittleEndian.AppendUint32(out, 0)
	for _, tok := range tokens {
		for _, v := range tok {
			out = binary.LittleEndian.AppendUint32(out, v)
		}
	}
	for _, off := range fileOffsets {
		out = binary.LittleEndian.AppendUint32(out, off)
	}
	out = append(out, fileBlob...)
	for _, off := range fnOffsets {
		out = binary.LittleEndian.AppendUint32(out, off)
	}
	out = append(out, fnBlob...)
	return out
}

func TestOpenTWRejectsCorruptStringOffsets(t *testing.T) {
	data := buildTW(t, nil, []uint32{0, 1000, 5}, []byte("abcde"), []uint32{0}, nil)
	if _, err := OpenTW(data); err == nil {
		t.Fatal("expected error for non-monotonic string offsets, got nil")
	}
}

func TestOpenTWRejectsOutOfRangeTokenIndexes(t *testing.T) {
	cases := map[string][6]uint32{
		"fileIdx_too_big": {0, 0, 1, 1, 5, 0xFFFFFFFF},
		"fnIdx_too_big":   {0, 0, 1, 1, 0, 7},
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			data := buildTW(t, [][6]uint32{tok}, []uint32{0, 4}, []byte("a.js"), []uint32{0, 2}, []byte("fn"))
			if _, err := OpenTW(data); err == nil {
				t.Fatal("expected error for out-of-range token index, got nil")
			}
		})
	}
}

func TestOpenTWAcceptsNegativeTokenIndexes(t *testing.T) {
	noID := uint32(0xFFFFFFFF)
	data := buildTW(t, [][6]uint32{{0, 0, 1, 1, noID, noID}}, []uint32{0, 4}, []byte("a.js"), []uint32{0, 2}, []byte("fn"))
	r, err := OpenTW(data)
	if err != nil {
		t.Fatalf("OpenTW: %v", err)
	}
	if _, ok := r.Lookup(0, 0); ok {
		t.Fatal("token with fileIdx=-1 should not resolve")
	}
}

func TestMarshalTWEmpty(t *testing.T) {
	r := &Resolver{}
	reopened, err := OpenTW(r.MarshalTW())
	if err != nil {
		t.Fatalf("OpenTW: %v", err)
	}
	if _, ok := reopened.Lookup(0, 0); ok {
		t.Error("empty resolver should not resolve anything")
	}
}
