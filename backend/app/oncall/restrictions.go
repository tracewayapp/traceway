package oncall

import (
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
)

const (
	RestrictionDaily  = "daily"
	RestrictionWeekly = "weekly"
)

// expandRestrictions turns recurring restriction windows into concrete UTC
// spans overlapping [from, to). Windows are computed in the schedule timezone.
func expandRestrictions(restrictions []models.OncallRestriction, tz *time.Location, from, to time.Time) []span {
	var windows []span
	for _, restriction := range restrictions {
		switch restriction.Type {
		case RestrictionDaily:
			windows = append(windows, expandDaily(restriction, tz, from, to)...)
		case RestrictionWeekly:
			windows = append(windows, expandWeekly(restriction, tz, from, to)...)
		}
	}
	return mergeSpans(windows)
}

func expandDaily(restriction models.OncallRestriction, tz *time.Location, from, to time.Time) []span {
	startHour, startMinute, ok := parseLocalTime(restriction.StartTime)
	if !ok {
		return nil
	}
	endHour, endMinute, ok := parseLocalTime(restriction.EndTime)
	if !ok {
		return nil
	}

	// Whether the window wraps past midnight is decided from the configured
	// wall-clock values, not the normalized instants: on a DST spring-forward
	// day a window inside the skipped hour collapses to a single instant,
	// which must yield no coverage rather than a full wrapped day.
	endDayOffset := 0
	if !isTimeAfter(endHour, endMinute, startHour, startMinute) {
		endDayOffset = 1
	}

	var windows []span
	// Start one day early so a window that wraps past midnight into the range
	// is not missed.
	local := from.In(tz)
	for day := -1; ; day++ {
		start := time.Date(local.Year(), local.Month(), local.Day()+day, startHour, startMinute, 0, 0, tz)
		end := time.Date(local.Year(), local.Month(), local.Day()+day+endDayOffset, endHour, endMinute, 0, 0, tz)
		if start.After(to) {
			break
		}
		if end.After(start) && end.After(from) {
			windows = append(windows, span{maxTime(start.UTC(), from), minTime(end.UTC(), to)})
		}
	}
	return windows
}

func expandWeekly(restriction models.OncallRestriction, tz *time.Location, from, to time.Time) []span {
	startHour, startMinute, ok := parseLocalTime(restriction.StartTime)
	if !ok {
		return nil
	}
	endHour, endMinute, ok := parseLocalTime(restriction.EndTime)
	if !ok {
		return nil
	}
	if restriction.StartDay < 1 || restriction.StartDay > 7 || restriction.EndDay < 1 || restriction.EndDay > 7 {
		return nil
	}

	// Anchor on the StartDay occurrence at or before `from`, minus one extra
	// week so a window already in progress is included.
	local := from.In(tz)
	daysSinceStartDay := (int(local.Weekday()) - restriction.StartDay%7 + 7) % 7
	anchorYear, anchorMonth, anchorDay := local.Year(), local.Month(), local.Day()-daysSinceStartDay-7

	dayLength := (restriction.EndDay - restriction.StartDay + 7) % 7
	wrapsWeek := dayLength == 0 && !isTimeAfter(endHour, endMinute, startHour, startMinute)
	if wrapsWeek {
		dayLength = 7
	}

	var windows []span
	for week := 0; ; week++ {
		start := time.Date(anchorYear, anchorMonth, anchorDay+week*7, startHour, startMinute, 0, 0, tz)
		end := time.Date(anchorYear, anchorMonth, anchorDay+week*7+dayLength, endHour, endMinute, 0, 0, tz)
		if start.After(to) {
			break
		}
		// A window whose end normalizes back onto its start (a spring-forward
		// gap swallowed it) covers nothing that week. Skip that week only:
		// abandoning the loop would drop every later week too. expandDaily
		// makes the same distinction.
		if end.After(start) && end.After(from) {
			windows = append(windows, span{maxTime(start.UTC(), from), minTime(end.UTC(), to)})
		}
	}
	return windows
}

func isTimeAfter(aHour, aMinute, bHour, bMinute int) bool {
	if aHour != bHour {
		return aHour > bHour
	}
	return aMinute > bMinute
}
