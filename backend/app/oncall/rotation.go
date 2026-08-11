package oncall

import (
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
)

const (
	RotationDaily  = "daily"
	RotationWeekly = "weekly"
	RotationCustom = "custom"
)

// layerPeriods returns the raw rotation periods of a layer overlapping
// [from, to), before restrictions. Period boundaries are computed in calendar
// space (time.Date in the schedule timezone), so a handoff configured at 09:00
// stays at 09:00 local time across DST transitions and transition days are
// simply 23 or 25 hours long.
func layerPeriods(layer *models.OncallLayer, tz *time.Location, from, to time.Time) []Shift {
	year, month, day, ok := parseLocalDate(layer.RotationStart)
	if !ok {
		return nil
	}
	hour, minute, ok := parseLocalTime(layer.HandoffTime)
	if !ok {
		return nil
	}

	periodDays := 1
	switch layer.RotationType {
	case RotationDaily:
	case RotationWeekly:
		periodDays = 7
	case RotationCustom:
		if layer.IntervalDays < 1 {
			return nil
		}
		periodDays = layer.IntervalDays
	default:
		return nil
	}

	if layer.RotationType == RotationWeekly && layer.HandoffDay >= 1 && layer.HandoffDay <= 7 {
		// Shift the anchor back to the configured handoff weekday, so period 0
		// is the week containing the rotation start. The weekday is read from
		// the civil date, not a tz instant, which a DST gap could normalize
		// onto the previous day.
		startWeekday := int(time.Date(year, month, day, 12, 0, 0, 0, time.UTC).Weekday())
		day -= (startWeekday - layer.HandoffDay%7 + 7) % 7
	}

	boundary := func(k int) time.Time {
		return time.Date(year, month, day+k*periodDays, hour, minute, 0, 0, tz)
	}
	anchor := boundary(0)

	// Locate the period containing `from` by day-count estimate, then correct
	// so that boundary(k) <= from < boundary(k+1). Integer seconds rather than
	// from.Sub(anchor), which saturates at ~292 years.
	k := floorDiv(floorDiv(int(from.Unix()-anchor.Unix()), 86400), periodDays)
	for boundary(k).After(from) {
		k--
	}
	for !boundary(k + 1).After(from) {
		k++
	}

	memberCount := len(layer.UserIds)
	var shifts []Shift
	for start := boundary(k); start.Before(to); k, start = k+1, boundary(k+1) {
		end := boundary(k + 1)
		shifts = append(shifts, Shift{
			UserId:  layer.UserIds[floorMod(k, memberCount)],
			LayerId: layer.Id,
			Start:   start.UTC(),
			End:     end.UTC(),
		})
	}
	return shifts
}

func floorMod(a, n int) int {
	return ((a % n) + n) % n
}

func floorDiv(a, n int) int {
	q := a / n
	if a%n != 0 && (a < 0) != (n < 0) {
		q--
	}
	return q
}

func parseLocalDate(s string) (year int, month time.Month, day int, ok bool) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return 0, 0, 0, false
	}
	return t.Year(), t.Month(), t.Day(), true
}

func parseLocalTime(s string) (hour, minute int, ok bool) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, 0, false
	}
	return t.Hour(), t.Minute(), true
}
