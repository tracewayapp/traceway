package sourcemapprocessor

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tracewayapp/traceway/backend/app/symbolicator"
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
	disk       *diskCache
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
	if r, err := c.disk.open(key); err == nil {
		return r, nil
	}
	r, err := build(ctx)
	if err != nil {
		return nil, err
	}
	if cached, werr := c.disk.write(key, r.MarshalTW()); werr == nil {
		return cached, nil
	}
	return r, nil
}

type diskCache struct {
	dir      string
	mu       sync.Mutex
	files    map[string]*list.Element
	order    *list.List
	maxBytes int64
	curBytes int64
}

type diskEntry struct {
	name string
	size int64
}

func newDiskCache(dir string, maxBytes int64, maxPct int) (*diskCache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache_dir: %w", err)
	}
	if maxPct > 0 {
		total, err := diskCapacityBytes(dir)
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
	d := &diskCache{
		dir:      dir,
		files:    make(map[string]*list.Element),
		order:    list.New(),
		maxBytes: maxBytes,
	}
	if err := d.scan(); err != nil {
		return nil, fmt.Errorf("failed to scan cache_dir: %w", err)
	}
	return d, nil
}

func (d *diskCache) nameFor(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:]) + ".tw"
}

func (d *diskCache) scan() error {
	type scanned struct {
		name  string
		size  int64
		mtime time.Time
	}
	var found []scanned
	err := filepath.WalkDir(d.dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".tw") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		found = append(found, scanned{name: entry.Name(), size: info.Size(), mtime: info.ModTime()})
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(found, func(i, j int) bool { return found[i].mtime.Before(found[j].mtime) })
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, f := range found {
		el := d.order.PushFront(&diskEntry{name: f.name, size: f.size})
		d.files[f.name] = el
		d.curBytes += f.size
	}
	d.evictLocked()
	return nil
}

func (d *diskCache) open(key string) (*symbolicator.Resolver, error) {
	name := d.nameFor(key)
	path := filepath.Join(d.dir, name)
	data, unmap, err := mmapFile(path)
	if err != nil {
		return nil, err
	}
	resolver, err := symbolicator.OpenTW(data)
	if err != nil {
		unmap()
		d.remove(name)
		return nil, err
	}
	runtime.AddCleanup(resolver, func(u func()) { u() }, unmap)
	d.touch(name, int64(len(data)))
	return resolver, nil
}

func (d *diskCache) write(key string, data []byte) (*symbolicator.Resolver, error) {
	name := d.nameFor(key)
	path := filepath.Join(d.dir, name)
	tmp, err := os.CreateTemp(d.dir, ".tw-*")
	if err != nil {
		return nil, err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return nil, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return nil, err
	}
	return d.open(key)
}

func (d *diskCache) touch(name string, size int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if el, ok := d.files[name]; ok {
		entry := el.Value.(*diskEntry)
		d.curBytes += size - entry.size
		entry.size = size
		d.order.MoveToFront(el)
	} else {
		d.files[name] = d.order.PushFront(&diskEntry{name: name, size: size})
		d.curBytes += size
	}
	d.evictLocked()
}

func (d *diskCache) remove(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if el, ok := d.files[name]; ok {
		d.dropLocked(el)
	} else {
		os.Remove(filepath.Join(d.dir, name))
	}
}

func (d *diskCache) dropLocked(el *list.Element) {
	entry := d.order.Remove(el).(*diskEntry)
	delete(d.files, entry.name)
	d.curBytes -= entry.size
	os.Remove(filepath.Join(d.dir, entry.name))
}

func (d *diskCache) evictLocked() {
	for d.curBytes > d.maxBytes {
		back := d.order.Back()
		if back == nil {
			break
		}
		d.dropLocked(back)
	}
}
