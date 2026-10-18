package home

import (
	"context"
	"sync"
	"testing"
	"time"
)

type memCache struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (c *memCache) Get(_ context.Context, k string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[k]
	return v, ok, nil
}

func (c *memCache) Set(_ context.Context, k string, v []byte, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[k] = v
	return nil
}

type countingBuilder struct {
	mu    sync.Mutex
	calls int
}

func (b *countingBuilder) Build(context.Context) (*Feed, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	return &Feed{Banners: []Row{{"id": 1}}}, nil
}

func TestFeedIsCachedAfterFirstBuild(t *testing.T) {
	b := &countingBuilder{}
	s := &Service{Cache: &memCache{m: map[string][]byte{}}, Builder: b}
	for i := 0; i < 3; i++ {
		f, err := s.Feed(context.Background())
		if err != nil || len(f.Banners) != 1 {
			t.Fatalf("feed %v %v", f, err)
		}
	}
	if b.calls != 1 {
		t.Fatalf("built %d times, want 1", b.calls)
	}
}
