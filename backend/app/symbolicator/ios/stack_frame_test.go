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
