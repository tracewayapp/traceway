package dart

import (
	"os"
	"path/filepath"
	"testing"
)

const benchDir = "fixtures/flutter-macos-arm64-dart3.10.1"

func BenchmarkBuildFlat(b *testing.B) {
	elfBytes, err := readFileBytes(filepath.Join(benchDir, "app.darwin-arm64.symbols"))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := BuildFlat(elfBytes); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLookupFlatTrace(b *testing.B) {
	elfBytes, err := readFileBytes(filepath.Join(benchDir, "app.darwin-arm64.symbols"))
	if err != nil {
		b.Fatal(err)
	}
	flat, err := BuildFlat(elfBytes)
	if err != nil {
		b.Fatal(err)
	}
	traceText, err := readFileBytes(filepath.Join(benchDir, "trace.txt"))
	if err != nil {
		b.Fatal(err)
	}
	frames := ParseTrace(string(traceText)).Frames

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, f := range frames {
			_ = LookupFlat(flat, f)
		}
	}
}

func readFileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}
