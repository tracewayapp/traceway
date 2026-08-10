package oncall

import (
	"sort"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
)

// Shift is one contiguous stretch of a single person being on call. Times are UTC.
type Shift struct {
	UserId     int       `json:"userId"`
	LayerId    string    `json:"layerId"`
	Start      time.Time `json:"start"`
	End        time.Time `json:"end"`
	IsOverride bool      `json:"isOverride"`
}

type span struct {
	start time.Time
	end   time.Time
}

// ResolveLayerRange renders one layer's coverage for [from, to) without
// stacking or overrides: rotation periods intersected with the layer's
// restriction windows.
func ResolveLayerRange(layer *models.OncallLayer, tz *time.Location, from, to time.Time) []Shift {
	if len(layer.UserIds) == 0 || !from.Before(to) {
		return nil
	}
	shifts := layerPeriods(layer, tz, from, to)
	if len(layer.Restrictions) > 0 {
		windows := expandRestrictions(layer.Restrictions, tz, from, to)
		shifts = intersectShifts(shifts, windows)
	}
	shifts = clampShifts(shifts, from, to)
	return coalesceShifts(shifts)
}

// ResolveRange renders the final schedule for [from, to): per-layer coverage,
// stacked so that a later layer's coverage replaces earlier layers where they
// overlap (PagerDuty "higher layer number takes precedence"), with overrides
// on top of everything.
func ResolveRange(def *models.OncallScheduleDefinition, tz *time.Location, overrides []*models.OncallOverride, from, to time.Time) []Shift {
	if !from.Before(to) {
		return nil
	}
	var result []Shift
	for i := range def.Layers {
		layerShifts := ResolveLayerRange(&def.Layers[i], tz, from, to)
		if len(layerShifts) == 0 {
			continue
		}
		result = subtractShifts(result, spansOf(layerShifts))
		result = append(result, layerShifts...)
	}

	// Later-created overrides win over earlier ones, and every override wins
	// over layer coverage, so apply them in created_at order: each subtracts
	// what came before it.
	sorted := make([]*models.OncallOverride, len(overrides))
	copy(sorted, overrides)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].CreatedAt.Before(sorted[j].CreatedAt) })
	for _, override := range sorted {
		start := maxTime(override.StartAt.UTC(), from)
		end := minTime(override.EndAt.UTC(), to)
		if !start.Before(end) {
			continue
		}
		result = subtractShifts(result, []span{{start, end}})
		result = append(result, Shift{
			UserId:     override.UserId,
			Start:      start,
			End:        end,
			IsOverride: true,
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Start.Before(result[j].Start) })
	return coalesceShifts(result)
}

// ResolveAt answers "who is on call at t" with the same stacking ResolveRange
// renders: a later layer replaces the layers under it and an override replaces
// everything, so at most one person is on call. Returning every covering user
// instead would page the person an override was meant to relieve, and page one
// person per layer on a stacked schedule. Empty means nobody is on call.
func ResolveAt(def *models.OncallScheduleDefinition, tz *time.Location, overrides []*models.OncallOverride, at time.Time) []int {
	for _, shift := range ResolveRange(def, tz, overrides, at, at.Add(time.Nanosecond)) {
		if !shift.Start.After(at) && shift.End.After(at) {
			return []int{shift.UserId}
		}
	}
	return nil
}

// CurrentAndNext gives the UI "on call until X, next up Y". The lookahead is
// bounded so sparse schedules terminate.
func CurrentAndNext(def *models.OncallScheduleDefinition, tz *time.Location, overrides []*models.OncallOverride, now time.Time) (current *Shift, next *Shift) {
	shifts := ResolveRange(def, tz, overrides, now, now.AddDate(0, 0, 35))
	for i := range shifts {
		shift := shifts[i]
		if !shift.Start.After(now) && shift.End.After(now) {
			current = &shift
			continue
		}
		if shift.Start.After(now) {
			next = &shift
			break
		}
	}
	return current, next
}

func spansOf(shifts []Shift) []span {
	spans := make([]span, 0, len(shifts))
	for _, shift := range shifts {
		spans = append(spans, span{shift.Start, shift.End})
	}
	return mergeSpans(spans)
}

func mergeSpans(spans []span) []span {
	if len(spans) == 0 {
		return nil
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start.Before(spans[j].start) })
	merged := []span{spans[0]}
	for _, s := range spans[1:] {
		last := &merged[len(merged)-1]
		if !s.start.After(last.end) {
			if s.end.After(last.end) {
				last.end = s.end
			}
			continue
		}
		merged = append(merged, s)
	}
	return merged
}

// intersectShifts cuts shifts down to the parts covered by windows.
func intersectShifts(shifts []Shift, windows []span) []Shift {
	windows = mergeSpans(windows)
	var result []Shift
	for _, shift := range shifts {
		for _, w := range windows {
			start := maxTime(shift.Start, w.start)
			end := minTime(shift.End, w.end)
			if start.Before(end) {
				cut := shift
				cut.Start = start
				cut.End = end
				result = append(result, cut)
			}
		}
	}
	return result
}

// subtractShifts removes the given spans from shifts, splitting where needed.
func subtractShifts(shifts []Shift, remove []span) []Shift {
	remove = mergeSpans(remove)
	var result []Shift
	for _, shift := range shifts {
		pieces := []span{{shift.Start, shift.End}}
		for _, r := range remove {
			var next []span
			for _, p := range pieces {
				if !r.end.After(p.start) || !p.end.After(r.start) {
					next = append(next, p)
					continue
				}
				if r.start.After(p.start) {
					next = append(next, span{p.start, minTime(p.end, r.start)})
				}
				if r.end.Before(p.end) {
					next = append(next, span{maxTime(p.start, r.end), p.end})
				}
			}
			pieces = next
		}
		for _, p := range pieces {
			if p.start.Before(p.end) {
				cut := shift
				cut.Start = p.start
				cut.End = p.end
				result = append(result, cut)
			}
		}
	}
	return result
}

func clampShifts(shifts []Shift, from, to time.Time) []Shift {
	var result []Shift
	for _, shift := range shifts {
		start := maxTime(shift.Start, from)
		end := minTime(shift.End, to)
		if start.Before(end) {
			shift.Start = start
			shift.End = end
			result = append(result, shift)
		}
	}
	return result
}

func coalesceShifts(shifts []Shift) []Shift {
	if len(shifts) == 0 {
		return nil
	}
	sort.Slice(shifts, func(i, j int) bool { return shifts[i].Start.Before(shifts[j].Start) })
	result := []Shift{shifts[0]}
	for _, shift := range shifts[1:] {
		last := &result[len(result)-1]
		if shift.UserId == last.UserId && shift.LayerId == last.LayerId && shift.IsOverride == last.IsOverride && shift.Start.Equal(last.End) {
			last.End = shift.End
			continue
		}
		result = append(result, shift)
	}
	return result
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
