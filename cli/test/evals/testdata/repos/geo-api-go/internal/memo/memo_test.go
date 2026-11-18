package memo

import (
	"testing"
	"time"
)

func TestGetReturnsFreshValues(t *testing.T) {
	m := New[int](time.Minute)
	m.Put("a", 1)
	if v, ok := m.Get("a"); !ok || v != 1 {
		t.Fatalf("Get(a) = %d, %v", v, ok)
	}
	if _, ok := m.Get("b"); ok {
		t.Fatal("Get(b) found a value that was never stored")
	}
}

func TestExpiredValuesAreNotReturned(t *testing.T) {
	m := New[int](10 * time.Millisecond)
	m.Put("a", 1)
	time.Sleep(20 * time.Millisecond)
	if _, ok := m.Get("a"); ok {
		t.Fatal("expired value returned")
	}
}
