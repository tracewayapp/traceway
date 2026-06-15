package dart

import (
	"path/filepath"
	"testing"
)

func TestFlatMatchesGolden(t *testing.T) {
	for _, dir := range fixtureDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			elf := readBytes(t, findSymbols(t, dir))

			flat, err := BuildFlat(elf)
			if err != nil {
				t.Fatal(err)
			}
			if !ValidFlat(flat) {
				t.Fatal("BuildFlat produced bytes ValidFlat rejects")
			}

			trace := ParseTrace(readFile(t, filepath.Join(dir, "trace.txt")))
			if len(trace.Frames) == 0 {
				t.Fatal("parsed 0 frames from trace.txt")
			}
			var lines []string
			for _, fr := range trace.Frames {
				got := LookupFlat(flat, fr)
				if len(got) == 0 {
					t.Errorf("frame did not resolve: %s", fr.Raw)
					continue
				}
				for _, sf := range got {
					lines = append(lines, sf.Function+" ("+sf.Location()+")")
				}
			}

			golden := nonEmptyLines(readFile(t, filepath.Join(dir, "expected.txt")))
			if len(lines) != len(golden) {
				t.Fatalf("flat produced %d frames, golden has %d", len(lines), len(golden))
			}
			for i := range golden {
				if lines[i] != golden[i] {
					t.Errorf("flat frame %d: got %q want %q", i, lines[i], golden[i])
				}
			}
		})
	}
}

func TestValidFlatRejectsBadData(t *testing.T) {
	for _, b := range [][]byte{nil, []byte("no"), []byte("TWDFxxxx"), make([]byte, 64)} {
		if ValidFlat(b) {
			t.Errorf("expected ValidFlat=false for %d-length input", len(b))
		}
		if got := LookupFlat(b, StackFrame{Section: "isolate", Offset: 1}); got != nil {
			t.Errorf("expected nil lookup for %d-length bad data, got %v", len(b), got)
		}
	}
}
