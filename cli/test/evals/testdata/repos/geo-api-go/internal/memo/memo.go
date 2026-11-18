// Package memo caches computed results in process.
package memo

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value   V
	expires time.Time
}

// Memo is a concurrency-safe map of results with a time to live.
type Memo[V any] struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[string]entry[V]
}

// New returns a memo whose entries live for ttl.
func New[V any](ttl time.Duration) *Memo[V] {
	return &Memo[V]{ttl: ttl, m: map[string]entry[V]{}}
}

// Get returns the value stored under key if it has not expired.
func (c *Memo[V]) Get(key string) (V, bool) {
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expires) {
		var zero V
		return zero, false
	}
	return e.value, true
}

// Put stores value under key.
func (c *Memo[V]) Put(key string, value V) {
	c.mu.Lock()
	c.m[key] = entry[V]{value: value, expires: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

// Len reports how many entries are stored.
func (c *Memo[V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.m)
}
