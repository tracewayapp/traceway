package twcache

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

func blobLoad(data []byte, calls *atomic.Int64) LoadFunc[[]byte] {
	return func(ctx context.Context) ([]byte, error) {
		if calls != nil {
			calls.Add(1)
		}
		return data, nil
	}
}

func caches(t *testing.T) map[string]*Cache[[]byte] {
	t.Helper()
	disk, err := NewDisk(t.TempDir(), 64<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]*Cache[[]byte]{
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

func TestGetSurvivesEvictionRace(t *testing.T) {
	ctx := context.Background()
	c := NewMem(1<<20, 48<<10)
	payload := make([]byte, 4<<10)
	const goroutines = 32
	const iters = 3000
	const keyspace = 64
	var fails atomic.Int64
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				key := "k" + strconv.Itoa((g*7+i)%keyspace) + ".tw"
				_, done, err := c.Get(ctx, key, blobLoad(payload, nil))
				if err != nil {
					fails.Add(1)
					continue
				}
				done()
			}
		}(g)
	}
	wg.Wait()
	if n := fails.Load(); n != 0 {
		t.Errorf("%d of %d Gets failed under eviction pressure; the bounded retry should resolve every present key", n, goroutines*iters)
	}
}

func TestMemFuncOnePoolAcrossKinds(t *testing.T) {
	type obj struct{ n int64 }
	weigh := func(v any) int64 {
		switch t := v.(type) {
		case []byte:
			return int64(len(t))
		case *obj:
			return t.n
		default:
			return 0
		}
	}
	ctx := context.Background()

	c := NewMemFunc[any](100, 1<<20, weigh)
	b, doneB, err := c.Get(ctx, "bytes", func(context.Context) (any, error) { return []byte("hello"), nil })
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := b.([]byte); !ok || string(got) != "hello" {
		t.Fatalf("bytes entry: got %#v", b)
	}
	doneB()

	o, doneO, err := c.Get(ctx, "obj", func(context.Context) (any, error) { return &obj{n: 42}, nil })
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := o.(*obj); !ok || got.n != 42 {
		t.Fatalf("obj entry: got %#v", o)
	}
	doneO()

	st := c.Stats()
	if st.Entries != 2 {
		t.Errorf("entries: got %d, want 2 (one pool holds both kinds)", st.Entries)
	}
	if want := int64(len("hello")) + 42; st.Bytes != want {
		t.Errorf("bytes: got %d, want %d (budget summed across kinds)", st.Bytes, want)
	}

	small := NewMemFunc[any](100, 50, weigh)
	if _, done, err := small.Get(ctx, "b", func(context.Context) (any, error) { return make([]byte, 40), nil }); err != nil {
		t.Fatal(err)
	} else {
		done()
	}
	if _, done, err := small.Get(ctx, "o", func(context.Context) (any, error) { return &obj{n: 40}, nil }); err != nil {
		t.Fatal(err)
	} else {
		done()
	}
	if s := small.Stats(); s.Entries != 1 || s.Bytes != 40 {
		t.Errorf("cross-kind eviction: got entries=%d bytes=%d, want entries=1 bytes=40", s.Entries, s.Bytes)
	}
}
