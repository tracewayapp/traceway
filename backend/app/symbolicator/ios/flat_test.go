package ios

import (
	"os"
	"path/filepath"
	"testing"
)

const fixtureUUID = "2dd71042118432be8f92dd4e3d3fe24a"

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("fixtures", "sample", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func buildFixtureTW(t *testing.T) []byte {
	t.Helper()
	tw, err := BuildFlat(readFixture(t, "sample.dsym"), fixtureUUID, "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if !ValidFlat(tw) {
		t.Fatal("BuildFlat produced bytes ValidFlat rejects")
	}
	return tw
}

func TestLookupKnownOffsets(t *testing.T) {
	tw := buildFixtureTW(t)
	cases := []struct {
		off uint64
		fn  string
		loc string
	}{
		{0x460, "leaf", "sample.c:7:18"},
		{0x478, "compute", "sample.c:14"},
		{0x494, "main", "sample.c:18"},
	}
	for _, c := range cases {
		got := LookupFlat(tw, c.off)
		if len(got) == 0 {
			t.Errorf("offset 0x%x did not resolve", c.off)
			continue
		}
		inner := got[0]
		if inner.Function != c.fn || inner.Location() != c.loc {
			t.Errorf("offset 0x%x: got %s (%s), want %s (%s)", c.off, inner.Function, inner.Location(), c.fn, c.loc)
		}
	}
}

func TestInlineChain(t *testing.T) {
	got := LookupFlat(buildFixtureTW(t), 0x47c)
	if len(got) < 2 {
		t.Fatalf("offset 0x47c resolved %d frames, want >= 2 (inline): %+v", len(got), got)
	}
	if got[0].Function != "inlined_helper" {
		t.Errorf("innermost = %q, want inlined_helper", got[0].Function)
	}
	if got[len(got)-1].Function != "compute" {
		t.Errorf("outermost = %q, want compute", got[len(got)-1].Function)
	}
}

func TestBuildFlatRejectsInvalid(t *testing.T) {
	for _, b := range [][]byte{nil, []byte("no"), []byte("not a mach-o at all"), make([]byte, 64)} {
		if _, err := ReadUUIDs(b); err == nil {
			t.Errorf("ReadUUIDs accepted %d-length non-Mach-O", len(b))
		}
		if _, err := BuildFlat(b, "", "arm64"); err == nil {
			t.Errorf("BuildFlat accepted %d-length non-Mach-O", len(b))
		}
	}
}

func TestValidFlatRejectsBadData(t *testing.T) {
	for _, b := range [][]byte{nil, []byte("no"), []byte("TWIOxxxx"), []byte("TWDF\x03\x00\x00\x00"), make([]byte, 64)} {
		if ValidFlat(b) {
			t.Errorf("ValidFlat=true for %d-length bad input", len(b))
		}
		if got := LookupFlat(b, 1); got != nil {
			t.Errorf("LookupFlat returned %v for bad data", got)
		}
	}
}

func TestIsMachO(t *testing.T) {
	accept := [][]byte{
		{0xfe, 0xed, 0xfa, 0xce},
		{0xfe, 0xed, 0xfa, 0xcf},
		{0xce, 0xfa, 0xed, 0xfe},
		{0xcf, 0xfa, 0xed, 0xfe},
		{0xca, 0xfe, 0xba, 0xbe},
		{0xbe, 0xba, 0xfe, 0xca},
	}
	for _, b := range accept {
		if !IsMachO(b) {
			t.Errorf("IsMachO rejected valid magic %x", b)
		}
	}
	reject := [][]byte{
		nil, {0xfe}, {0x7f, 'E', 'L', 'F'},
		{0xfd, 0xed, 0xfa, 0xce},
		{0xfe, 0xed, 0xfa, 0xc0},
		{'T', 'W', 'I', 'O'},
	}
	for _, b := range reject {
		if IsMachO(b) {
			t.Errorf("IsMachO accepted invalid magic %x", b)
		}
	}
}
