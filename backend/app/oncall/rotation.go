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

	anchor := time.Date(year, month, day, hour, minute, 0, 0, tz)
	if layer.RotationType == RotationWeekly && layer.HandoffDay >= 1 && layer.HandoffDay <= 7 {
		// Shift the anchor back to the configured handoff weekday, so period 0
		// is the week containing the rotation start.
		daysSinceHandoff := (int(anchor.Weekday()) - layer.HandoffDay%7 + 7) % 7
		anchor = time.Date(year, month, day-daysSinceHandoff, hour, minute, 0, 0, tz)
	}

	boundary := func(k int) time.Time {
		return time.Date(anchor.Year(), anchor.Month(), anchor.Day()+k*periodDays, anchor.Hour(), anchor.Minute(), 0, 0, tz)
	}

	// Locate the period containing `from` by day-count estimate, then correct
	// so that boundary(k) <= from < boundary(k+1).
	k := int(from.Sub(anchor).Hours() / 24 / float64(periodDays))
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
