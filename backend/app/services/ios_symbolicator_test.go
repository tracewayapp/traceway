package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/ios"

	"github.com/google/uuid"
)

const iosFixtureDir = "../symbolicator/ios/fixtures/sample"
const iosFixtureUUID = "2dd71042118432be8f92dd4e3d3fe24a"

func readIOSFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(iosFixtureDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestResolveIOSStackTrace(t *testing.T) {
	useMemCache(t)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	store := newDartMemStore()
	storage.Store = store

	projectId := uuid.New()
	store.data[IOSSymbolsKey(projectId, iosFixtureUUID)] = readIOSFixture(t, "sample.dsym")

	raw := string(readIOSFixture(t, "trace.txt"))
	out := ResolveIOSStackTrace(context.Background(), projectId, raw)

	got := resolvedFrameLines(out)
	want := expectedLines(readIOSFixture(t, "expected.txt"))
	if len(got) != len(want) {
		t.Fatalf("frame count: got %d want %d\nout:\n%s", len(got), len(want), out)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("frame %d: got %q want %q", i, got[i], want[i])
		}
	}
	if !strings.HasPrefix(out, "SampleError: boom") {
		t.Errorf("error preamble not preserved:\n%s", out)
	}
}

func TestResolveIOSStackTraceNoSymbols(t *testing.T) {
	useMemCache(t)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	storage.Store = newDartMemStore()

	raw := string(readIOSFixture(t, "trace.txt"))
	out := ResolveIOSStackTrace(context.Background(), uuid.New(), raw)

	if strings.Contains(out, "leaf (") {
		t.Errorf("did not expect symbolication without uploaded symbols:\n%s", out)
	}
	if !strings.Contains(out, "sample+0x460") {
		t.Errorf("unresolved frame should fall back to image+offset:\n%s", out)
	}
}

func TestResolveIOSStackTraceInvalidationParity(t *testing.T) {
	useMemCache(t)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	store := newDartMemStore()
	storage.Store = store

	projectId := uuid.New()
	raw := string(readIOSFixture(t, "trace.txt"))

	if out := ResolveIOSStackTrace(context.Background(), projectId, raw); strings.Contains(out, "leaf (") {
		t.Fatal("did not expect symbolication before upload")
	}

	key := IOSSymbolsKey(projectId, iosFixtureUUID)
	store.data[key] = readIOSFixture(t, "sample.dsym")
	InvalidateIOSSymbols(key)

	out := ResolveIOSStackTrace(context.Background(), projectId, raw)
	if !strings.Contains(out, "leaf (") {
		t.Errorf("symbols not picked up after invalidation:\n%s", out)
	}
}

func TestIOSSymbolsColdBuildsAndCachesFlat(t *testing.T) {
	useMemCache(t)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	store := newDartMemStore()
	storage.Store = store

	projectId := uuid.New()
	dsymKey := IOSSymbolsKey(projectId, iosFixtureUUID)
	store.data[dsymKey] = readIOSFixture(t, "sample.dsym")
	raw := string(readIOSFixture(t, "trace.txt"))

	if out := ResolveIOSStackTrace(context.Background(), projectId, raw); !strings.Contains(out, "leaf (") {
		t.Fatalf("first resolve did not symbolicate:\n%s", out)
	}

	twKey := iosFlatKey(dsymKey)
	store.mu.Lock()
	flat, hasFlat := store.data[twKey]
	store.mu.Unlock()
	if !hasFlat {
		t.Fatalf("expected a built .tw artifact at %s", twKey)
	}
	if !ios.ValidFlat(flat) {
		t.Error("persisted .tw artifact is invalid")
	}
	t.Logf("built .tw %d bytes from dSYM %d bytes", len(flat), len(store.data[dsymKey]))

	sharedCache = newSymbolicatorCache()
	store.mu.Lock()
	delete(store.data, dsymKey)
	store.mu.Unlock()

	if out := ResolveIOSStackTrace(context.Background(), projectId, raw); !strings.Contains(out, "leaf (") {
		t.Errorf("cold load from .tw failed:\n%s", out)
	}
}

func TestIOSSymbolsKeyForms(t *testing.T) {
	pid := uuid.New()
	dashed := "2DD7-1042-1184-32BE-8F92-DD4E3D3FE24A"
	if IOSSymbolsKey(pid, dashed) != IOSSymbolsKey(pid, iosFixtureUUID) {
		t.Error("dashed/uppercase UUID must produce the same key as normalized hex")
	}
	if !strings.HasSuffix(IOSSymbolsKey(pid, iosFixtureUUID), "/"+iosFixtureUUID+".dsym") {
		t.Errorf("key %q should be <uuid>.dsym", IOSSymbolsKey(pid, iosFixtureUUID))
	}
}
