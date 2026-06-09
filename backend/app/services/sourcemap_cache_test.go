package services

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator"

	"github.com/google/uuid"
)

func newTestSourceMapCache(maxEntries int, maxBytes int64) *sourceMapCache {
	return &sourceMapCache{
		items:      make(map[string]*list.Element),
		order:      list.New(),
		loading:    make(map[string]*resolverLoad),
		negative:   make(map[string]*negativeEntry),
		maxEntries: maxEntries,
		maxBytes:   maxBytes,
	}
}

func storageResolverBuild(key string) resolverBuild {
	return func(ctx context.Context) (*symbolicator.Resolver, int64, error) {
		data, err := storage.Store.Read(ctx, key)
		if err != nil {
			return nil, 0, err
		}
		r, err := symbolicator.NewResolver(data, nil)
		if err != nil {
			return nil, 0, err
		}
		return r, int64(len(data)), nil
	}
}

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

func TestSourceMapCacheSingleflight(t *testing.T) {
	prev := storage.Store
	defer func() { storage.Store = prev }()
	cs := &countingStorage{reads: map[string]int{}, data: map[string][]byte{
		"singleflight-test.js.map": []byte(`{"version":3,"sources":["a.js"],"names":[],"mappings":"AAAA"}`),
	}}
	storage.Store = cs
	c := newTestSourceMapCache(10, 1<<20)

	const n = 16
	results := make([]*symbolicator.Resolver, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Go(func() {
			<-start
			r, err := c.getOrBuild(context.Background(), "singleflight-test.js.map", storageResolverBuild("singleflight-test.js.map"))
			if err != nil {
				t.Error(err)
				return
			}
			results[i] = r
		})
	}
	close(start)
	wg.Wait()

	if got := cs.reads["singleflight-test.js.map"]; got != 1 {
		t.Errorf("expected 1 storage read for concurrent lookups, got %d", got)
	}
	for i := 1; i < n; i++ {
		if results[i] != results[0] {
			t.Fatal("expected all callers to share a single resolver instance")
		}
	}

	if _, err := c.getOrBuild(context.Background(), "singleflight-test.js.map", storageResolverBuild("singleflight-test.js.map")); err != nil {
		t.Fatal(err)
	}
	if got := cs.reads["singleflight-test.js.map"]; got != 1 {
		t.Errorf("expected cached lookup to not hit storage, got %d reads", got)
	}
}

func TestSourceMapCacheDistinctKeysConcurrent(t *testing.T) {
	prev := storage.Store
	defer func() { storage.Store = prev }()
	cs := &countingStorage{reads: map[string]int{}, data: map[string][]byte{}}
	const keys = 8
	for i := range keys {
		cs.data[fmt.Sprintf("distinct-%d.js.map", i)] = []byte(`{"version":3,"sources":["a.js"],"names":[],"mappings":"AAAA"}`)
	}
	storage.Store = cs
	c := newTestSourceMapCache(keys, 1<<20)

	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make([]*symbolicator.Resolver, keys*4)
	for i := range keys * 4 {
		wg.Go(func() {
			<-start
			key := fmt.Sprintf("distinct-%d.js.map", i%keys)
			r, err := c.getOrBuild(context.Background(), key, storageResolverBuild(key))
			if err != nil {
				t.Error(err)
				return
			}
			results[i] = r
		})
	}
	close(start)
	wg.Wait()

	for i := range keys {
		key := fmt.Sprintf("distinct-%d.js.map", i)
		if got := cs.reads[key]; got != 1 {
			t.Errorf("key %s: expected 1 storage read, got %d", key, got)
		}
	}
	for i := range keys * 4 {
		if results[i] != results[i%keys] {
			t.Fatal("callers of the same key should share one instance")
		}
	}
}

type flakyStorage struct {
	mu       sync.Mutex
	reads    int
	failures int
	data     []byte
}

