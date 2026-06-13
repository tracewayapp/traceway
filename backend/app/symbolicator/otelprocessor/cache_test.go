package otelprocessor

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/symbolicator"
)

func peakBuildConcurrency(t *testing.T, maxConcurrent, keys int) int32 {
	t.Helper()
	cfg := createDefaultConfig().(*Config)
	cfg.MaxConcurrentBuilds = maxConcurrent
	cache, err := newResolverCache(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var inFlight, peak atomic.Int32
	build := func(ctx context.Context) (*symbolicator.Resolver, error) {
		cur := inFlight.Add(1)
		for {
			p := peak.Load()
			if cur <= p || peak.CompareAndSwap(p, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		inFlight.Add(-1)
		return &symbolicator.Resolver{}, nil
	}
	var wg sync.WaitGroup
	for i := range keys {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := cache.get(context.Background(), fmt.Sprintf("key-%d", i), build); err != nil {
				t.Errorf("get: %v", err)
			}
		}(i)
	}
	wg.Wait()
	return peak.Load()
}

func TestBuildSemaphoreBoundsConcurrency(t *testing.T) {
	const keys = 32
	if peak := peakBuildConcurrency(t, 4, keys); peak > 4 {
		t.Fatalf("bounded peak concurrency = %d, want <= 4", peak)
	}
	if peak := peakBuildConcurrency(t, 0, keys); peak <= 4 {
		t.Fatalf("unbounded peak concurrency = %d, want > 4", peak)
	}
}

func TestBuildSemaphoreRespectsContextCancellation(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaxConcurrentBuilds = 1
	cache, err := newResolverCache(cfg)
	if err != nil {
		t.Fatal(err)
	}

	release := make(chan struct{})
	started := make(chan struct{})
	go func() {
		_, _ = cache.get(context.Background(), "holder", func(ctx context.Context) (*symbolicator.Resolver, error) {
			close(started)
			<-release
			return &symbolicator.Resolver{}, nil
		})
	}()
	<-started

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = cache.get(ctx, "waiter", func(ctx context.Context) (*symbolicator.Resolver, error) {
		t.Error("build ran despite cancelled context")
		return &symbolicator.Resolver{}, nil
	})
	if err != context.Canceled {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	close(release)
}
