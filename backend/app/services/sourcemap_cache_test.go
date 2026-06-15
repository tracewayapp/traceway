package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/storage"

	"github.com/google/uuid"
)

type countingStorage struct {
	mu    sync.Mutex
	reads map[string]int
	data  map[string][]byte
}

func (c *countingStorage) Write(_ context.Context, key string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = data
	return nil
}

func (c *countingStorage) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	return nil
}

func (c *countingStorage) Read(_ context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	c.reads[key]++
	c.mu.Unlock()
	time.Sleep(10 * time.Millisecond)
	d, ok := c.data[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return d, nil
}

func TestResolveStackTraceFailedMapAttemptedOncePerTrace(t *testing.T) {
	useMemCache(t)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	cs := &countingStorage{reads: map[string]int{}, data: map[string][]byte{}}
	storage.Store = cs

	projectId := uuid.New()
	trace := "Error: boom\n    fn()\n    dead.js:1:10\n    fn2()\n    dead.js:1:20\n    fn3()\n    dead.js:1:30"

	resolved := ResolveStackTrace(context.Background(), projectId, trace, nil)
	if resolved != trace {
		t.Error("trace should be stored as-is when its source map cannot be loaded")
	}
	mapKey := fmt.Sprintf("sourcemaps/%s/dead.js.map", projectId)
	if got := cs.reads[mapKey]; got != 1 {
		t.Errorf("expected 1 load attempt per trace for a failing map, got %d", got)
	}
}

type flakyStorage struct {
	mu    sync.Mutex
	reads int
}

func (f *flakyStorage) Write(_ context.Context, _ string, _ []byte) error { return nil }
func (f *flakyStorage) Delete(_ context.Context, _ string) error          { return nil }

func (f *flakyStorage) Read(_ context.Context, _ string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	return nil, errors.New("storage down")
}

func TestResolveStackTraceTransientFailureNegativeCached(t *testing.T) {
	useMemCache(t)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	fs := &flakyStorage{}
	storage.Store = fs

	projectId := uuid.New()
	trace := "Error: boom\n    fn()\n    down.js:1:10"

	_ = ResolveStackTrace(context.Background(), projectId, trace, nil)
	firstReads := fs.reads
	_ = ResolveStackTrace(context.Background(), projectId, trace, nil)

	if fs.reads != firstReads {
		t.Errorf("transient storage failures should be negative cached, got %d extra reads", fs.reads-firstReads)
	}
}

func TestInvalidateSourceMapClearsNegative(t *testing.T) {
	useMemCache(t)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	cs := &countingStorage{reads: map[string]int{}, data: map[string][]byte{}}
	storage.Store = cs

	projectId := uuid.New()
	mapKey := fmt.Sprintf("sourcemaps/%s/late.js.map", projectId)
	trace := "Error: boom\n    fn()\n    late.js:1:1"

	if got := ResolveStackTrace(context.Background(), projectId, trace, nil); got != trace {
		t.Error("trace should pass through while the map is missing")
	}

	cs.data[mapKey] = []byte(`{"version":3,"sources":["a.js"],"names":[],"mappings":"AAAA"}`)
	InvalidateSourceMap(projectId, "late.js.map")

	resolved := ResolveStackTrace(context.Background(), projectId, trace, nil)
	if !strings.Contains(resolved, "a.js:1:1") {
		t.Errorf("upload should clear the negative entry immediately, got %q", resolved)
	}
}

func TestInvalidateSourceMapEvictsStaleResolver(t *testing.T) {
	useMemCache(t)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	cs := &countingStorage{reads: map[string]int{}, data: map[string][]byte{}}
	storage.Store = cs

	projectId := uuid.New()
	mapKey := fmt.Sprintf("sourcemaps/%s/stable.js.map", projectId)
	cs.data[mapKey] = []byte(`{"version":3,"sources":["a.js"],"names":[],"mappings":"AAAA"}`)
	trace := "Error: boom\n    fn()\n    stable.js:1:1"

	if got := ResolveStackTrace(context.Background(), projectId, trace, nil); !strings.Contains(got, "a.js:1:1") {
		t.Fatalf("expected initial resolution against a.js, got %q", got)
	}

	cs.data[mapKey] = []byte(`{"version":3,"sources":["b.js"],"names":[],"mappings":"AAAA"}`)
	if got := ResolveStackTrace(context.Background(), projectId, trace, nil); !strings.Contains(got, "a.js:1:1") {
		t.Fatalf("re-upload without invalidation should still serve the cached resolver, got %q", got)
	}

	GenerateTWArtifacts(context.Background(), projectId, []string{"stable.js.map"})
	if got := ResolveStackTrace(context.Background(), projectId, trace, nil); !strings.Contains(got, "b.js:1:1") {
		t.Errorf("upload must evict both the cached artifact and the stale tw, got %q", got)
	}
}

type bundleFailStorage struct {
	data       map[string][]byte
	failBundle bool
}

func (b *bundleFailStorage) Delete(_ context.Context, _ string) error { return nil }

func (b *bundleFailStorage) Write(_ context.Context, key string, data []byte) error {
	b.data[key] = data
	return nil
}

func (b *bundleFailStorage) Read(_ context.Context, key string) ([]byte, error) {
	if b.failBundle && !strings.HasSuffix(key, ".map") && !strings.HasSuffix(key, ".tw") {
		return nil, errors.New("storage down")
	}
	d, ok := b.data[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return d, nil
}

func TestTransientBundleReadFailureFailsBuild(t *testing.T) {
	useMemCache(t)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	bs := &bundleFailStorage{failBundle: true, data: map[string][]byte{
		prefix + "app.js.map": []byte(`{"version":3,"sources":["a.js"],"names":[],"mappings":"AAAA"}`),
		prefix + "app.js":     []byte(`var x=1;`),
	}}
	storage.Store = bs

	trace := "Error: boom\n    fn()\n    app.js:1:1"

	if got := ResolveStackTrace(context.Background(), projectId, trace, nil); got != trace {
		t.Errorf("a transient bundle read failure must not cache a names-less artifact, got %q", got)
	}

	bs.failBundle = false
	InvalidateSourceMap(projectId, "app.js.map")

	if got := ResolveStackTrace(context.Background(), projectId, trace, nil); !strings.Contains(got, "a.js:1:1") {
		t.Errorf("expected full resolution once the bundle read recovers, got %q", got)
	}
}