func (f *flakyStorage) Write(_ context.Context, _ string, _ []byte) error { return nil }

func (f *flakyStorage) Read(_ context.Context, _ string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	if f.reads <= f.failures {
		return nil, errors.New("storage down")
	}
	return f.data, nil
}

func TestSourceMapCacheFailedLoadRetries(t *testing.T) {
	prev := storage.Store
	defer func() { storage.Store = prev }()
	fs := &flakyStorage{failures: 1, data: []byte(`{"version":3,"sources":["a.js"],"names":[],"mappings":"AAAA"}`)}
	storage.Store = fs
	c := newTestSourceMapCache(10, 1<<20)

	if _, err := c.getOrBuild(context.Background(), "flaky.js.map", storageResolverBuild("flaky.js.map")); err == nil {
		t.Fatal("expected first load to fail")
	}
	c.mu.Lock()
	if c.failures != 1 {
		t.Errorf("expected 1 recorded failure, got %d", c.failures)
	}
	c.mu.Unlock()
	r, err := c.getOrBuild(context.Background(), "flaky.js.map", storageResolverBuild("flaky.js.map"))
	if err != nil || r == nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if fs.reads != 2 {
		t.Errorf("expected 2 storage reads (fail then retry), got %d", fs.reads)
	}
	if _, err := c.getOrBuild(context.Background(), "flaky.js.map", storageResolverBuild("flaky.js.map")); err != nil {
		t.Fatal(err)
	}
	if fs.reads != 2 {
		t.Errorf("expected cached lookup after retry, got %d reads", fs.reads)
	}
}

type blockingPanicStorage struct {
	release chan struct{}
}

func (b *blockingPanicStorage) Write(_ context.Context, _ string, _ []byte) error { return nil }

func (b *blockingPanicStorage) Read(_ context.Context, _ string) ([]byte, error) {
	<-b.release
	panic("storage exploded")
}

