package slo

import (
	"context"
	"sync"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
)

const snapshotRefreshInterval = time.Hour

// The window sum is split at an anchor: the stable part before it is kept in
// memory and only loses the buckets that slide out of the window, the recent
// part after it, where recomputes and late data land, is read every tick.
// Any write before the anchor, an edit, or a snapshot written by another
// instance forces a full read, and every SLO is read in full once an hour.
type windowCache struct {
	revision    int
	computedAt  time.Time
	anchor      time.Time
	windowStart time.Time
	stable      Window
}

var (
	windowCacheMu sync.Mutex
	windowCaches  = map[int]*windowCache{}
)

func recentSpan(kind string) time.Duration {
	if kind == models.SLIKindSession {
		return lateCrashHorizon + time.Hour
	}
	return 7 * time.Hour
}

type windowSums struct {
	window, lastHour, lastSixHours Window
	cache                          *windowCache
	full                           bool
}

func readWindowSums(ctx context.Context, result *tickResult, now time.Time) (windowSums, error) {
	slo := result.slo
	lastClosed := result.lastClosed
	sixFrom, hourFrom := lastClosed.Add(-6*time.Hour), lastClosed.Add(-time.Hour)

	windowCacheMu.Lock()
	cached := windowCaches[slo.Id]
	windowCacheMu.Unlock()

	full := cached == nil ||
		cached.revision != slo.Revision ||
		slo.ComputedAt == nil ||
		!sameInstant(cached.computedAt, *slo.ComputedAt) ||
		result.windowStart.Before(cached.windowStart) ||
		(result.earliestWrite != nil && result.earliestWrite.Before(cached.anchor)) ||
		sixFrom.Before(cached.anchor) ||
		lastClosed.Sub(cached.anchor) > recentSpan(slo.SliKind)+2*snapshotRefreshInterval ||
		passDue(slo.Id, snapshotRefreshInterval, slo.ComputedAt, now)

	next := &windowCache{revision: slo.Revision, computedAt: now, windowStart: result.windowStart}
	if full {
		next.anchor = lastClosed.Add(-recentSpan(slo.SliKind)).Truncate(slo.BucketDuration())
		if next.anchor.Before(result.windowStart) {
			next.anchor = result.windowStart
		}
		stable, err := sumRange(ctx, slo, result.windowStart, next.anchor)
		if err != nil {
			return windowSums{}, err
		}
		next.stable = stable
	} else {
		next.anchor = cached.anchor
		next.stable = cached.stable
		if result.windowStart.After(cached.windowStart) {
			dropped, err := sumRange(ctx, slo, cached.windowStart, result.windowStart)
			if err != nil {
				return windowSums{}, err
			}
			next.stable.Good -= dropped.Good
			next.stable.Total -= dropped.Total
		}
	}
	recent, err := telemetry.SLOBucketRepository.SnapshotSums(ctx, slo.ProjectId, slo.Id, next.anchor, lastClosed, sixFrom, hourFrom)
	if err != nil {
		return windowSums{}, err
	}
	return windowSums{
		window:       Window{Good: next.stable.Good + recent.Window.Good, Total: next.stable.Total + recent.Window.Total},
		lastHour:     window(recent.LastHour),
		lastSixHours: window(recent.LastSixHours),
		cache:        next,
		full:         full,
	}, nil
}

func storeWindowCache(sloId int, cache *windowCache) {
	windowCacheMu.Lock()
	defer windowCacheMu.Unlock()
	if cache == nil {
		delete(windowCaches, sloId)
		return
	}
	windowCaches[sloId] = cache
}

func pruneWindowCaches(keep map[int]bool) {
	windowCacheMu.Lock()
	defer windowCacheMu.Unlock()
	for id := range windowCaches {
		if !keep[id] {
			delete(windowCaches, id)
		}
	}
}

func sameInstant(a, b time.Time) bool {
	d := a.Sub(b)
	return d > -time.Millisecond && d < time.Millisecond
}
