package services

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/storage"

	"github.com/google/uuid"
)

func newTestSourceMapCache(maxEntries int, maxBytes int64) *sourceMapCache {
	return &sourceMapCache{
		items:      make(map[string]*list.Element),
		order:      list.New(),
		loading:    make(map[string]*sourceMapLoad),
		maxEntries: maxEntries,
		maxBytes:   maxBytes,
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
		return nil, errors.New("not found")
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
	results := make([]*parsedSourceMap, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Go(func() {
			<-start
			sm, err := c.getOrLoad(context.Background(), "singleflight-test.js.map")
			if err != nil {
				t.Error(err)
				return
			}
			results[i] = sm
		})
	}
	close(start)
	wg.Wait()

	if got := cs.reads["singleflight-test.js.map"]; got != 1 {
		t.Errorf("expected 1 storage read for concurrent lookups, got %d", got)
	}
	for i := 1; i < n; i++ {
		if results[i] != results[0] {
			t.Fatal("expected all callers to share a single parsed instance")
		}
	}

	if _, err := c.getOrLoad(context.Background(), "singleflight-test.js.map"); err != nil {
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
	results := make([]*parsedSourceMap, keys*4)
	for i := range keys * 4 {
		wg.Go(func() {
			<-start
			sm, err := c.getOrLoad(context.Background(), fmt.Sprintf("distinct-%d.js.map", i%keys))
			if err != nil {
				t.Error(err)
				return
			}
			results[i] = sm
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

	if _, err := c.getOrLoad(context.Background(), "flaky.js.map"); err == nil {
		t.Fatal("expected first load to fail")
	}
	c.mu.Lock()
	if c.failures != 1 {
		t.Errorf("expected 1 recorded failure, got %d", c.failures)
	}
	c.mu.Unlock()
	sm, err := c.getOrLoad(context.Background(), "flaky.js.map")
	if err != nil || sm == nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if fs.reads != 2 {
		t.Errorf("expected 2 storage reads (fail then retry), got %d", fs.reads)
	}
	if _, err := c.getOrLoad(context.Background(), "flaky.js.map"); err != nil {
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
		sm  *parsedSourceMap
		err error
	}
	leader := make(chan result, 1)
	go func() {
		sm, err := c.getOrLoad(context.Background(), "boom.js.map")
		leader <- result{sm, err}
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
			sm, err := c.getOrLoad(context.Background(), "boom.js.map")
			waiters <- result{sm, err}
		}()
	}

	close(release)

	r := <-leader
	if r.err == nil || r.sm != nil {
		t.Fatalf("expected leader to receive panic error, got sm=%v err=%v", r.sm, r.err)
	}
	for range 2 {
		r := <-waiters
		if r.err == nil || r.sm != nil {
			t.Fatalf("expected waiter to receive an error, got sm=%v err=%v", r.sm, r.err)
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

	sourceMaps := []*models.SourceMap{{FileName: "dead.js.map", StorageKey: "dead.js.map"}}
	trace := "Error: boom\n    fn()\n    dead.js:1:10\n    fn2()\n    dead.js:1:20\n    fn3()\n    dead.js:1:30"

	resolved := ResolveStackTrace(context.Background(), uuid.New(), trace, sourceMaps)
	if resolved != trace {
		t.Error("trace should be stored as-is when its source map cannot be loaded")
	}
	if got := cs.reads["dead.js.map"]; got != 1 {
		t.Errorf("expected 1 load attempt per trace for a failing map, got %d", got)
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
		if _, err := c.getOrLoad(context.Background(), key); err != nil {
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
