package clientmetrics

import (
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const (
	dedupHashes     = 4
	dedupGeneration = 24 * time.Hour
	defaultDedupMB  = 16
)

// batchFilter remembers which payloads were received, so a payload a device sends
// again (its page died before the answer came) is counted once. It is a Bloom filter
// in two generations of a day each: memory is fixed, every payload id of the last 24
// to 48 hours is found, and a false positive only drops one resend in a few hundred.
// It is consulted for resends only, so a first upload is never refused by it.
type batchFilter struct {
	mu      sync.RWMutex
	current []atomic.Uint64
	old     []atomic.Uint64
	bits    uint64
	rotated time.Time
}

func newBatchFilter(megabytes int, now time.Time) *batchFilter {
	if megabytes <= 0 {
		megabytes = defaultDedupMB
	}
	words := megabytes << 20 / 2 / 8
	return &batchFilter{current: make([]atomic.Uint64, words), old: make([]atomic.Uint64, words), bits: uint64(words) * 64, rotated: now}
}

func (f *batchFilter) positions(projectId uuid.UUID, id string) [dedupHashes]uint64 {
	h := fnv.New128a()
	_, _ = h.Write(projectId[:])
	_, _ = h.Write([]byte(id))
	sum := h.Sum(nil)
	var a, b uint64
	for i := 0; i < 8; i++ {
		a = a<<8 | uint64(sum[i])
		b = b<<8 | uint64(sum[8+i])
	}
	// FNV leaves the low bits of similar ids close together, and the filter indexes by
	// the low bits.
	a, b = mix64(a), mix64(b)
	var out [dedupHashes]uint64
	for i := range out {
		out[i] = (a + uint64(i)*(b|1)) % f.bits
	}
	return out
}

func mix64(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	return x ^ x>>31
}

func has(words []atomic.Uint64, positions [dedupHashes]uint64) bool {
	for _, p := range positions {
		if words[p/64].Load()&(1<<(p%64)) == 0 {
			return false
		}
	}
	return true
}

// seen records the payload and reports whether it was already there.
func (f *batchFilter) seen(projectId uuid.UUID, id string, now time.Time) bool {
	f.rotate(now)
	positions := f.positions(projectId, id)
	f.mu.RLock()
	defer f.mu.RUnlock()
	known := has(f.current, positions) || has(f.old, positions)
	for _, p := range positions {
		f.current[p/64].Or(1 << (p % 64))
	}
	return known
}

func (f *batchFilter) rotate(now time.Time) {
	f.mu.RLock()
	due := now.Sub(f.rotated) >= dedupGeneration
	f.mu.RUnlock()
	if !due {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if now.Sub(f.rotated) < dedupGeneration {
		return
	}
	f.current, f.old = make([]atomic.Uint64, len(f.current)), f.current
	f.rotated = now
}