func TestSourceMapCachePanicRecovery(t *testing.T) {
	prev := storage.Store
	defer func() { storage.Store = prev }()
	release := make(chan struct{})
	storage.Store = &blockingPanicStorage{release: release}
	c := newTestSourceMapCache(10, 1<<20)

	type result struct {
		resolver *symbolicator.Resolver
		err      error
	}
	leader := make(chan result, 1)
	go func() {
		r, err := c.getOrBuild(context.Background(), "boom.js.map", storageResolverBuild("boom.js.map"))
		leader <- result{r, err}
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		c.mu.Lock()
		_, loading := c.loading["boom.js.map"]
		c.mu.Unlock()
		if loading {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("load never started")
		}
		time.Sleep(time.Millisecond)
	}

	waiters := make(chan result, 2)
	for range 2 {
		go func() {
			r, err := c.getOrBuild(context.Background(), "boom.js.map", storageResolverBuild("boom.js.map"))
			waiters <- result{r, err}
		}()
	}

	close(release)

	r := <-leader
	if r.err == nil || r.resolver != nil {
		t.Fatalf("expected leader to receive panic error, got resolver=%v err=%v", r.resolver, r.err)
	}
	for range 2 {
		r := <-waiters
		if r.err == nil || r.resolver != nil {
			t.Fatalf("expected waiter to receive an error, got resolver=%v err=%v", r.resolver, r.err)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.loading) != 0 {
		t.Error("loading entry leaked after panic")
	}
	if c.order.Len() != 0 {
		t.Error("nothing should be cached after a failed load")
	}
	if c.failures == 0 {
		t.Error("expected at least one recorded failure")
	}
}

func TestResolveStackTraceFailedMapAttemptedOncePerTrace(t *testing.T) {
	prev := storage.Store
	defer func() { storage.Store = prev }()
	cs := &countingStorage{reads: map[string]int{}, data: map[string][]byte{}}
	storage.Store = cs

	projectId := uuid.New()
	trace := "Error: boom\n    fn()\n    dead.js:1:10\n    fn2()\n    dead.js:1:20\n    fn3()\n    dead.js:1:30"

	resolved := ResolveStackTrace(context.Background(), projectId, trace)
	if resolved != trace {
		t.Error("trace should be stored as-is when its source map cannot be loaded")
	}
	mapKey := fmt.Sprintf("sourcemaps/%s/dead.js.map", projectId)
	if got := cs.reads[mapKey]; got != 1 {
		t.Errorf("expected 1 load attempt per trace for a failing map, got %d", got)
	}
}

func TestSourceMapNegativeBackoffEscalates(t *testing.T) {
	c := newTestSourceMapCache(10, 1<<20)

	c.mu.Lock()
	c.markNegativeLocked("k", sourceMapNegativeBaseTTL)
	first := time.Until(c.negative["k"].expiresAt)
	c.mu.Unlock()
	if first > sourceMapNegativeBaseTTL || first < sourceMapNegativeBaseTTL-time.Second {
		t.Errorf("first failure should use the base TTL, got %v", first)
	}

	c.mu.Lock()
	c.markNegativeLocked("k", sourceMapNegativeBaseTTL)
	second := time.Until(c.negative["k"].expiresAt)
	for range 20 {
		c.markNegativeLocked("k", sourceMapNegativeBaseTTL)
	}
	capped := time.Until(c.negative["k"].expiresAt)
	c.mu.Unlock()

	if second < first {
		t.Errorf("TTL should escalate on repeat failures: first %v, second %v", first, second)
	}
	if capped > sourceMapNegativeMaxTTL || capped < sourceMapNegativeMaxTTL-time.Second {
		t.Errorf("TTL should cap at %v, got %v", sourceMapNegativeMaxTTL, capped)
	}
}

func TestSourceMapNegativeMapPrunesAtCap(t *testing.T) {
	c := newTestSourceMapCache(10, 1<<20)
	c.mu.Lock()
	for i := range sourceMapNegativeMaxKeys {
		c.markNegativeLocked(fmt.Sprintf("k-%d", i), sourceMapNegativeBaseTTL)
	}
	c.markNegativeLocked("one-more", sourceMapNegativeBaseTTL)
	size := len(c.negative)
	c.mu.Unlock()
	if size > sourceMapNegativeMaxKeys {
		t.Errorf("negative map should stay at or below %d keys, got %d", sourceMapNegativeMaxKeys, size)
	}
}

func TestResolveStackTraceTransientFailureNegativeCached(t *testing.T) {
	prev := storage.Store
	defer func() { storage.Store = prev }()
	fs := &flakyStorage{failures: 1 << 30}
	storage.Store = fs

	projectId := uuid.New()
	trace := "Error: boom\n    fn()\n    down.js:1:10"

	_ = ResolveStackTrace(context.Background(), projectId, trace)
	_ = ResolveStackTrace(context.Background(), projectId, trace)

	if fs.reads != 1 {
		t.Errorf("transient storage failures should be negative cached, got %d reads", fs.reads)
	}
}

func TestInvalidateSourceMapClearsNegative(t *testing.T) {
	InitSourceMapCache(100, 64<<20)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	cs := &countingStorage{reads: map[string]int{}, data: map[string][]byte{}}
	storage.Store = cs

	projectId := uuid.New()
	mapKey := fmt.Sprintf("sourcemaps/%s/late.js.map", projectId)
	trace := "Error: boom\n    fn()\n    late.js:1:1"

	if got := ResolveStackTrace(context.Background(), projectId, trace); got != trace {
		t.Error("trace should pass through while the map is missing")
	}

	cs.data[mapKey] = []byte(`{"version":3,"sources":["a.js"],"names":[],"mappings":"AAAA"}`)
	InvalidateSourceMap(projectId, "late.js.map")

	resolved := ResolveStackTrace(context.Background(), projectId, trace)
	if !strings.Contains(resolved, "a.js:1:1") {
		t.Errorf("upload should clear the negative entry immediately, got %q", resolved)
	}
}

func TestInvalidateSourceMapEvictsStaleResolver(t *testing.T) {
	InitSourceMapCache(100, 64<<20)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	cs := &countingStorage{reads: map[string]int{}, data: map[string][]byte{}}
	storage.Store = cs

	projectId := uuid.New()
	mapKey := fmt.Sprintf("sourcemaps/%s/stable.js.map", projectId)
	cs.data[mapKey] = []byte(`{"version":3,"sources":["a.js"],"names":[],"mappings":"AAAA"}`)
	trace := "Error: boom\n    fn()\n    stable.js:1:1"

	if got := ResolveStackTrace(context.Background(), projectId, trace); !strings.Contains(got, "a.js:1:1") {
		t.Fatalf("expected initial resolution against a.js, got %q", got)
	}

	cs.data[mapKey] = []byte(`{"version":3,"sources":["b.js"],"names":[],"mappings":"AAAA"}`)
	if got := ResolveStackTrace(context.Background(), projectId, trace); !strings.Contains(got, "a.js:1:1") {
		t.Fatalf("re-upload without invalidation should still serve the cached resolver, got %q", got)
	}

	InvalidateSourceMap(projectId, "stable.js")
	if got := ResolveStackTrace(context.Background(), projectId, trace); !strings.Contains(got, "b.js:1:1") {
		t.Errorf("invalidation should evict the cached resolver, got %q", got)
	}
}

type bundleFailStorage struct {
	data       map[string][]byte
	failBundle bool
}

func (b *bundleFailStorage) Write(_ context.Context, key string, data []byte) error {
	b.data[key] = data
	return nil
}

func (b *bundleFailStorage) Read(_ context.Context, key string) ([]byte, error) {
	if b.failBundle && !strings.HasSuffix(key, ".map") {
		return nil, errors.New("storage down")
	}
	d, ok := b.data[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return d, nil
}

func TestTransientBundleReadFailureFailsBuild(t *testing.T) {
	InitSourceMapCache(100, 64<<20)
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

	if got := ResolveStackTrace(context.Background(), projectId, trace); got != trace {
		t.Errorf("a transient bundle read failure must not cache a names-less resolver, got %q", got)
	}

	bs.failBundle = false
	InvalidateSourceMap(projectId, "app.js.map")

	if got := ResolveStackTrace(context.Background(), projectId, trace); !strings.Contains(got, "a.js:1:1") {
		t.Errorf("expected full resolution once the bundle read recovers, got %q", got)
	}
}

func TestSourceMapCacheByteCapEviction(t *testing.T) {
	prev := storage.Store
	defer func() { storage.Store = prev }()
	cs := &countingStorage{reads: map[string]int{}, data: map[string][]byte{}}
	storage.Store = cs

	c := newTestSourceMapCache(10, 2000)

	content := make([]byte, 700)
	for i := range content {
		content[i] = 'x'
	}
	for i := range 3 {
		key := fmt.Sprintf("evict-%d.js.map", i)
		cs.data[key] = fmt.Appendf(nil, `{"version":3,"sources":["a.js"],"sourcesContent":[%q],"names":[],"mappings":"AAAA"}`, content)
		if _, err := c.getOrBuild(context.Background(), key, storageResolverBuild(key)); err != nil {
			t.Fatal(err)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.curBytes > c.maxBytes {
		t.Errorf("curBytes %d exceeds maxBytes %d", c.curBytes, c.maxBytes)
	}
	if c.evictions == 0 {
		t.Error("expected at least one eviction")
	}
	if _, ok := c.items["evict-2.js.map"]; !ok {
		t.Error("most recent entry should still be cached")
	}
	if _, ok := c.items["evict-0.js.map"]; ok {
		t.Error("oldest entry should have been evicted")
	}
}
