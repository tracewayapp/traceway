package twcache

import (
	"container/list"
	"sync"
)

type twcachemem struct {
	mu         sync.Mutex
	items      map[string]*list.Element
	order      *list.List
	maxEntries int
	maxBytes   int64
	curBytes   int64
	evictions  uint64
}

type memEntry struct {
	name string
	data []byte
	size int64
}

func newMemStore(maxEntries int, maxBytes int64) *twcachemem {
	return &twcachemem{
		items:      make(map[string]*list.Element),
		order:      list.New(),
		maxEntries: maxEntries,
		maxBytes:   maxBytes,
	}
}

func (s *twcachemem) get(name string) ([]byte, func(), bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	el, ok := s.items[name]
	if !ok {
		return nil, noop, false
	}
	s.order.MoveToFront(el)
	return el.Value.(*memEntry).data, noop, true
}

func (s *twcachemem) contains(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.items[name]
	return ok
}

func (s *twcachemem) put(name string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	size := int64(len(data))
	if el, ok := s.items[name]; ok {
		e := el.Value.(*memEntry)
		s.curBytes += size - e.size
		e.data = data
		e.size = size
		s.order.MoveToFront(el)
	} else {
		s.items[name] = s.order.PushFront(&memEntry{name: name, data: data, size: size})
		s.curBytes += size
	}
	s.evictLocked()
	return nil
}

func (s *twcachemem) remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if el, ok := s.items[name]; ok {
		s.dropLocked(el)
	}
}

func (s *twcachemem) setLimits(maxEntries int, maxBytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if maxEntries > 0 {
		s.maxEntries = maxEntries
	}
	if maxBytes > 0 {
		s.maxBytes = maxBytes
	}
	s.evictLocked()
}

func (s *twcachemem) dropLocked(el *list.Element) {
	e := s.order.Remove(el).(*memEntry)
	delete(s.items, e.name)
	s.curBytes -= e.size
}

func (s *twcachemem) evictLocked() {
	for s.order.Len() > s.maxEntries || s.curBytes > s.maxBytes {
		back := s.order.Back()
		if back == nil {
			break
		}
		s.dropLocked(back)
		s.evictions++
	}
}

func (s *twcachemem) stats() storeStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return storeStats{
		Mode:       "memory",
		Entries:    s.order.Len(),
		Bytes:      s.curBytes,
		MaxBytes:   s.maxBytes,
		MaxEntries: s.maxEntries,
		Evictions:  s.evictions,
	}
}

func (s *twcachemem) dir() string { return "" }
