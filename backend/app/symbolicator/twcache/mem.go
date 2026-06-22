package twcache

import (
	"container/list"
	"sync"
)

type twcachemem[V any] struct {
	mu         sync.Mutex
	items      map[string]*list.Element
	order      *list.List
	maxEntries int
	maxBytes   int64
	curBytes   int64
	evictions  uint64
	weigh      func(V) int64
}

type memEntry[V any] struct {
	name string
	data V
	size int64
}

func newMemStore[V any](maxEntries int, maxBytes int64, weigh func(V) int64) *twcachemem[V] {
	return &twcachemem[V]{
		items:      make(map[string]*list.Element),
		order:      list.New(),
		maxEntries: maxEntries,
		maxBytes:   maxBytes,
		weigh:      weigh,
	}
}

func (s *twcachemem[V]) get(name string) (V, func(), bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	el, ok := s.items[name]
	if !ok {
		var zero V
		return zero, noop, false
	}
	s.order.MoveToFront(el)
	return el.Value.(*memEntry[V]).data, noop, true
}

func (s *twcachemem[V]) contains(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.items[name]
	return ok
}

func (s *twcachemem[V]) put(name string, data V) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	size := s.weigh(data)
	if el, ok := s.items[name]; ok {
		e := el.Value.(*memEntry[V])
		s.curBytes += size - e.size
		e.data = data
		e.size = size
		s.order.MoveToFront(el)
	} else {
		s.items[name] = s.order.PushFront(&memEntry[V]{name: name, data: data, size: size})
		s.curBytes += size
	}
	s.evictLocked()
	return nil
}

func (s *twcachemem[V]) remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if el, ok := s.items[name]; ok {
		s.dropLocked(el)
	}
}

func (s *twcachemem[V]) setLimits(maxEntries int, maxBytes int64) {
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

func (s *twcachemem[V]) dropLocked(el *list.Element) {
	e := s.order.Remove(el).(*memEntry[V])
	delete(s.items, e.name)
	s.curBytes -= e.size
}

func (s *twcachemem[V]) evictLocked() {
	for s.order.Len() > s.maxEntries || s.curBytes > s.maxBytes {
		back := s.order.Back()
		if back == nil {
			break
		}
		s.dropLocked(back)
		s.evictions++
	}
}

func (s *twcachemem[V]) stats() storeStats {
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

func (s *twcachemem[V]) dir() string { return "" }
