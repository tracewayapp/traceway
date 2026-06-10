package services

import (
	"container/list"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tracewayapp/traceway/backend/app/symbolicator"

	traceway "go.tracewayapp.com"
)

type sourceMapDiskCache struct {
	mem      *sourceMapCache
	dir      string
	mu       sync.Mutex
	files    map[string]*list.Element
	order    *list.List
	maxBytes int64
	curBytes int64

	diskHits      atomic.Uint64
	diskEvictions uint64
}

const diskCacheMtimeInterval = 10 * time.Minute

type diskCacheEntry struct {
	key       string
	size      int64
	touchedAt time.Time
}

func EnableSourceMapDiskCache(dir string, maxBytes int64) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create source map disk cache dir: %w", err)
	}
	d := &sourceMapDiskCache{
		mem:      smCache,
		dir:      dir,
		files:    make(map[string]*list.Element),
		order:    list.New(),
		maxBytes: maxBytes,
	}
	if err := d.scan(); err != nil {
		return fmt.Errorf("failed to scan source map disk cache dir: %w", err)
	}
	activeSMCache = d
	return nil
}

func twKeyFor(mapKey string) string {
	return strings.TrimSuffix(mapKey, ".map") + ".tw"
}

func (d *sourceMapDiskCache) pathFor(mapKey string) (string, bool) {
	rel := filepath.FromSlash(twKeyFor(mapKey))
	if !filepath.IsLocal(rel) {
		return "", false
	}
	return filepath.Join(d.dir, rel), true
}

func (d *sourceMapDiskCache) scan() error {
	type scanned struct {
		key   string
		size  int64
		mtime time.Time
	}
	var found []scanned
	err := filepath.WalkDir(d.dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			traceway.CaptureException(fmt.Errorf("skipping unreadable tw cache entry (path=%s): %w", path, err))
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".tw") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(d.dir, path)
		if err != nil {
			return nil
		}
		found = append(found, scanned{key: filepath.ToSlash(rel), size: info.Size(), mtime: info.ModTime()})
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(found, func(i, j int) bool { return found[i].mtime.Before(found[j].mtime) })
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, f := range found {
		el := d.order.PushFront(&diskCacheEntry{key: f.key, size: f.size})
		d.files[f.key] = el
		d.curBytes += f.size
	}
	d.evictLocked()
	return nil
}

func (d *sourceMapDiskCache) getOrBuild(ctx context.Context, key string, build resolverBuild) (*symbolicator.Resolver, error) {
	return d.mem.getOrBuild(ctx, key, func(ctx context.Context) (*symbolicator.Resolver, int64, error) {
		return d.load(ctx, key, build)
	})
}

func (d *sourceMapDiskCache) load(ctx context.Context, mapKey string, build resolverBuild) (*symbolicator.Resolver, int64, error) {
	path, ok := d.pathFor(mapKey)
	if !ok {
		return build(ctx)
	}

	if r, size, err := d.openLocal(mapKey, path); err == nil {
		d.diskHits.Add(1)
		return r, size, nil
	}

	resolver, size, err := build(ctx)
	if err != nil {
		return nil, 0, err
	}
	if r, msize, werr := d.writeAndOpen(mapKey, path, resolver.MarshalTW()); werr == nil {
		return r, msize, nil
	} else {
		traceway.CaptureException(fmt.Errorf("failed to write tw cache file (key=%s): %w", mapKey, werr))
	}
	return resolver, size, nil
}

func (d *sourceMapDiskCache) openLocal(mapKey, path string) (*symbolicator.Resolver, int64, error) {
	data, unmap, err := mmapFile(path)
	if err != nil {
		return nil, 0, err
	}
	resolver, err := symbolicator.OpenTW(data)
	if err != nil {
		unmap()
		d.remove(mapKey)
		return nil, 0, err
	}
	runtime.AddCleanup(resolver, func(u func()) { u() }, unmap)
	d.touch(mapKey, int64(len(data)))
	return resolver, resolver.ApproxSize(), nil
}

func (d *sourceMapDiskCache) writeAndOpen(mapKey, path string, data []byte) (*symbolicator.Resolver, int64, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, 0, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tw-*")
	if err != nil {
		return nil, 0, err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, 0, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return nil, 0, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return nil, 0, err
	}
	return d.openLocal(mapKey, path)
}

func (d *sourceMapDiskCache) touch(mapKey string, size int64) {
	key := twKeyFor(mapKey)
	now := time.Now()
	updateMtime := false
	d.mu.Lock()
	if el, ok := d.files[key]; ok {
		entry := el.Value.(*diskCacheEntry)
		d.curBytes += size - entry.size
		entry.size = size
		d.order.MoveToFront(el)
		if now.Sub(entry.touchedAt) > diskCacheMtimeInterval {
			entry.touchedAt = now
			updateMtime = true
		}
	} else {
		d.files[key] = d.order.PushFront(&diskCacheEntry{key: key, size: size, touchedAt: now})
		d.curBytes += size
	}
	d.evictLocked()
	d.mu.Unlock()
	if updateMtime {
		if path, ok := d.pathFor(mapKey); ok {
			_ = os.Chtimes(path, now, now)
		}
	}
}

func (d *sourceMapDiskCache) remove(mapKey string) {
	key := twKeyFor(mapKey)
	d.mu.Lock()
	defer d.mu.Unlock()
	if el, ok := d.files[key]; ok {
		d.dropLocked(el)
	} else if path, ok := d.pathFor(mapKey); ok {
		os.Remove(path)
	}
}

func (d *sourceMapDiskCache) dropLocked(el *list.Element) {
	entry := d.order.Remove(el).(*diskCacheEntry)
	delete(d.files, entry.key)
	d.curBytes -= entry.size
	os.Remove(filepath.Join(d.dir, filepath.FromSlash(entry.key)))
}

func (d *sourceMapDiskCache) evictLocked() {
	for d.curBytes > d.maxBytes {
		back := d.order.Back()
		if back == nil {
			break
		}
		d.dropLocked(back)
		d.diskEvictions++
	}
}

func (d *sourceMapDiskCache) isNegative(key string) bool {
	return d.mem.isNegative(key)
}

func (d *sourceMapDiskCache) invalidate(key string) {
	d.mem.invalidate(key)
	d.remove(key)
}

func (d *sourceMapDiskCache) stats() SourceMapCacheStats {
	s := d.mem.stats()
	d.mu.Lock()
	defer d.mu.Unlock()
	s.DiskEnabled = true
	s.DiskEntries = d.order.Len()
	s.DiskBytes = d.curBytes
	s.DiskMaxBytes = d.maxBytes
	s.DiskHits = d.diskHits.Load()
	s.DiskEvictions = d.diskEvictions
	return s
}
