package ios

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTrace(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("fixtures", "sample", "trace.txt"))
	if err != nil {
		t.Fatal(err)
	}
	tr := ParseTrace(string(raw))

	if tr.OS != "ios" || tr.Arch != "arm64" {
		t.Errorf("os/arch = %q/%q, want ios/arm64", tr.OS, tr.Arch)
	}
	if len(tr.Frames) != 4 {
		t.Fatalf("got %d frames, want 4", len(tr.Frames))
	}
	f0 := tr.Frames[0]
	if f0.UUID != "2dd71042118432be8f92dd4e3d3fe24a" || f0.Offset != 0x460 || f0.Image != "sample" {
		t.Errorf("frame0 = %+v", f0)
	}
	if tr.Frames[3].Image != "UnknownLib" || tr.Frames[3].Offset != 0x1234 {
		t.Errorf("frame3 = %+v", tr.Frames[3])
	}
}

func TestIsHoneycombTrace(t *testing.T) {
	hc := "0   sample   0x100000460   2DD71042-1184-32BE-8F92-DD4E3D3FE24A + 1120"
	if !IsHoneycombTrace(hc) {
		t.Error("honeycomb trace not detected")
	}
	tw := "#00 2dd71042118432be8f92dd4e3d3fe24a 0x460 sample"
	if IsHoneycombTrace(tw) {
		t.Error("traceway trace misdetected as honeycomb")
	}
}

func TestParseHoneycombTrace(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("fixtures", "sample", "trace_honeycomb.txt"))
	if err != nil {
		t.Fatal(err)
	}
	tr := ParseHoneycombTrace(string(raw), "", "")
	if tr.OS != "ios" || tr.Arch != "arm64" {
		t.Errorf("os/arch = %q/%q, want ios/arm64", tr.OS, tr.Arch)
	}
	if len(tr.Frames) != 4 {
		t.Fatalf("got %d frames, want 4", len(tr.Frames))
	}
	f0 := tr.Frames[0]
	if f0.UUID != "2dd71042118432be8f92dd4e3d3fe24a" || f0.Offset != 0x460 || f0.Image != "sample" {
		t.Errorf("frame0 = %+v", f0)
	}
	if tr.Frames[3].Offset != 0x1234 {
		t.Errorf("frame3 offset = %#x, want 0x1234", tr.Frames[3].Offset)
	}
}

func TestParseHoneycombTraceBinaryName(t *testing.T) {
	raw := "os: ios arch: arm64\n0   sample   0x100000460   sample + 1120"
	tr := ParseHoneycombTrace(raw, "2DD71042-1184-32BE-8F92-DD4E3D3FE24A", "sample")
	if len(tr.Frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(tr.Frames))
	}
	if tr.Frames[0].UUID != "2dd71042118432be8f92dd4e3d3fe24a" || tr.Frames[0].Offset != 0x460 {
		t.Errorf("frame = %+v", tr.Frames[0])
	}
}

func TestParseHoneycombTraceCorpusFormat(t *testing.T) {
	raw := "os: ios arch: arm64\n" +
		"0   sample   0x0000000100000460   2DD71042-1184-32BE-8F92-DD4E3D3FE24A + 1120\n" +
		"1   sample   0x0000000100000494   2dd71042-1184-32be-8f92-dd4e3d3fe24a + 1172"
	tr := ParseHoneycombTrace(raw, "", "")
	if len(tr.Frames) != 2 {
		t.Fatalf("got %d frames, want 2", len(tr.Frames))
	}
	if tr.Frames[0].UUID != "2dd71042118432be8f92dd4e3d3fe24a" || tr.Frames[0].Offset != 0x460 || tr.Frames[0].Image != "sample" {
		t.Errorf("frame0 = %+v", tr.Frames[0])
	}
	if tr.Frames[1].UUID != "2dd71042118432be8f92dd4e3d3fe24a" || tr.Frames[1].Offset != 0x494 {
		t.Errorf("frame1 = %+v", tr.Frames[1])
	}
}

func TestParseHoneycombTraceSpacedBinary(t *testing.T) {
	raw := "12   Chateaux Bufeaux   0x000000010000abcd   6A8CB813-45F6-3652-AD33-778FD1EAB196 + 100436"
	tr := ParseHoneycombTrace(raw, "", "")
	if len(tr.Frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(tr.Frames))
	}
	if tr.Frames[0].Image != "Chateaux Bufeaux" || tr.Frames[0].UUID != "6a8cb81345f63652ad33778fd1eab196" || tr.Frames[0].Offset != 100436 {
		t.Errorf("frame = %+v", tr.Frames[0])
	}
}

func TestIsIOSTrace(t *testing.T) {
	if !IsIOSTrace("os: ios arch: arm64\n#00 2dd71042118432be8f92dd4e3d3fe24a 0x460 sample") {
		t.Error("valid iOS trace not detected")
	}
	dart := "#00 abs 00000001131eca6b _kDartIsolateSnapshotInstructions+0x141e6b"
	if IsIOSTrace(dart) {
		t.Error("dart trace misdetected as iOS")
	}
	if IsIOSTrace("just some text\nat foo (bar.js:1:2)") {
		t.Error("js/plain text misdetected as iOS")
	}
}
