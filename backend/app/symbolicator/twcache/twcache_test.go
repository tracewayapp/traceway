package twcache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func blobLoad(data []byte, calls *atomic.Int64) LoadFunc {
	return func(ctx context.Context) ([]byte, error) {
		if calls != nil {
			calls.Add(1)
		}
		return data, nil
	}
}

func caches(t *testing.T) map[string]*Cache {
	t.Helper()
	disk, err := NewDisk(t.TempDir(), 64<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]*Cache{
		"mem":  NewMem(100, 64<<20),
		"disk": disk,
	}
}

func TestGetBuildsThenHits(t *testing.T) {
	ctx := context.Background()
	for mode, c := range caches(t) {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int64
			data, done, err := c.Get(ctx, "k.tw", blobLoad([]byte("hello"), &calls))
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if string(data) != "hello" {
				t.Errorf("got %q, want hello", data)
			}
			done()

			data2, done2, err := c.Get(ctx, "k.tw", blobLoad([]byte("DIFFERENT"), &calls))
			if err != nil {
				t.Fatalf("Get (warm): %v", err)
			}
			if string(data2) != "hello" {
				t.Errorf("warm get: got %q, want the cached hello", data2)
			}
			done2()
			if calls.Load() != 1 {
				t.Errorf("expected load called once, got %d", calls.Load())
			}
		})
	}
}

func TestDiskPersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	c, err := NewDisk(dir, 64<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, done, err := c.Get(ctx, "k.tw", blobLoad([]byte("persisted"), nil))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "persisted" {
		t.Fatalf("got %q", data)
	}
	done()

	c2, err := NewDisk(dir, 64<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	failLoad := func(ctx context.Context) ([]byte, error) { return nil, errors.New("should not build") }
	data2, done2, err := c2.Get(ctx, "k.tw", failLoad)
	if err != nil {
		t.Fatalf("restart get: %v", err)
	}
	if string(data2) != "persisted" {
		t.Errorf("restart got %q, want persisted", data2)
	}
	done2()
}

func TestSingleflight(t *testing.T) {
	ctx := context.Background()
	for mode, c := range caches(t) {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int64
			release := make(chan struct{})
			load := func(ctx context.Context) ([]byte, error) {
				calls.Add(1)
				<-release
				return []byte("v"), nil
			}
			const n = 16
			var wg sync.WaitGroup
			start := make(chan struct{})
			for range n {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					_, done, err := c.Get(ctx, "k.tw", load)
					if err == nil {
						done()
					}
				}()
			}
			close(start)
			close(release)
			wg.Wait()
			if calls.Load() != 1 {
				t.Errorf("expected 1 build for concurrent gets, got %d", calls.Load())
			}
		})
	}
}

func TestNegativeAndInvalidate(t *testing.T) {
	ctx := context.Background()
	for mode, c := range caches(t) {
		t.Run(mode, func(t *testing.T) {
			boom := errors.New("boom")
			if _, _, err := c.Get(ctx, "k.tw", func(ctx context.Context) ([]byte, error) { return nil, boom }); err == nil {
				t.Fatal("expected error")
			}
			if !c.IsNegative("k.tw") {
				t.Error("expected negative entry after a failed load")
			}

			var calls atomic.Int64
			_, done, err := c.Get(ctx, "k.tw", blobLoad([]byte("v"), &calls))
			if err != nil {
				t.Fatal(err)
			}
			done()
			if c.IsNegative("k.tw") {
				t.Error("successful load should clear the negative entry")
			}

			c.Invalidate("k.tw")
			_, done2, err := c.Get(ctx, "k.tw", blobLoad([]byte("v"), &calls))
			if err != nil {
				t.Fatal(err)
			}
			done2()
			if calls.Load() != 2 {
				t.Errorf("invalidate should force a rebuild: got %d builds, want 2", calls.Load())
			}
		})
	}
}
