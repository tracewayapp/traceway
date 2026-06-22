package twcache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	negativeBaseTTL      = time.Minute
	transientNegativeTTL = 15 * time.Second
	negativeMaxTTL       = 15 * time.Minute
	negativeMaxKeys      = 10000
	failReportInterval   = time.Minute
)

var ErrInvalidName = errors.New("twcache: cache name escapes the cache directory")

type LoadFunc[V any] func(ctx context.Context) (V, error)

type store[V any] interface {
	get(name string) (data V, done func(), ok bool)

	contains(name string) bool
	put(name string, data V) error
	remove(name string)
	setLimits(maxEntries int, maxBytes int64)
	stats() storeStats
	dir() string
}

type storeStats struct {
	Mode       string
	Entries    int
	Bytes      int64
	MaxBytes   int64
	MaxEntries int
	Evictions  uint64
}

func noop() {}

type Cache[V any] struct {
	name  string
	store store[V]
	warn  func(error)

	NotFound func(error) bool

	Validate func(V) bool

	mu                  sync.Mutex
	loading             map[string]*cacheLoad[V]
	negative            map[string]*negEntry
	negativeHits        uint64
	lastParseMs         float64
	failuresSinceReport uint64
	lastFailAt          time.Time

	hits     atomic.Uint64
	misses   atomic.Uint64
	failures atomic.Uint64
	notFound atomic.Uint64
}

type negEntry struct {
	expiresAt time.Time
	failures  uint32
}

type cacheLoad[V any] struct {
	done chan struct{}
	err  error
	blob V
}

func newCache[V any](s store[V], warn func(error)) *Cache[V] {
	return &Cache[V]{
		name:     "symbolication artifact",
		store:    s,
		warn:     warn,
		loading:  make(map[string]*cacheLoad[V]),
		negative: make(map[string]*negEntry),
	}
}

func NewMem(maxEntries int, maxBytes int64) *Cache[[]byte] {
	return newCache(newMemStore(maxEntries, maxBytes, func(b []byte) int64 { return int64(len(b)) }), nil)
}

func NewMemFunc[V any](maxEntries int, maxBytes int64, weigh func(V) int64) *Cache[V] {
	return newCache(newMemStore(maxEntries, maxBytes, weigh), nil)
}

func NewDisk(dir string, maxBytes int64, warn func(error)) (*Cache[[]byte], error) {
	s, err := newDiskStore(dir, maxBytes, warn)
	if err != nil {
		return nil, err
	}
	return newCache(s, warn), nil
}

type anyDiskStore struct{ inner *twcachedisk }

func (a anyDiskStore) get(name string) (any, func(), bool) {
	b, done, ok := a.inner.get(name)
	if !ok {
		return nil, noop, false
	}
	return b, done, true
}

func (a anyDiskStore) contains(name string) bool { return a.inner.contains(name) }

func (a anyDiskStore) put(name string, data any) error {
	b, ok := data.([]byte)
	if !ok {
		return nil
	}
	return a.inner.put(name, b)
}

func (a anyDiskStore) remove(name string) { a.inner.remove(name) }
func (a anyDiskStore) setLimits(maxEntries int, maxBytes int64) {
	a.inner.setLimits(maxEntries, maxBytes)
}
func (a anyDiskStore) stats() storeStats { return a.inner.stats() }
func (a anyDiskStore) dir() string       { return a.inner.dir() }

func NewDiskAny(dir string, maxBytes int64, warn func(error)) (*Cache[any], error) {
	s, err := newDiskStore(dir, maxBytes, warn)
	if err != nil {
		return nil, err
	}
	return newCache(anyDiskStore{inner: s}, warn), nil
}

func (c *Cache[V]) SetWarn(warn func(error)) { c.warn = warn }

func (c *Cache[V]) SetLimits(maxEntries int, maxBytes int64) {
	c.store.setLimits(maxEntries, maxBytes)
}

func (c *Cache[V]) Dir() string { return c.store.dir() }

func (c *Cache[V]) Get(ctx context.Context, key string, load LoadFunc[V]) (V, func(), error) {
	if data, done, ok := c.store.get(key); ok {
		if c.Validate == nil || c.Validate(data) {
			c.hits.Add(1)
			return data, done, nil
		}

		done()
		c.store.remove(key)
	}
	return c.ensureBuilt(ctx, key, load)
}

