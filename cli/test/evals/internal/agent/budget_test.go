package agent

import (
	"math"
	"testing"
)

func TestLedgerReservations(t *testing.T) {
	l := NewLedger(10)
	a, b := l.Reserve(4), l.Reserve(3)
	if got := l.Available(); got != 3 {
		t.Fatalf("two open holds must be set aside: available %v, want 3", got)
	}
	a.Add("m", 1)
	if got := l.Available(); got != 3 {
		t.Fatalf("spending inside a hold must not change what is free: %v", got)
	}
	a.Add("m", 5)
	if got := l.Available(); got != 1 {
		t.Fatalf("spending past a hold must come out of the free pool: %v, want 1", got)
	}
	a.Release()
	b.Release()
	if got, spent := l.Available(), l.Spent(); got != 4 || spent != 6 {
		t.Fatalf("released holds return their unspent part: available %v spent %v", got, spent)
	}
	a.Add("judge", 1)
	if got := l.Available(); got != 3 {
		t.Fatalf("spending after release comes from the free pool: %v", got)
	}
	if l.Exhausted() {
		t.Fatal("not exhausted at $7 of $10")
	}
	if !math.IsInf(NewLedger(0).Available(), 1) || NewLedger(0).Exhausted() {
		t.Fatal("a ledger without a cap is unlimited")
	}
}
