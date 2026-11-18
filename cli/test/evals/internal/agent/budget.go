package agent

import (
	"fmt"
	"math"
	"sync"
)

// Ledger tracks spending across every run of a dispatch (agents and judge)
// against the --max-cost-usd ceiling. A run reserves its cap when it starts
// (Reserve), so the runs in flight are never promised more than is left
// between them, and the loop stops a run before a call that would likely
// cross its cap. The ceiling can still be passed by the error in that
// estimate (about one turn's growth per run in flight) plus the judge.
type Ledger struct {
	mu    sync.Mutex
	Cap   float64
	spent float64
	// held is the unspent part of every open reservation.
	held    float64
	byModel map[string]float64
}

// NewLedger returns a ledger with a ceiling. Cap <= 0 means unlimited, which
// only the fake model uses.
func NewLedger(cap float64) *Ledger { return &Ledger{Cap: cap, byModel: map[string]float64{}} }

// Add records spending outside any run's reservation.
func (l *Ledger) Add(model string, usd float64) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.spent += usd
	l.byModel[model] += usd
}

// Spent returns the total.
func (l *Ledger) Spent() float64 {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.spent
}

// ByModel returns spending per model.
func (l *Ledger) ByModel() map[string]float64 {
	out := map[string]float64{}
	if l == nil {
		return out
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, v := range l.byModel {
		out[k] = v
	}
	return out
}

// Available returns what is neither spent nor reserved by a run in flight
// (+Inf when there is no cap).
func (l *Ledger) Available() float64 {
	if l == nil || l.Cap <= 0 {
		return math.Inf(1)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Cap - l.spent - l.held
}

// Exhausted reports whether the spending reached the ceiling.
func (l *Ledger) Exhausted() bool {
	if l == nil || l.Cap <= 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.spent >= l.Cap
}

// String summarizes the ledger.
func (l *Ledger) String() string {
	if l == nil {
		return "no ledger"
	}
	return fmt.Sprintf("$%.4f of $%.2f", l.Spent(), l.Cap)
}

// Hold is one run's reservation of its cap.
type Hold struct {
	l      *Ledger
	Cap    float64
	spent  float64
	closed bool
}

// Reserve sets usd aside for one run until the hold is released.
func (l *Ledger) Reserve(usd float64) *Hold {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.held += usd
	return &Hold{l: l, Cap: usd}
}

// Add records the run's spending, which uses up its reservation first.
func (h *Hold) Add(model string, usd float64) {
	if h == nil {
		return
	}
	l := h.l
	l.mu.Lock()
	defer l.mu.Unlock()
	if !h.closed {
		l.held -= min(usd, max(0, h.Cap-h.spent))
	}
	h.spent += usd
	l.spent += usd
	l.byModel[model] += usd
}

// Spent returns what the run spent.
func (h *Hold) Spent() float64 {
	if h == nil {
		return 0
	}
	h.l.mu.Lock()
	defer h.l.mu.Unlock()
	return h.spent
}

// Release returns the unspent part of the reservation to the ledger.
func (h *Hold) Release() {
	if h == nil {
		return
	}
	h.l.mu.Lock()
	defer h.l.mu.Unlock()
	if !h.closed {
		h.l.held -= max(0, h.Cap-h.spent)
		h.closed = true
	}
}
