package twcache

import (
	"container/list"
	"errors"
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

const mtimeRefreshInterval = 10 * time.Minute

var ErrInvalidName = errors.New("twcache: cache name escapes the cache directory")

type Cache struct {
	dir  string
	warn func(error)

	mu        sync.Mutex
	files     map[string]*list.Element
	order     *list.List
	maxBytes  int64
	curBytes  int64
	hits      uint64
	evictions uint64
}

type entry struct {
	name      string
	size      int64
	touchedAt time.Time
}

type Stats struct {
	Entries   int
	Bytes     int64
	MaxBytes  int64
	Hits      uint64
	Evictions uint64
}

func New(dir string, maxBytes int64, warn func(error)) (*Cache, error) {
	if maxBytes <= 0 {
		return nil, errors.New("twcache: maxBytes must be positive")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create tw cache dir: %w", err)
	}
	c := &Cache{
		dir:      dir,
		warn:     warn,
		files:    make(map[string]*list.Element),
		order:    list.New(),
		maxBytes: maxBytes,
	}
	if err := c.scan(); err != nil {
		return nil, fmt.Errorf("failed to scan tw cache dir: %w", err)
	}
	return c, nil
}

func (c *Cache) Dir() string {
	return c.dir
}

func (c *Cache) path(name string) (string, error) {
	rel := filepath.FromSlash(name)
	if !filepath.IsLocal(rel) {
		return "", ErrInvalidName
	}
	return filepath.Join(c.dir, rel), nil
}

func (c *Cache) scan() error {
	type scanned struct {
		name  string
		size  int64
		mtime time.Time
	}
	var found []scanned
	err := filepath.WalkDir(c.dir, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			if c.warn != nil {
				c.warn(fmt.Errorf("skipping unreadable tw cache entry (path=%s): %w", path, err))
			}
			return nil
		}
		if dirEntry.IsDir() || !strings.HasSuffix(path, ".tw") {
			return nil
		}
		info, err := dirEntry.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(c.dir, path)
		if err != nil {
			return nil
		}
		found = append(found, scanned{name: filepath.ToSlash(rel), size: info.Size(), mtime: info.ModTime()})
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(found, func(i, j int) bool { return found[i].mtime.Before(found[j].mtime) })
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, f := range found {
		el := c.order.PushFront(&entry{name: f.name, size: f.size})
		c.files[f.name] = el
		c.curBytes += f.size
	}
	c.evictLocked()
	return nil
}

// Open mmaps the named .tw file and returns a resolver backed by it. The
// mapping is released by a runtime cleanup once the resolver is collected.
// Corrupt files are removed so the caller's rebuild can replace them.
func (c *Cache) Open(name string) (*symbolicator.Resolver, error) {
	return c.open(name, true)
}

func (c *Cache) open(name string, countHit bool) (*symbolicator.Resolver, error) {
	path, err := c.path(name)
	if err != nil {
		return nil, err
	}
	data, unmap, err := mmapFile(path)
	if err != nil {
		return nil, err
	}
	resolver, err := symbolicator.OpenTW(data)
	if err != nil {
		unmap()
		c.Remove(name)
		return nil, err
	}
	runtime.AddCleanup(resolver, func(u func()) { u() }, unmap)
	c.noteUse(name, int64(len(data)), countHit)
	return resolver, nil
}

// Write atomically persists data as the named .tw file and returns a
// resolver mmapped from it. Write does not count as a cache hit.
func (c *Cache) Write(name string, data []byte) (*symbolicator.Resolver, error) {
	path, err := c.path(name)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tw-*")
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
	return c.open(name, false)
}

func (c *Cache) noteUse(name string, size int64, countHit bool) {
	now := time.Now()
	updateMtime := false
	c.mu.Lock()
	if countHit {
		c.hits++
	}
	if el, ok := c.files[name]; ok {
		e := el.Value.(*entry)
		c.curBytes += size - e.size
		e.size = size
		c.order.MoveToFront(el)
		if now.Sub(e.touchedAt) > mtimeRefreshInterval {
			e.touchedAt = now
			updateMtime = true
		}
	} else {
		c.files[name] = c.order.PushFront(&entry{name: name, size: size, touchedAt: now})
		c.curBytes += size
	}
	c.evictLocked()
	c.mu.Unlock()
	if updateMtime {
		if path, err := c.path(name); err == nil {
			_ = os.Chtimes(path, now, now)
		}
	}
}

func (c *Cache) Remove(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.files[name]; ok {
		c.dropLocked(el)
	} else if path, err := c.path(name); err == nil {
		os.Remove(path)
	}
}

func (c *Cache) dropLocked(el *list.Element) {
	e := c.order.Remove(el).(*entry)
	delete(c.files, e.name)
	c.curBytes -= e.size
	os.Remove(filepath.Join(c.dir, filepath.FromSlash(e.name)))
}

func (c *Cache) evictLocked() {
	for c.curBytes > c.maxBytes {
		back := c.order.Back()
		if back == nil {
			break
		}
		c.dropLocked(back)
		c.evictions++
	}
}

func (c *Cache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{
		Entries:   c.order.Len(),
		Bytes:     c.curBytes,
		MaxBytes:  c.maxBytes,
		Hits:      c.hits,
		Evictions: c.evictions,
	}
}
