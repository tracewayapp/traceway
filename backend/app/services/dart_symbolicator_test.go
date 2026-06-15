package services

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/storage"

	"github.com/google/uuid"
)

const dartFixtureDir = "../symbolicator/dart/fixtures/flutter-macos-arm64-dart3.10.1"

const fixtureBuildID = "fe664295997135e7b67b648ba66ca9eb"

type dartMemStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newDartMemStore() *dartMemStore { return &dartMemStore{data: map[string][]byte{}} }

func (s *dartMemStore) Write(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = data
	return nil
}

func (s *dartMemStore) Read(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.data[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return d, nil
}

func (s *dartMemStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func readDartFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dartFixtureDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

var frameLineRe = regexp.MustCompile(`^#\d+\s+(.*)$`)

func resolvedFrameLines(out string) []string {
	var lines []string
	for _, ln := range strings.Split(out, "\n") {
		if m := frameLineRe.FindStringSubmatch(ln); m != nil {
			lines = append(lines, strings.TrimSpace(m[1]))
		}
	}
	return lines
}

func expectedLines(b []byte) []string {
	var out []string
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) != "" {
			out = append(out, strings.TrimSpace(ln))
		}
	}
	return out
}

func TestResolveDartStackTraceArchFallback(t *testing.T) {
	useMemCache(t)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	store := newDartMemStore()
	storage.Store = store

	projectId := uuid.New()
	symbols := readDartFixture(t, "app.darwin-arm64.symbols")

	store.data[DartSymbolsKey(projectId, fixtureBuildID, "arm64")] = symbols

	raw := string(readDartFixture(t, "trace.txt"))
	out := ResolveDartStackTrace(context.Background(), projectId, raw)

	got := resolvedFrameLines(out)
	want := expectedLines(readDartFixture(t, "expected.txt"))
	if len(got) != len(want) {
		t.Fatalf("frame count: got %d want %d\nout:\n%s", len(got), len(want), out)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("frame %d: got %q want %q", i, got[i], want[i])
		}
	}
	if !strings.Contains(out, "chargeCard (") || !strings.Contains(out, "main.dart:20:3") {
		t.Errorf("missing chargeCard frame in:\n%s", out)
	}
}

func TestResolveDartStackTraceNoSymbolsStable(t *testing.T) {
	useMemCache(t)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	storage.Store = newDartMemStore()

	raw := string(readDartFixture(t, "trace.txt"))

	rawShifted := shiftAbsAddresses(raw)
	if rawShifted == raw {
		t.Fatal("test setup: abs addresses were not shifted")
	}

	pid := uuid.New()
	out1 := ResolveDartStackTrace(context.Background(), pid, raw)
	out2 := ResolveDartStackTrace(context.Background(), uuid.New(), rawShifted)

	if out1 != out2 {
		t.Errorf("same crash, different abs -> different output:\n--- out1 ---\n%s\n--- out2 ---\n%s", out1, out2)
	}
	if strings.Contains(out1, "build_id:") || strings.Contains(out1, "abs ") {
		t.Errorf("volatile header/abs leaked into normalized output:\n%s", out1)
	}
	if !strings.Contains(out1, "_kDartIsolateSnapshotInstructions+") {
		t.Errorf("expected stable offset frames in:\n%s", out1)
	}
}

func shiftAbsAddresses(raw string) string {
	re := regexp.MustCompile(`abs [0-9a-fA-F]+`)
	return re.ReplaceAllString(raw, "abs 00000007abcdef00")
}

func TestDartSymbolsColdLoadsFromFlatArtifact(t *testing.T) {
	useMemCache(t)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	store := newDartMemStore()
	storage.Store = store

	projectId := uuid.New()
	archKey := DartSymbolsKey(projectId, fixtureBuildID, "arm64")
	symbols := readDartFixture(t, "app.darwin-arm64.symbols")
	store.data[archKey] = symbols
	raw := string(readDartFixture(t, "trace.txt"))

	if out := ResolveDartStackTrace(context.Background(), projectId, raw); !strings.Contains(out, "chargeCard (") {
		t.Fatalf("first resolve did not symbolicate:\n%s", out)
	}
	twKey := dartFlatKey(archKey)
	store.mu.Lock()
	flat, hasFlat := store.data[twKey]
	store.mu.Unlock()
	if !hasFlat {
		t.Fatalf("expected a flat artifact at %s", twKey)
	}
	t.Logf("flat artifact %d bytes vs symbols %d bytes", len(flat), len(symbols))

	sharedCache = newSymbolicatorCache()
	store.mu.Lock()
	delete(store.data, archKey)
	store.mu.Unlock()

	out := ResolveDartStackTrace(context.Background(), projectId, raw)
	if !strings.Contains(out, "chargeCard (") || !strings.Contains(out, "main.dart:20:3") {
		t.Errorf("cold load from .tw failed:\n%s", out)
	}
}

func TestDartSymbolsLocalDiskTier(t *testing.T) {
	prev := storage.Store
	defer func() { storage.Store = prev }()
	store := newDartMemStore()
	storage.Store = store

	dir := t.TempDir()
	useDiskCache(t, dir, 64<<20)

	projectId := uuid.New()
	archKey := DartSymbolsKey(projectId, fixtureBuildID, "arm64")
	store.data[archKey] = readDartFixture(t, "app.darwin-arm64.symbols")
	raw := string(readDartFixture(t, "trace.txt"))

	if out := ResolveDartStackTrace(context.Background(), projectId, raw); !strings.Contains(out, "chargeCard (") {
		t.Fatalf("first resolve failed:\n%s", out)
	}

	useDiskCache(t, dir, 64<<20)

	store.mu.Lock()
	delete(store.data, archKey)
	delete(store.data, dartFlatKey(archKey))
	store.mu.Unlock()

	out := ResolveDartStackTrace(context.Background(), projectId, raw)
	if !strings.Contains(out, "chargeCard (") || !strings.Contains(out, "main.dart:20:3") {
		t.Errorf("local disk tier did not serve the artifact:\n%s", out)
	}
}

func TestDartSymbolsKeyMatchesAcrossForms(t *testing.T) {
	pid := uuid.New()
	machoUUID := "FE664295-9971-35E7-B67B-648BA66CA9EB"
	headerID := "fe664295997135e7b67b648ba66ca9eb"
	if DartSymbolsKey(pid, machoUUID, "arm64") != DartSymbolsKey(pid, headerID, "arm64") {
		t.Fatal("Mach-O UUID and trace-header id must produce the same key")
	}
	if DartSymbolsKey(pid, headerID, "x86_64") != DartSymbolsKey(pid, headerID, "x64") {
		t.Error("arch synonyms should collapse to the same key")
	}
}

func TestResolveDartStackTraceInvalidationParity(t *testing.T) {
	useMemCache(t)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	store := newDartMemStore()
	storage.Store = store

	projectId := uuid.New()
	raw := string(readDartFixture(t, "trace.txt"))

	if out := ResolveDartStackTrace(context.Background(), projectId, raw); strings.Contains(out, "chargeCard (") {
		t.Fatal("did not expect symbolication before upload")
	}

	archKey := DartSymbolsKey(projectId, fixtureBuildID, "arm64")
	store.data[archKey] = readDartFixture(t, "app.darwin-arm64.symbols")
	InvalidateDartSymbols(archKey)

	out := ResolveDartStackTrace(context.Background(), projectId, raw)
	if !strings.Contains(out, "chargeCard (") {
		t.Errorf("symbols not picked up after invalidation:\n%s", out)
	}
}
