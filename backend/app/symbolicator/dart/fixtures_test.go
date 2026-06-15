package dart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTraceShapeVaries(t *testing.T) {
	for _, dir := range fixtureDirs(t) {
		name := filepath.Base(dir)
		raw := readFile(t, filepath.Join(dir, "trace.txt"))
		trace := ParseTrace(raw)
		if !IsNonSymbolic(raw) {
			t.Errorf("%s: not detected as non-symbolic", name)
		}
		elfBuildID, err := ReadBuildID(readBytes(t, findSymbols(t, dir)))
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%-36s frames=%2d header_virt=%-5v header_build_id=%-6v elf_build_id=%q",
			name, len(trace.Frames), strings.Contains(raw, " virt "),
			trace.BuildID != "", elfBuildID)
	}
}

func fixtureDirs(t *testing.T) []string {
	t.Helper()
	all, _ := filepath.Glob("fixtures/*")
	var dirs []string
	for _, d := range all {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		t.Fatal("no fixtures under fixtures/")
	}
	return dirs
}

func findSymbols(t *testing.T, dir string) string {
	t.Helper()
	for _, pat := range []string{"*.symbols", "*.elf"} {
		if m, _ := filepath.Glob(filepath.Join(dir, pat)); len(m) > 0 {
			return m[0]
		}
	}
	t.Fatalf("no .symbols or .elf file in %s", dir)
	return ""
}

func readFile(t *testing.T, path string) string { return string(readBytes(t, path)) }

func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
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
