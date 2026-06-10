package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/twcache"

	"github.com/google/uuid"
)

func newTestDiskCache(t *testing.T, maxBytes int64) *sourceMapDiskCache {
	t.Helper()
	return newTestDiskCacheAt(t, t.TempDir(), maxBytes)
}

func newTestDiskCacheAt(t *testing.T, dir string, maxBytes int64) *sourceMapDiskCache {
	t.Helper()
	disk, err := twcache.New(dir, maxBytes, nil)
	if err != nil {
		t.Fatalf("twcache.New: %v", err)
	}
	return &sourceMapDiskCache{
		mem:  newTestSourceMapCache(100, 64<<20),
		disk: disk,
	}
}

func twPathFor(d *sourceMapDiskCache, mapKey string) string {
	return filepath.Join(d.disk.Dir(), filepath.FromSlash(twKeyFor(mapKey)))
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

func assertSimpleLookup(t *testing.T, r *symbolicator.Resolver) {
	t.Helper()
	frame, ok := r.Lookup(0, 10)
	if !ok {
		t.Fatal("expected lookup to resolve")
	}
	if frame.File != "tests/fixtures/simple/original.js" || frame.Fn != "abcd" {
		t.Fatalf("unexpected frame: %+v", frame)
	}
}

func TestDiskCacheBuildsThenServesFromLocalFile(t *testing.T) {
	cs := swapStorage(t)
	d := newTestDiskCache(t, 64<<20)

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"minified.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	seedFixture(t, cs, prefix+"minified.js", "testdata/sourcemapcache/simple/minified.js")

	mapKey := prefix + "minified.js.map"
	bundleKey := prefix + "minified.js"

	r, err := d.getOrBuild(context.Background(), mapKey, buildResolver(mapKey, bundleKey))
	if err != nil {
		t.Fatalf("getOrBuild: %v", err)
	}
	assertSimpleLookup(t, r)

	if _, err := os.Stat(twPathFor(d, mapKey)); err != nil {
		t.Fatalf("expected tw file on disk: %v", err)
	}
	if smBuilds.Load() != 1 {
		t.Fatalf("builds: got %d, want 1", smBuilds.Load())
	}

	restarted := newTestDiskCacheAt(t, d.disk.Dir(), 64<<20)

	mapReads := cs.reads[mapKey]
	r2, err := restarted.getOrBuild(context.Background(), mapKey, buildResolver(mapKey, bundleKey))
	if err != nil {
		t.Fatalf("getOrBuild after restart: %v", err)
	}
	assertSimpleLookup(t, r2)
	if hits := restarted.disk.Stats().Hits; hits != 1 {
		t.Fatalf("disk hits: got %d, want 1", hits)
	}
	if cs.reads[mapKey] != mapReads {
		t.Fatal("restart should serve from local tw file without reading the source map")
	}
}

func TestDiskCachePullsTWFromStorage(t *testing.T) {
	cs := swapStorage(t)
	d := newTestDiskCache(t, 64<<20)

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
	resolver, err := symbolicator.NewResolver(mapBytes, bundleBytes)
	if err != nil {
		t.Fatal(err)
	}
	cs.data[prefix+"minified.js.tw"] = resolver.MarshalTW()

	r, err := d.getOrBuild(context.Background(), mapKey, buildResolver(mapKey, bundleKey))
	if err != nil {
		t.Fatalf("getOrBuild: %v", err)
	}
	assertSimpleLookup(t, r)

	if smStoreHits.Load() != 1 {
		t.Fatalf("storeHits: got %d, want 1", smStoreHits.Load())
	}
	if cs.reads[mapKey] != 0 || cs.reads[bundleKey] != 0 {
		t.Fatal("tw artifact in storage should make map and bundle reads unnecessary")
	}
	if !fileExists(twPathFor(d, mapKey)) {
		t.Fatal("tw pulled from storage should be cached on local disk")
	}
}

func TestDiskCacheCorruptLocalFileFallsBack(t *testing.T) {
	cs := swapStorage(t)
	d := newTestDiskCache(t, 64<<20)

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"minified.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	mapKey := prefix + "minified.js.map"

	twPath := twPathFor(d, mapKey)
	if err := os.MkdirAll(filepath.Dir(twPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(twPath, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := d.getOrBuild(context.Background(), mapKey, buildResolver(mapKey, prefix+"minified.js"))
	if err != nil {
		t.Fatalf("getOrBuild: %v", err)
	}
	if _, ok := r.Lookup(0, 10); !ok {
		t.Fatal("expected lookup to resolve after rebuilding from source map")
	}
	if smBuilds.Load() != 1 {
		t.Fatalf("builds: got %d, want 1", smBuilds.Load())
	}
	data, err := os.ReadFile(twPath)
	if err != nil {
		t.Fatalf("expected regenerated tw file: %v", err)
	}
	if _, err := symbolicator.OpenTW(data); err != nil {
		t.Fatalf("regenerated tw file should be valid: %v", err)
	}
}

func TestDiskCacheCapacityEviction(t *testing.T) {
	cs := swapStorage(t)

	mapBytes, err := os.ReadFile("testdata/sourcemapcache/simple/minified.js.map")
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := symbolicator.NewResolver(mapBytes, nil)
	if err != nil {
		t.Fatal(err)
	}
	twSize := int64(len(resolver.MarshalTW()))

	d := newTestDiskCache(t, twSize+twSize/2)

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"first.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	seedFixture(t, cs, prefix+"second.js.map", "testdata/sourcemapcache/simple/minified.js.map")

	firstKey := prefix + "first.js.map"
	secondKey := prefix + "second.js.map"

	if _, err := d.getOrBuild(context.Background(), firstKey, buildResolver(firstKey, prefix+"first.js")); err != nil {
		t.Fatalf("getOrBuild first: %v", err)
	}
	if _, err := d.getOrBuild(context.Background(), secondKey, buildResolver(secondKey, prefix+"second.js")); err != nil {
		t.Fatalf("getOrBuild second: %v", err)
	}

	if fileExists(twPathFor(d, firstKey)) {
		t.Fatal("oldest tw file should be evicted when over capacity")
	}
	if !fileExists(twPathFor(d, secondKey)) {
		t.Fatal("newest tw file should survive eviction")
	}
	stats := d.disk.Stats()
	if stats.Evictions != 1 {
		t.Fatalf("disk evictions: got %d, want 1", stats.Evictions)
	}
	if stats.Bytes > stats.MaxBytes {
		t.Fatalf("cached bytes %d exceed maxBytes %d", stats.Bytes, stats.MaxBytes)
	}
}

func TestDiskCacheInvalidateRemovesFile(t *testing.T) {
	cs := swapStorage(t)
	d := newTestDiskCache(t, 64<<20)

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"minified.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	mapKey := prefix + "minified.js.map"

	if _, err := d.getOrBuild(context.Background(), mapKey, buildResolver(mapKey, prefix+"minified.js")); err != nil {
		t.Fatalf("getOrBuild: %v", err)
	}
	twPath := twPathFor(d, mapKey)
	if !fileExists(twPath) {
		t.Fatal("expected tw file before invalidate")
	}

	d.invalidate(mapKey)
	if fileExists(twPath) {
		t.Fatal("invalidate should remove the local tw file")
	}
	if stats := d.disk.Stats(); stats.Entries != 0 || stats.Bytes != 0 {
		t.Fatalf("expected empty disk index, got %d entries / %d bytes", stats.Entries, stats.Bytes)
	}
}

func TestDiskCacheResolveStackTraceEndToEnd(t *testing.T) {
	cs := swapStorage(t)
	swapActiveCache(t, newTestDiskCache(t, 64<<20))

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

func swapActiveCache(t *testing.T, c resolverCache) {
	t.Helper()
	prev := activeSMCache
	activeSMCache = c
	t.Cleanup(func() { activeSMCache = prev })
}

func TestGenerateTWArtifacts(t *testing.T) {
	cs := swapStorage(t)
	swapActiveCache(t, newTestDiskCache(t, 64<<20))

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"minified.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	seedFixture(t, cs, prefix+"minified.js", "testdata/sourcemapcache/simple/minified.js")

	GenerateTWArtifacts(context.Background(), projectId, []string{"minified.js.map", "minified.js"})

	twBytes, ok := cs.data[prefix+"minified.js.tw"]
	if !ok {
		t.Fatal("expected tw artifact in storage after generation")
	}
	r, err := symbolicator.OpenTW(twBytes)
	if err != nil {
		t.Fatalf("OpenTW: %v", err)
	}
	assertSimpleLookup(t, r)
}

func TestGenerateTWArtifactsDeletesStaleOnFailure(t *testing.T) {
	cs := swapStorage(t)
	swapActiveCache(t, newTestDiskCache(t, 64<<20))

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
	d := newTestDiskCache(t, 64<<20)

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"minified.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	seedFixture(t, cs, prefix+"minified.js", "testdata/sourcemapcache/simple/minified.js")
	mapKey := prefix + "minified.js.map"

	r, err := d.getOrBuild(context.Background(), mapKey, buildResolver(mapKey, prefix+"minified.js"))
	if err != nil {
		t.Fatalf("getOrBuild should fall back to building from the map, got: %v", err)
	}
	assertSimpleLookup(t, r)
	if smBuilds.Load() != 1 {
		t.Fatalf("builds: got %d, want 1", smBuilds.Load())
	}
	if _, ok := cs.data[prefix+"minified.js.tw"]; ok {
		t.Fatal("transient tw read error must not trigger a storage refresh")
	}
}

func TestDiskCacheRefreshesCorruptStoreTw(t *testing.T) {
	cs := swapStorage(t)
	d := newTestDiskCache(t, 64<<20)

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"minified.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	cs.data[prefix+"minified.js.tw"] = []byte("corrupt artifact")
	mapKey := prefix + "minified.js.map"

	r, err := d.getOrBuild(context.Background(), mapKey, buildResolver(mapKey, prefix+"minified.js"))
	if err != nil {
		t.Fatalf("getOrBuild: %v", err)
	}
	if _, ok := r.Lookup(0, 10); !ok {
		t.Fatal("expected lookup to resolve after rebuild")
	}

	refreshed := cs.data[prefix+"minified.js.tw"]
	if _, err := symbolicator.OpenTW(refreshed); err != nil {
		t.Fatalf("corrupt storage tw should be replaced with a valid artifact: %v", err)
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

	if _, err := twcache.New(dir, 64<<20, nil); err != nil {
		t.Fatalf("scan must tolerate unreadable entries, got: %v", err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
