package ios

import (
	"encoding/binary"
	"testing"
)

const (
	fatArm64UUID  = "3185fc2b69b738638f132803dac76198"
	fatArm64eUUID = "7b1d8aee3ed8339e88b1595d0f815ffe"
)

func embeddedUUID(tw []byte) string {
	if len(tw) < dwHeaderSize {
		return ""
	}
	n := int(binary.LittleEndian.Uint32(tw[40:]))
	if n <= 0 || n > len(tw) {
		return ""
	}
	return string(tw[len(tw)-n:])
}

func TestReadUUIDsThin(t *testing.T) {
	slices, err := ReadUUIDs(readFixture(t, "sample.dsym"))
	if err != nil {
		t.Fatal(err)
	}
	if len(slices) != 1 || slices[0].UUID != fixtureUUID || slices[0].Arch != "arm64" {
		t.Errorf("slices = %+v, want one {%s arm64}", slices, fixtureUUID)
	}
}

func TestReadUUIDsFat(t *testing.T) {
	slices, err := ReadUUIDs(readFixture(t, "sample_fat.dsym"))
	if err != nil {
		t.Fatal(err)
	}
	if len(slices) != 2 {
		t.Fatalf("got %d slices, want 2: %+v", len(slices), slices)
	}
	got := map[string]string{}
	for _, s := range slices {
		got[s.UUID] = s.Arch
	}
	if got[fatArm64UUID] != "arm64" || got[fatArm64eUUID] != "arm64" {
		t.Errorf("fat slices = %+v, want both arch arm64 with the two UUIDs", slices)
	}
}

func TestBuildFlatFatSelectsByUUID(t *testing.T) {
	fat := readFixture(t, "sample_fat.dsym")
	for _, want := range []string{fatArm64UUID, fatArm64eUUID} {
		tw, err := BuildFlat(fat, want, "arm64")
		if err != nil {
			t.Fatalf("BuildFlat(uuid=%s): %v", want, err)
		}
		if got := embeddedUUID(tw); got != want {
			t.Errorf("BuildFlat(uuid=%s, arm64) built the %s slice, UUID selection failed", want, got)
		}
	}
}

func TestBuildFlatArchFallbackOnUUIDMismatch(t *testing.T) {
	tw, err := BuildFlat(readFixture(t, "sample.dsym"), "00000000000000000000000000000000", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	got := LookupFlat(tw, 0x460)
	if len(got) == 0 || got[0].Function != "leaf" {
		t.Fatalf("arch fallback failed to resolve: %+v", got)
	}
}

func TestBuildFlatNoMatchingSlice(t *testing.T) {
	fat := readFixture(t, "sample_fat.dsym")
	if _, err := BuildFlat(fat, "ffffffffffffffffffffffffffffffff", "ppc"); err == nil {
		t.Error("expected an error for an unknown uuid+arch in a multi-slice dSYM")
	}
}
