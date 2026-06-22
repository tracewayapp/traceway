package android

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestR8Retrace(t *testing.T) {
	dirs, _ := filepath.Glob(filepath.Join("fixtures", "r8*"))
	if len(dirs) == 0 {
		t.Fatal("no r8 fixtures found")
	}
	for _, dir := range dirs {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			continue
		}
		t.Run(filepath.Base(dir), func(t *testing.T) {
			mapping := mustRead(t, filepath.Join(dir, "mapping.txt"))
			obf := mustRead(t, filepath.Join(dir, "obfuscated.txt"))
			want := nonEmptyLines(mustRead(t, filepath.Join(dir, "expected.txt")))
			got := nonEmptyLines(ParseMapping(mapping).Retrace(obf))
			compareLines(t, got, want)
		})
	}
}

func TestR8FlatMatchesInMemory(t *testing.T) {
	dirs, _ := filepath.Glob(filepath.Join("fixtures", "r8*"))
	if len(dirs) == 0 {
		t.Fatal("no r8 fixtures found")
	}
	for _, dir := range dirs {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			continue
		}
		t.Run(filepath.Base(dir), func(t *testing.T) {
			mapping := mustRead(t, filepath.Join(dir, "mapping.txt"))
			obf := mustRead(t, filepath.Join(dir, "obfuscated.txt"))
			want := nonEmptyLines(mustRead(t, filepath.Join(dir, "expected.txt")))

			flat := BuildR8Flat(mapping)
			if !ValidR8Flat(flat) {
				t.Fatal("BuildR8Flat produced data that fails ValidR8Flat")
			}

			got := nonEmptyLines(RetraceFlat(flat, obf))
			compareLines(t, got, want)

			if RetraceFlat(flat, obf) != ParseMapping(mapping).Retrace(obf) {
				t.Errorf("flat retrace differs from in-memory retrace for %s", filepath.Base(dir))
			}
		})
	}
}

func TestValidR8FlatRejectsGarbage(t *testing.T) {
	if ValidR8Flat([]byte("not a flat mapping")) {
		t.Error("ValidR8Flat accepted garbage")
	}
	if ValidR8Flat(nil) {
		t.Error("ValidR8Flat accepted nil")
	}
}

func TestLooksLikeR8Mapping(t *testing.T) {
	mapping := mustReadBytes(t, filepath.Join("fixtures", "r8", "mapping.txt"))
	if !LooksLikeR8Mapping(mapping) {
		t.Error("mapping.txt not detected as R8 mapping")
	}
	if LooksLikeR8Mapping([]byte("just some text\nwith no mapping\n")) {
		t.Error("plain text wrongly detected as R8 mapping")
	}
}

func TestTraceDetection(t *testing.T) {
	r8 := mustRead(t, filepath.Join("fixtures", "r8", "obfuscated.txt"))
	if !IsR8Trace(r8) || !IsAndroidTrace(r8) {
		t.Error("R8 trace not detected")
	}
}

func compareLines(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("frame count: got %d, want %d", len(got), len(want))
	}
	n := min(len(got), len(want))
	for i := 0; i < n; i++ {
		if got[i] != want[i] {
			t.Errorf("line %d:\n  got:  %s\n  want: %s", i, got[i], want[i])
		}
	}
	for i := n; i < len(got); i++ {
		t.Errorf("extra got line %d: %s", i, got[i])
	}
	for i := n; i < len(want); i++ {
		t.Errorf("missing want line %d: %s", i, want[i])
	}
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	return string(mustReadBytes(t, path))
}

func mustReadBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
