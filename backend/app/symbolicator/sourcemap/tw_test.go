package sourcemap

import (
	"encoding/binary"
	"testing"
)

const minimalMap = `{"version":3,"sources":["a.js"],"names":["foo"],"mappings":"AAAAA"}`

func TestLookupTWRoundTrip(t *testing.T) {
	tw, err := BuildTW([]byte(minimalMap), nil)
	if err != nil {
		t.Fatalf("BuildTW: %v", err)
	}
	if !ValidTW(tw) {
		t.Fatal("BuildTW produced bytes ValidTW rejects")
	}
	frame, ok := LookupTW(tw, 0, 0)
	if !ok {
		t.Fatal("expected a mapping at (0,0)")
	}
	if frame.File != "a.js" || frame.Line != 1 || frame.Col != 1 {
		t.Errorf("got %+v, want a.js:1:1", frame)
	}
}

func TestLookupTWEmpty(t *testing.T) {
	tw, err := BuildTW([]byte(`{"version":3,"sources":[],"names":[],"mappings":""}`), nil)
	if err != nil {
		t.Fatalf("BuildTW: %v", err)
	}
	if !ValidTW(tw) {
		t.Fatal("empty map should still be a valid artifact")
	}
	if _, ok := LookupTW(tw, 0, 0); ok {
		t.Error("empty resolver should not resolve anything")
	}
}

func TestValidTWRejectsInvalid(t *testing.T) {
	cases := map[string][]byte{
		"empty":         nil,
		"short":         []byte("TWSM"),
		"bad_magic":     append([]byte("XXXX"), make([]byte, 20)...),
		"bad_version":   append([]byte("TWSM\xff\x00\x00\x00"), make([]byte, 16)...),
		"token_overrun": append([]byte("TWSM\x01\x00\x00\x00\xff\xff\xff\x00"), make([]byte, 12)...),
	}
	for name, data := range cases {
		if ValidTW(data) {
			t.Errorf("%s: ValidTW should be false", name)
		}
		if _, ok := LookupTW(data, 0, 0); ok {
			t.Errorf("%s: LookupTW should not resolve invalid data", name)
		}
	}
}

func buildTW(tokens [][6]uint32, files, fns []string) []byte {
	le := binary.LittleEndian
	out := append([]byte{}, twMagic[:]...)
	out = le.AppendUint32(out, twVersion)
	out = le.AppendUint32(out, uint32(len(tokens)))
	out = le.AppendUint32(out, uint32(len(files)))
	out = le.AppendUint32(out, uint32(len(fns)))
	out = le.AppendUint32(out, 0)
	for _, tok := range tokens {
		for _, v := range tok {
			out = le.AppendUint32(out, v)
		}
	}
	out = appendStringTableBytes(out, files)
	out = appendStringTableBytes(out, fns)
	return out
}

func appendStringTableBytes(out []byte, ss []string) []byte {
	offsets, blob := encodeStringTable(ss)
	for _, o := range offsets {
		out = binary.LittleEndian.AppendUint32(out, o)
	}
	return append(out, blob...)
}

func TestLookupTWOutOfRangeIndexSafe(t *testing.T) {
	const noID = uint32(0xFFFFFFFF)
	cases := map[string][6]uint32{
		"fileIdx_too_big": {0, 0, 1, 1, 5, 0},
		"fnIdx_too_big":   {0, 0, 1, 1, 0, 7},
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			data := buildTW([][6]uint32{tok}, []string{"a.js"}, []string{"fn"})
			frame, ok := LookupTW(data, 0, 0)
			if name == "fileIdx_too_big" && ok {
				t.Errorf("out-of-range fileIdx should not resolve, got %+v", frame)
			}

			if name == "fnIdx_too_big" {
				if !ok || frame.File != "a.js" {
					t.Errorf("expected file to resolve, got %+v ok=%v", frame, ok)
				}
				if frame.Fn != "" {
					t.Errorf("out-of-range fnIdx should drop the name, got %q", frame.Fn)
				}
			}
			_ = noID
		})
	}
}

func TestLookupTWNegativeIndex(t *testing.T) {
	const noID = uint32(0xFFFFFFFF)
	data := buildTW([][6]uint32{{0, 0, 1, 1, noID, noID}}, []string{"a.js"}, []string{"fn"})
	if _, ok := LookupTW(data, 0, 0); ok {
		t.Fatal("token with fileIdx=-1 should not resolve")
	}
}
