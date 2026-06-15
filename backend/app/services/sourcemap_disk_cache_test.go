package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/twcache"

	"github.com/google/uuid"
)

func useMemCache(t *testing.T) {
	t.Helper()
	prev := sharedCache
	sharedCache = newSymbolicatorCache()
	t.Cleanup(func() { sharedCache = prev })
}

func useDiskCache(t *testing.T, dir string, maxBytes int64) *twcache.Cache {
	t.Helper()
	prev := sharedCache
	if err := EnableSymbolicatorDiskCache(dir, maxBytes); err != nil {
		t.Fatalf("EnableSymbolicatorDiskCache: %v", err)
	}
	t.Cleanup(func() { sharedCache = prev })
	return sharedCache
}

func resolveSMFrame(t *testing.T, ctx context.Context, mapKey, bundleKey string) (sourcemap.StackTraceFrame, bool, error) {
	t.Helper()
	data, done, err := sharedCache.Get(ctx, twKeyFor(mapKey), loadSourceMapBlob(mapKey, bundleKey))
	if err != nil {
		return sourcemap.StackTraceFrame{}, false, err
	}
	defer done()
	frame, ok := sourcemap.LookupTW(data, 0, 10)
	return frame, ok, nil
}

func twPathFor(disk *twcache.Cache, mapKey string) string {
	return filepath.Join(disk.Dir(), filepath.FromSlash(twKeyFor(mapKey)))
}

func swapStorage(t *testing.T) *countingStorage {
	t.Helper()
	prev := storage.Store
	t.Cleanup(func() { storage.Store = prev })
	cs := &countingStorage{reads: map[string]int{}, data: map[string][]byte{}}
	storage.Store = cs
	smStoreHits.Store(0)
	smBuilds.Store(0)
	return cs
}

func assertSimpleLookup(t *testing.T, frame sourcemap.StackTraceFrame, ok bool) {
	t.Helper()
	if !ok {
		t.Fatal("expected lookup to resolve")
	}
	if frame.File != "tests/fixtures/simple/original.js" || frame.Fn != "abcd" {
		t.Fatalf("unexpected frame: %+v", frame)
	}
}

func TestDiskCacheBuildsThenServesFromLocalFile(t *testing.T) {
	cs := swapStorage(t)
	disk := useDiskCache(t, t.TempDir(), 64<<20)

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"minified.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	seedFixture(t, cs, prefix+"minified.js", "testdata/sourcemapcache/simple/minified.js")

	mapKey := prefix + "minified.js.map"
	bundleKey := prefix + "minified.js"

	frame, ok, err := resolveSMFrame(t, context.Background(), mapKey, bundleKey)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	assertSimpleLookup(t, frame, ok)

	if _, err := os.Stat(twPathFor(disk, mapKey)); err != nil {
		t.Fatalf("expected tw file on disk: %v", err)
	}
	if smBuilds.Load() != 1 {
		t.Fatalf("builds: got %d, want 1", smBuilds.Load())
	}

	restarted := useDiskCache(t, disk.Dir(), 64<<20)
	mapReads := cs.reads[mapKey]
	frame2, ok2, err := resolveSMFrame(t, context.Background(), mapKey, bundleKey)
	if err != nil {
		t.Fatalf("resolve after restart: %v", err)
	}
	assertSimpleLookup(t, frame2, ok2)
	if hits := restarted.Stats().Hits; hits != 1 {
		t.Fatalf("disk hits: got %d, want 1", hits)
	}
	if cs.reads[mapKey] != mapReads {
		t.Fatal("restart should serve from local tw file without reading the source map")
	}
}

func TestDiskCachePullsTWFromStorage(t *testing.T) {
	cs := swapStorage(t)
	disk := useDiskCache(t, t.TempDir(), 64<<20)

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	mapKey := prefix + "minified.js.map"
	bundleKey := prefix + "minified.js"

	mapBytes, err := os.ReadFile("testdata/sourcemapcache/simple/minified.js.map")
	if err != nil {
		t.Fatal(err)
	}
	bundleBytes, err := os.ReadFile("testdata/sourcemapcache/simple/minified.js")
	if err != nil {
		t.Fatal(err)
	}
	tw, err := sourcemap.BuildTW(mapBytes, bundleBytes)
	if err != nil {
		t.Fatal(err)
	}
	cs.data[prefix+"minified.js.tw"] = tw

	frame, ok, err := resolveSMFrame(t, context.Background(), mapKey, bundleKey)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	assertSimpleLookup(t, frame, ok)

	if smStoreHits.Load() != 1 {
		t.Fatalf("storeHits: got %d, want 1", smStoreHits.Load())
	}
	if cs.reads[mapKey] != 0 || cs.reads[bundleKey] != 0 {
		t.Fatal("tw artifact in storage should make map and bundle reads unnecessary")
	}
	if !fileExists(twPathFor(disk, mapKey)) {
		t.Fatal("tw pulled from storage should be cached on local disk")
	}
}

