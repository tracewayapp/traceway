package notifications

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	traceway "go.tracewayapp.com"
)

type cooldownTracker struct {
	mu    sync.RWMutex
	fired map[int]time.Time
}

var cooldowns = &cooldownTracker{fired: make(map[int]time.Time)}

func (m *cooldownTracker) canFire(ruleId int, cooldownMinutes int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	last, ok := m.fired[ruleId]
	if !ok {
		return true
	}
	return time.Since(last) > time.Duration(cooldownMinutes)*time.Minute
}

func (m *cooldownTracker) recordFire(ruleId int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fired[ruleId] = time.Now()
}

func ruleStatePrefix(ruleId int) string {
	return fmt.Sprintf("%d:", ruleId)
}

func errorDedupKey(ruleId int, hash string) string {
	return ruleStatePrefix(ruleId) + hash
}

func aiCostDedupKey(ruleId int, traceName string) string {
	return ruleStatePrefix(ruleId) + "ai_cost:" + traceName
}

func ClearRuleState(ruleId int) {
	cooldowns.mu.Lock()
	delete(cooldowns.fired, ruleId)
	cooldowns.mu.Unlock()

	prefix := ruleStatePrefix(ruleId)
	dedup.mu.Lock()
	for k := range dedup.seen {
		if strings.HasPrefix(k, prefix) {
			delete(dedup.seen, k)
		}
	}
	dedup.mu.Unlock()

	impactStateMu.Lock()
	for k := range impactState {
		if strings.HasPrefix(k, prefix) {
			delete(impactState, k)
		}
	}
	impactStateMu.Unlock()
}

func (m *cooldownTracker) seed(entries map[int]time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, t := range entries {
		if existing, ok := m.fired[id]; !ok || t.After(existing) {
			m.fired[id] = t
		}
	}
}

type dedupTracker struct {
	mu   sync.RWMutex
	seen map[string]time.Time
}

var dedup = &dedupTracker{seen: make(map[string]time.Time)}

func (m *dedupTracker) isDuplicate(key string, cooldown time.Duration) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	last, ok := m.seen[key]
	return ok && time.Since(last) < cooldown
}

func (m *dedupTracker) record(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seen[key] = time.Now()
}

func (m *dedupTracker) purgeExpired(maxAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, t := range m.seen {
		if now.Sub(t) > maxAge {
			delete(m.seen, k)
		}
	}
}

func startDedupPurger(ctx context.Context) {
	go func() {
		defer traceway.Recover()

		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				dedup.purgeExpired(24 * time.Hour)
			}
		}
	}()
}