func (c *Cache[V]) ensureBuilt(ctx context.Context, key string, load LoadFunc[V]) (V, func(), error) {
	var zero V
	c.mu.Lock()
	if l, ok := c.loading[key]; ok {
		c.mu.Unlock()
		<-l.done
		if l.err != nil {
			return zero, noop, l.err
		}
		c.hits.Add(1)
		return l.blob, noop, nil
	}

	if data, done, ok := c.store.get(key); ok {
		c.mu.Unlock()
		c.hits.Add(1)
		return data, done, nil
	}
	c.misses.Add(1)
	l := &cacheLoad[V]{done: make(chan struct{})}
	c.loading[key] = l
	c.mu.Unlock()

	var ms float64
	func() {
		defer func() {
			if r := recover(); r != nil {
				l.err = fmt.Errorf("%s load panicked (key=%s): %v", c.name, key, r)
			}
		}()
		start := time.Now()
		blob, lerr := load(ctx)
		ms = float64(time.Since(start).Microseconds()) / 1000.0
		if lerr != nil {
			l.err = lerr
			return
		}
		l.blob = blob
		l.err = c.store.put(key, blob)
	}()

	c.mu.Lock()
	delete(c.loading, key)
	if l.err == nil {
		c.lastParseMs = ms
		delete(c.negative, key)
	} else {
		c.markNegativeLocked(key, l.err)
	}
	c.mu.Unlock()

	close(l.done)
	if l.err != nil {
		if !c.isNotFound(l.err) {
			c.reportFailure(l.err)
		}
		return zero, noop, l.err
	}
	return l.blob, noop, nil
}

func (c *Cache[V]) isNotFound(err error) bool {
	return c.NotFound == nil || c.NotFound(err)
}

func (c *Cache[V]) IsNegative(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.negative[key]
	if !ok || time.Now().After(e.expiresAt) {
		return false
	}
	c.negativeHits++
	return true
}

func (c *Cache[V]) Invalidate(key string) {
	c.mu.Lock()
	delete(c.negative, key)
	c.mu.Unlock()
	c.store.remove(key)
}

func (c *Cache[V]) markNegativeLocked(key string, loadErr error) {
	base := transientNegativeTTL
	if c.isNotFound(loadErr) {
		base = negativeBaseTTL
		c.notFound.Add(1)
	} else {
		c.failures.Add(1)
	}
	e := c.negative[key]
	if e == nil {
		if len(c.negative) >= negativeMaxKeys {
			c.pruneNegativeLocked()
		}
		e = &negEntry{}
		c.negative[key] = e
	}
	ttl := min(base<<min(e.failures, 16), negativeMaxTTL)
	e.failures++
	e.expiresAt = time.Now().Add(ttl)
}

func (c *Cache[V]) pruneNegativeLocked() {
	now := time.Now()
	for k, e := range c.negative {
		if now.After(e.expiresAt) {
			delete(c.negative, k)
		}
	}
	for k := range c.negative {
		if len(c.negative) < negativeMaxKeys {
			break
		}
		delete(c.negative, k)
	}
}

func (c *Cache[V]) reportFailure(err error) {
	var report uint64
	c.mu.Lock()
	c.failuresSinceReport++
	if time.Since(c.lastFailAt) >= failReportInterval {
		report = c.failuresSinceReport
		c.failuresSinceReport = 0
		c.lastFailAt = time.Now()
	}
	c.mu.Unlock()
	if report > 0 && c.warn != nil {
		c.warn(fmt.Errorf("%s loads failed %d time(s) since last report: %w", c.name, report, err))
	}
}

type Stats struct {
	Mode            string
	Entries         int
	Bytes           int64
	MaxBytes        int64
	MaxEntries      int
	Hits            uint64
	Misses          uint64
	Evictions       uint64
	Failures        uint64
	NotFound        uint64
	NegativeHits    uint64
	NegativeEntries int
	LastParseMs     float64
}

func (c *Cache[V]) Stats() Stats {
	ss := c.store.stats()
	c.mu.Lock()
	negEntries := len(c.negative)
	negHits := c.negativeHits
	lastMs := c.lastParseMs
	c.mu.Unlock()
	return Stats{
		Mode:            ss.Mode,
		Entries:         ss.Entries,
		Bytes:           ss.Bytes,
		MaxBytes:        ss.MaxBytes,
		MaxEntries:      ss.MaxEntries,
		Evictions:       ss.Evictions,
		Hits:            c.hits.Load(),
		Misses:          c.misses.Load(),
		Failures:        c.failures.Load(),
		NotFound:        c.notFound.Load(),
		NegativeHits:    negHits,
		NegativeEntries: negEntries,
		LastParseMs:     lastMs,
	}
}
