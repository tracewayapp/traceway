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

type LoadFunc func(ctx context.Context) ([]byte, error)

type store interface {
	get(name string) (data []byte, done func(), ok bool)

	contains(name string) bool
	put(name string, data []byte) error
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

type Cache struct {
	name  string
	store store
	warn  func(error)

	NotFound func(error) bool

	Validate func([]byte) bool

	mu                  sync.Mutex
	loading             map[string]*cacheLoad
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

type cacheLoad struct {
	done chan struct{}
	err  error
}

func newCache(s store, warn func(error)) *Cache {
	return &Cache{
		name:     "symbolication artifact",
		store:    s,
		warn:     warn,
		loading:  make(map[string]*cacheLoad),
		negative: make(map[string]*negEntry),
	}
}

func NewMem(maxEntries int, maxBytes int64) *Cache {
	return newCache(newMemStore(maxEntries, maxBytes), nil)
}

func NewDisk(dir string, maxBytes int64, warn func(error)) (*Cache, error) {
	s, err := newDiskStore(dir, maxBytes, warn)
	if err != nil {
		return nil, err
	}
	return newCache(s, warn), nil
}

func (c *Cache) SetWarn(warn func(error)) { c.warn = warn }

func (c *Cache) SetLimits(maxEntries int, maxBytes int64) {
	c.store.setLimits(maxEntries, maxBytes)
}

func (c *Cache) Dir() string { return c.store.dir() }

func (c *Cache) Get(ctx context.Context, key string, load LoadFunc) (data []byte, done func(), err error) {
	if data, done, ok := c.store.get(key); ok {
		if c.Validate == nil || c.Validate(data) {
			c.hits.Add(1)
			return data, done, nil
		}

		done()
		c.store.remove(key)
	}
	if err := c.ensureBuilt(ctx, key, load); err != nil {
		return nil, noop, err
	}
	if data, done, ok := c.store.get(key); ok {
		return data, done, nil
	}
	return nil, noop, fmt.Errorf("%s: %q evicted before use", c.name, key)
}

func (c *Cache) ensureBuilt(ctx context.Context, key string, load LoadFunc) error {
	c.mu.Lock()
	if l, ok := c.loading[key]; ok {
		c.mu.Unlock()
		<-l.done
		if l.err == nil {
			c.hits.Add(1)
		}
		return l.err
	}

	if c.store.contains(key) {
		c.mu.Unlock()
		c.hits.Add(1)
		return nil
	}
	c.misses.Add(1)
	l := &cacheLoad{done: make(chan struct{})}
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
	if l.err != nil && !c.isNotFound(l.err) {
		c.reportFailure(l.err)
	}
	return l.err
}

func (c *Cache) isNotFound(err error) bool {
	return c.NotFound == nil || c.NotFound(err)
}

func (c *Cache) IsNegative(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.negative[key]
	if !ok || time.Now().After(e.expiresAt) {
		return false
	}
	c.negativeHits++
	return true
}

func (c *Cache) Invalidate(key string) {
	c.mu.Lock()
	delete(c.negative, key)
	c.mu.Unlock()
	c.store.remove(key)
}

func (c *Cache) markNegativeLocked(key string, loadErr error) {
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

func (c *Cache) pruneNegativeLocked() {
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

func (c *Cache) reportFailure(err error) {
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

func (c *Cache) Stats() Stats {
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
