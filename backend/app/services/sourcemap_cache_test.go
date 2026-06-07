package services

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/storage"
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

	const n = 16
	results := make([]*parsedSourceMap, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Go(func() {
			<-start
			sm, err := smCache.getOrLoad(context.Background(), "singleflight-test.js.map")
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

	if _, err := smCache.getOrLoad(context.Background(), "singleflight-test.js.map"); err != nil {
		t.Fatal(err)
	}
	if got := cs.reads["singleflight-test.js.map"]; got != 1 {
		t.Errorf("expected cached lookup to not hit storage, got %d reads", got)
	}
}

func TestSourceMapCacheByteCapEviction(t *testing.T) {
	prev := storage.Store
	defer func() { storage.Store = prev }()
	cs := &countingStorage{reads: map[string]int{}, data: map[string][]byte{}}
	storage.Store = cs

	c := &sourceMapCache{
		items:      make(map[string]*list.Element),
		order:      list.New(),
		loading:    make(map[string]*sourceMapLoad),
		maxEntries: 10,
		maxBytes:   2000,
	}

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
