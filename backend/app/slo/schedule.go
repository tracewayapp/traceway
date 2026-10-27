package slo

import (
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
)

func slotOffset(sloId int, interval time.Duration) time.Duration {
	seconds := uint64(interval / time.Second)
	if seconds == 0 {
		return 0
	}
	return time.Duration((uint64(sloId)*2654435761)%seconds) * time.Second
}

func slotIndex(t time.Time, offset, interval time.Duration) int64 {
	n := t.UnixNano() - int64(offset)
	d := int64(interval)
	q := n / d
	if n%d != 0 && n < 0 {
		q--
	}
	return q
}

// passDue reports whether the SLO's own slot of the interval falls in
// (since, now]. Slots are spread by SLO id and keyed on the persisted
// computed_at, so a restart or a shard moving to another instance neither
// skips a pass nor runs every SLO's pass on the same tick.
func passDue(sloId int, interval time.Duration, since *time.Time, now time.Time) bool {
	if since == nil {
		return true
	}
	if !now.After(*since) {
		return false
	}
	offset := slotOffset(sloId, interval)
	return slotIndex(now, offset, interval) > slotIndex(*since, offset, interval)
}

func recomputeHorizon(slo *models.SLO, now time.Time) time.Duration {
	if slo.SliKind == models.SLIKindSession && passDue(slo.Id, lateCrashInterval, slo.ComputedAt, now) {
		return lateCrashHorizon
	}
	if passDue(slo.Id, lateArrivalInterval, slo.ComputedAt, now) {
		return lateArrivalHorizon(slo.SliKind)
	}
	return lateDataWindow
}
