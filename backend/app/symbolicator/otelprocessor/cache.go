package otelprocessor

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/tracewayapp/traceway/backend/app/symbolicator"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/twcache"
)

const negativeCacheTTL = time.Minute
const negativeCacheMaxKeys = 10000

type buildFunc func(ctx context.Context) (*symbolicator.Resolver, error)

type resolverCache struct {
	mu         sync.Mutex
	entries    map[string]*list.Element
	order      *list.List
	maxEntries int
	loading    map[string]*resolverLoad
	negative   map[string]negativeEntry
	disk       *twcache.Cache
}

type cacheEntry struct {
	key      string
	resolver *symbolicator.Resolver
}

type resolverLoad struct {
	done     chan struct{}
	resolver *symbolicator.Resolver
	err      error
}

type negativeEntry struct {
	expiresAt time.Time
	err       error
}

func newResolverCache(cfg *Config) (*resolverCache, error) {
	c := &resolverCache{
		entries:    make(map[string]*list.Element),
		order:      list.New(),
		maxEntries: cfg.SourceMapCacheSize,
		loading:    make(map[string]*resolverLoad),
		negative:   make(map[string]negativeEntry),
	}
	if cfg.CacheDir != "" {
		disk, err := newDiskCache(cfg.CacheDir, int64(cfg.CacheMaxMB)<<20, cfg.CacheMaxDiskPct)
		if err != nil {
			return nil, err
		}
		c.disk = disk
	}
	return c, nil
}

func (c *resolverCache) get(ctx context.Context, key string, build buildFunc) (*symbolicator.Resolver, error) {
	c.mu.Lock()
	if el, ok := c.entries[key]; ok {
		c.order.MoveToFront(el)
		r := el.Value.(*cacheEntry).resolver
		c.mu.Unlock()
		return r, nil
	}
	if n, ok := c.negative[key]; ok {
		if time.Now().Before(n.expiresAt) {
			c.mu.Unlock()
			return nil, n.err
		}
		delete(c.negative, key)
	}
	if l, ok := c.loading[key]; ok {
		c.mu.Unlock()
		<-l.done
		return l.resolver, l.err
	}
	l := &resolverLoad{done: make(chan struct{})}
	c.loading[key] = l
	c.mu.Unlock()

	l.resolver, l.err = c.load(ctx, key, build)

	c.mu.Lock()
	delete(c.loading, key)
	if l.err == nil && l.resolver != nil {
		el := c.order.PushFront(&cacheEntry{key: key, resolver: l.resolver})
		c.entries[key] = el
		for c.order.Len() > c.maxEntries {
			back := c.order.Back()
			evicted := c.order.Remove(back).(*cacheEntry)
			delete(c.entries, evicted.key)
		}
	} else if l.err != nil {
		if len(c.negative) >= negativeCacheMaxKeys {
			now := time.Now()
			for k, n := range c.negative {
				if now.After(n.expiresAt) {
					delete(c.negative, k)
				}
			}
		}
		if len(c.negative) < negativeCacheMaxKeys {
			c.negative[key] = negativeEntry{expiresAt: time.Now().Add(negativeCacheTTL), err: l.err}
		}
	}
	c.mu.Unlock()
	close(l.done)
	return l.resolver, l.err
}

func (c *resolverCache) load(ctx context.Context, key string, build buildFunc) (*symbolicator.Resolver, error) {
	if c.disk == nil {
		return build(ctx)
	}
	name := diskNameFor(key)
	if r, err := c.disk.Open(name); err == nil {
		return r, nil
	}
	r, err := build(ctx)
	if err != nil {
		return nil, err
	}
	if cached, werr := c.disk.Write(name, r.MarshalTW()); werr == nil {
		return cached, nil
	}
	return r, nil
}

func diskNameFor(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:]) + ".tw"
}

func newDiskCache(dir string, maxBytes int64, maxPct int) (*twcache.Cache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache_dir: %w", err)
	}
	if maxPct > 0 {
		total, err := twcache.DiskCapacityBytes(dir)
		if err != nil {
			return nil, fmt.Errorf("cache_max_disk_pct requires disk capacity detection: %w", err)
		}
		pctBytes := total / 100 * int64(maxPct)
		if maxBytes <= 0 || pctBytes < maxBytes {
			maxBytes = pctBytes
		}
	}
	if maxBytes <= 0 {
		return nil, fmt.Errorf("the source map cache requires a positive byte cap (cache_max_mb or cache_max_disk_pct)")
	}
	cache, err := twcache.New(dir, maxBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache_dir: %w", err)
	}
	return cache, nil
}