func TestDiskCacheCorruptLocalFileFallsBack(t *testing.T) {
	cs := swapStorage(t)
	disk := useDiskCache(t, t.TempDir(), 64<<20)

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"minified.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	mapKey := prefix + "minified.js.map"

	twPath := twPathFor(disk, mapKey)
	if err := os.MkdirAll(filepath.Dir(twPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(twPath, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}

	frame, ok, err := resolveSMFrame(t, context.Background(), mapKey, prefix+"minified.js")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !ok {
		t.Fatal("expected lookup to resolve after rebuilding from source map")
	}
	_ = frame
	if smBuilds.Load() != 1 {
		t.Fatalf("builds: got %d, want 1", smBuilds.Load())
	}
	data, err := os.ReadFile(twPath)
	if err != nil {
		t.Fatalf("expected regenerated tw file: %v", err)
	}
	if !sourcemap.ValidTW(data) {
		t.Fatal("regenerated tw file should be valid")
	}
}

func TestDiskCacheCapacityEviction(t *testing.T) {
	cs := swapStorage(t)

	mapBytes, err := os.ReadFile("testdata/sourcemapcache/simple/minified.js.map")
	if err != nil {
		t.Fatal(err)
	}
	tw, err := sourcemap.BuildTW(mapBytes, nil)
	if err != nil {
		t.Fatal(err)
	}
	twSize := int64(len(tw))

	disk := useDiskCache(t, t.TempDir(), twSize+twSize/2)

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"first.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	seedFixture(t, cs, prefix+"second.js.map", "testdata/sourcemapcache/simple/minified.js.map")

	firstKey := prefix + "first.js.map"
	secondKey := prefix + "second.js.map"

	if _, _, err := resolveSMFrame(t, context.Background(), firstKey, prefix+"first.js"); err != nil {
		t.Fatalf("resolve first: %v", err)
	}
	if _, _, err := resolveSMFrame(t, context.Background(), secondKey, prefix+"second.js"); err != nil {
		t.Fatalf("resolve second: %v", err)
	}

	if fileExists(twPathFor(disk, firstKey)) {
		t.Fatal("oldest tw file should be evicted when over capacity")
	}
	if !fileExists(twPathFor(disk, secondKey)) {
		t.Fatal("newest tw file should survive eviction")
	}
	stats := disk.Stats()
	if stats.Evictions != 1 {
		t.Fatalf("disk evictions: got %d, want 1", stats.Evictions)
	}
	if stats.Bytes > stats.MaxBytes {
		t.Fatalf("cached bytes %d exceed maxBytes %d", stats.Bytes, stats.MaxBytes)
	}
}

func TestDiskCacheInvalidateRemovesFile(t *testing.T) {
	cs := swapStorage(t)
	disk := useDiskCache(t, t.TempDir(), 64<<20)

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"minified.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	mapKey := prefix + "minified.js.map"

	if _, _, err := resolveSMFrame(t, context.Background(), mapKey, prefix+"minified.js"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	twPath := twPathFor(disk, mapKey)
	if !fileExists(twPath) {
		t.Fatal("expected tw file before invalidate")
	}

	InvalidateSourceMap(projectId, "minified.js.map")
	if fileExists(twPath) {
		t.Fatal("invalidate should remove the local tw file")
	}
	if stats := disk.Stats(); stats.Entries != 0 || stats.Bytes != 0 {
		t.Fatalf("expected empty disk index, got %d entries / %d bytes", stats.Entries, stats.Bytes)
	}
}

func TestDiskCacheResolveStackTraceEndToEnd(t *testing.T) {
	cs := swapStorage(t)
	useDiskCache(t, t.TempDir(), 64<<20)

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"minified.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	seedFixture(t, cs, prefix+"minified.js", "testdata/sourcemapcache/simple/minified.js")

	input := "Error: boom\nanonymous()\n    minified.js:1:11"
	lines := strings.Split(ResolveStackTrace(context.Background(), projectId, input, nil), "\n")

	if got, want := lines[2], "    tests/fixtures/simple/original.js:2:10"; got != want {
		t.Errorf("location: got %q, want %q", got, want)
	}
	if got, want := lines[1], "abcd()"; got != want {
		t.Errorf("function name: got %q, want %q", got, want)
	}
}

func TestGenerateTWArtifacts(t *testing.T) {
	cs := swapStorage(t)
	useDiskCache(t, t.TempDir(), 64<<20)

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"minified.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	seedFixture(t, cs, prefix+"minified.js", "testdata/sourcemapcache/simple/minified.js")

	GenerateTWArtifacts(context.Background(), projectId, []string{"minified.js.map", "minified.js"})

	twBytes, ok := cs.data[prefix+"minified.js.tw"]
	if !ok {
		t.Fatal("expected tw artifact in storage after generation")
	}
	frame, lok := sourcemap.LookupTW(twBytes, 0, 10)
	assertSimpleLookup(t, frame, lok)
}

func TestGenerateTWArtifactsDeletesStaleOnFailure(t *testing.T) {
	cs := swapStorage(t)
	useDiskCache(t, t.TempDir(), 64<<20)

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	cs.data[prefix+"minified.js.tw"] = []byte("stale artifact from previous release")
	cs.data[prefix+"minified.js.map"] = []byte("{not a valid source map")

	GenerateTWArtifacts(context.Background(), projectId, []string{"minified.js.map"})

	if _, ok := cs.data[prefix+"minified.js.tw"]; ok {
		t.Fatal("stale tw artifact must be deleted even when regeneration fails")
	}
}

type twErrStorage struct {
	*countingStorage
}

func (s *twErrStorage) Read(ctx context.Context, key string) ([]byte, error) {
	if strings.HasSuffix(key, ".tw") {
		return nil, fmt.Errorf("storage backend unavailable")
	}
	return s.countingStorage.Read(ctx, key)
}

func TestDiskCacheTransientTwErrorFallsBackToBuild(t *testing.T) {
	cs := swapStorage(t)
	storage.Store = &twErrStorage{countingStorage: cs}
	useDiskCache(t, t.TempDir(), 64<<20)

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"minified.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	seedFixture(t, cs, prefix+"minified.js", "testdata/sourcemapcache/simple/minified.js")
	mapKey := prefix + "minified.js.map"

	frame, ok, err := resolveSMFrame(t, context.Background(), mapKey, prefix+"minified.js")
	if err != nil {
		t.Fatalf("resolve should fall back to building from the map, got: %v", err)
	}
	assertSimpleLookup(t, frame, ok)
	if smBuilds.Load() != 1 {
		t.Fatalf("builds: got %d, want 1", smBuilds.Load())
	}
	if _, ok := cs.data[prefix+"minified.js.tw"]; ok {
		t.Fatal("transient tw read error must not trigger a storage refresh")
	}
}

func TestDiskCacheRefreshesCorruptStoreTw(t *testing.T) {
	cs := swapStorage(t)
	useDiskCache(t, t.TempDir(), 64<<20)

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"minified.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	cs.data[prefix+"minified.js.tw"] = []byte("corrupt artifact")
	mapKey := prefix + "minified.js.map"

	_, ok, err := resolveSMFrame(t, context.Background(), mapKey, prefix+"minified.js")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !ok {
		t.Fatal("expected lookup to resolve after rebuild")
	}

	if !sourcemap.ValidTW(cs.data[prefix+"minified.js.tw"]) {
		t.Fatal("corrupt storage tw should be replaced with a valid artifact")
	}
}

func TestDiskCacheScanSkipsUnreadableEntries(t *testing.T) {
	swapStorage(t)
	dir := t.TempDir()

	locked := filepath.Join(dir, "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "x.tw"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	if _, err := twcache.NewDisk(dir, 64<<20, nil); err != nil {
		t.Fatalf("scan must tolerate unreadable entries, got: %v", err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
