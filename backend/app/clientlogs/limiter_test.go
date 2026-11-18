package clientlogs

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBudgetIsolationExpiryAndReview(t *testing.T) {
	l := New(10)
	p := uuid.New()
	now := time.Now()
	check := func(p uuid.UUID, ip string, n int, at time.Time, want int, flagged bool) {
		t.Helper()
		got, flag, full := l.Allow(p, ip, n, at)
		if got != want || flag != flagged || full {
			t.Fatalf("got %d %v %v, want %d %v", got, flag, full, want, flagged)
		}
	}
	check(p, "192.0.2.1", 8, now, 8, false)
	check(p, "192.0.2.1", 8, now, 2, true)
	check(p, "192.0.2.1", 8, now, 0, false)
	check(p, "192.0.2.2", 10, now, 10, false)
	check(uuid.New(), "192.0.2.1", 10, now, 10, false)
	check(p, "192.0.2.1", 11, now.Add(Window), 10, true)
}

func TestConcurrentBudget(t *testing.T) {
	l := New(1000)
	p := uuid.New()
	now := time.Now()
	var total, flags atomic.Int64
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			n, flag, _ := l.Allow(p, "192.0.2.1", 30, now)
			total.Add(int64(n))
			if flag {
				flags.Add(1)
			}
		})
	}
	wg.Wait()
	if total.Load() != 1000 || flags.Load() != 1 {
		t.Fatalf("accepted=%d flags=%d", total.Load(), flags.Load())
	}
}

func TestCapacityFailsClosedWithoutEvictingActiveKeys(t *testing.T) {
	l := New(1)
	now := time.Now()
	p := uuid.New()
	for range maxKeys {
		l.Allow(uuid.New(), "192.0.2.1", 1, now)
	}
	if n, flag, full := l.Allow(p, "192.0.2.1", 1, now); n != 0 || flag || !full {
		t.Fatalf("%d %v %v", n, flag, full)
	}
	if n, _, full := l.Allow(p, "192.0.2.1", 1, now.Add(Window)); n != 1 || full {
		t.Fatalf("expiry: %d %v", n, full)
	}
}

func TestEquivalentIPAddressesShareBudget(t *testing.T) {
	l := New(1)
	p := uuid.New()
	now := time.Now()
	l.Allow(p, "::ffff:192.0.2.1", 1, now)
	if n, _, _ := l.Allow(p, "192.0.2.1", 1, now); n != 0 {
		t.Fatal("IPv4-mapped IP bypassed the budget")
	}
	l.Allow(p, "2001:db8::1", 1, now)
	if n, _, _ := l.Allow(p, "2001:0db8:0:0:0:0:0:1", 1, now); n != 0 {
		t.Fatal("expanded IPv6 bypassed the budget")
	}
}
