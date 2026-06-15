package twcache

import (
	"container/list"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const mtimeRefreshInterval = 10 * time.Minute

type twcachedisk struct {
	dirPath string
	warn    func(error)

	mu        sync.Mutex
	files     map[string]*list.Element
	order     *list.List
	maxBytes  int64
	curBytes  int64
	evictions uint64
}

type diskEntry struct {
	name      string
	size      int64
	touchedAt time.Time
}

func newDiskStore(dir string, maxBytes int64, warn func(error)) (*twcachedisk, error) {
	if maxBytes <= 0 {
		return nil, errors.New("twcache: maxBytes must be positive")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create tw cache dir: %w", err)
	}
	s := &twcachedisk{
		dirPath:  dir,
		warn:     warn,
		files:    make(map[string]*list.Element),
		order:    list.New(),
		maxBytes: maxBytes,
	}
	if err := s.scan(); err != nil {
		return nil, fmt.Errorf("failed to scan tw cache dir: %w", err)
	}
	return s, nil
}

func (s *twcachedisk) dir() string { return s.dirPath }

func (s *twcachedisk) path(name string) (string, error) {
	rel := filepath.FromSlash(name)
	if !filepath.IsLocal(rel) {
		return "", ErrInvalidName
	}
	return filepath.Join(s.dirPath, rel), nil
}

func (s *twcachedisk) scan() error {
	type scanned struct {
		name  string
		size  int64
		mtime time.Time
	}
	var found []scanned
	err := filepath.WalkDir(s.dirPath, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			if s.warn != nil {
				s.warn(fmt.Errorf("skipping unreadable tw cache entry (path=%s): %w", path, err))
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
		rel, err := filepath.Rel(s.dirPath, path)
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
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range found {
		el := s.order.PushFront(&diskEntry{name: f.name, size: f.size})
		s.files[f.name] = el
		s.curBytes += f.size
	}
	s.evictLocked()
	return nil
}

func (s *twcachedisk) get(name string) ([]byte, func(), bool) {
	path, err := s.path(name)
	if err != nil {
		return nil, noop, false
	}
	data, unmap, err := mmapFile(path)
	if err != nil {
		return nil, noop, false
	}
	s.noteUse(name, int64(len(data)))
	return data, unmap, true
}

func (s *twcachedisk) contains(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.files[name]
	return ok
}

func (s *twcachedisk) put(name string, data []byte) error {
	if err := s.persist(name, data); err != nil {
		return err
	}
	s.noteUse(name, int64(len(data)))
	return nil
}

func (s *twcachedisk) persist(name string, data []byte) error {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tw-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

func (s *twcachedisk) noteUse(name string, size int64) {
	now := time.Now()
	updateMtime := false
	s.mu.Lock()
	if el, ok := s.files[name]; ok {
		e := el.Value.(*diskEntry)
		s.curBytes += size - e.size
		e.size = size
		s.order.MoveToFront(el)
		if now.Sub(e.touchedAt) > mtimeRefreshInterval {
			e.touchedAt = now
			updateMtime = true
		}
	} else {
		s.files[name] = s.order.PushFront(&diskEntry{name: name, size: size, touchedAt: now})
		s.curBytes += size
	}
	s.evictLocked()
	s.mu.Unlock()
	if updateMtime {
		if path, err := s.path(name); err == nil {
			_ = os.Chtimes(path, now, now)
		}
	}
}

func (s *twcachedisk) remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if el, ok := s.files[name]; ok {
		s.dropLocked(el)
	} else if path, err := s.path(name); err == nil {
		os.Remove(path)
	}
}

func (s *twcachedisk) setLimits(_ int, maxBytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if maxBytes > 0 {
		s.maxBytes = maxBytes
	}
	s.evictLocked()
}

func (s *twcachedisk) dropLocked(el *list.Element) {
	e := s.order.Remove(el).(*diskEntry)
	delete(s.files, e.name)
	s.curBytes -= e.size
	os.Remove(filepath.Join(s.dirPath, filepath.FromSlash(e.name)))
}

func (s *twcachedisk) evictLocked() {
	for s.curBytes > s.maxBytes {
		back := s.order.Back()
		if back == nil {
			break
		}
		s.dropLocked(back)
		s.evictions++
	}
}

func (s *twcachedisk) stats() storeStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return storeStats{
		Mode:      "disk",
		Entries:   s.order.Len(),
		Bytes:     s.curBytes,
		MaxBytes:  s.maxBytes,
		Evictions: s.evictions,
	}
}
