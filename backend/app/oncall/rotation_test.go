package oncall

import (
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
)

func mustLoad(t *testing.T, name string) *time.Location {
	t.Helper()
	tz, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load tz %s: %v", name, err)
	}
	return tz
}

func utc(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse %s: %v", value, err)
	}
	return parsed.UTC()
}

func local(t *testing.T, tz *time.Location, value string) time.Time {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02 15:04", value, tz)
	if err != nil {
		t.Fatalf("parse %s: %v", value, err)
	}
	return parsed
}

func assertShift(t *testing.T, shift Shift, userId int, start, end time.Time) {
	t.Helper()
	if shift.UserId != userId {
		t.Errorf("shift user = %d, want %d (shift %v..%v)", shift.UserId, userId, shift.Start, shift.End)
	}
	if !shift.Start.Equal(start) {
		t.Errorf("shift start = %v, want %v", shift.Start, start)
	}
	if !shift.End.Equal(end) {
		t.Errorf("shift end = %v, want %v", shift.End, end)
	}
}

func TestDailyRotation(t *testing.T) {
	tz := mustLoad(t, "UTC")
	layer := &models.OncallLayer{
		Id:            "l_1",
		Name:          "Primary",
		RotationType:  RotationDaily,
		HandoffTime:   "09:00",
		RotationStart: "2026-06-01",
		UserIds:       []int{1, 2, 3},
	}
	from := utc(t, "2026-06-01T09:00:00Z")
	to := utc(t, "2026-06-04T09:00:00Z")
	shifts := ResolveLayerRange(layer, tz, from, to)
	if len(shifts) != 3 {
		t.Fatalf("expected 3 shifts, got %d: %+v", len(shifts), shifts)
	}
	assertShift(t, shifts[0], 1, utc(t, "2026-06-01T09:00:00Z"), utc(t, "2026-06-02T09:00:00Z"))
	assertShift(t, shifts[1], 2, utc(t, "2026-06-02T09:00:00Z"), utc(t, "2026-06-03T09:00:00Z"))
	assertShift(t, shifts[2], 3, utc(t, "2026-06-03T09:00:00Z"), utc(t, "2026-06-04T09:00:00Z"))
}

func TestDailyRotationBeforeAnchor(t *testing.T) {
	tz := mustLoad(t, "UTC")
	layer := &models.OncallLayer{
		Id:            "l_1",
		Name:          "Primary",
		RotationType:  RotationDaily,
		HandoffTime:   "09:00",
		RotationStart: "2026-06-10",
		UserIds:       []int{1, 2, 3},
	}
	// Two days before the anchor: floor-mod should give user 2 (index -2 -> 1).
	from := utc(t, "2026-06-08T09:00:00Z")
	to := utc(t, "2026-06-09T09:00:00Z")
	shifts := ResolveLayerRange(layer, tz, from, to)
	if len(shifts) != 1 {
		t.Fatalf("expected 1 shift, got %d", len(shifts))
	}
	assertShift(t, shifts[0], 2, from, to)
}

func TestWeeklyRotationHandoffDay(t *testing.T) {
	tz := mustLoad(t, "UTC")
	// 2026-06-03 is a Wednesday; handoff day Monday (1) at 09:00.
	layer := &models.OncallLayer{
		Id:            "l_1",
		Name:          "Primary",
		RotationType:  RotationWeekly,
		HandoffTime:   "09:00",
		HandoffDay:    1,
		RotationStart: "2026-06-03",
		UserIds:       []int{1, 2},
	}
	// Period 0 anchors back to Monday 2026-06-01 09:00.
	from := utc(t, "2026-06-01T09:00:00Z")
	to := utc(t, "2026-06-15T09:00:00Z")
	shifts := ResolveLayerRange(layer, tz, from, to)
	if len(shifts) != 2 {
		t.Fatalf("expected 2 shifts, got %d: %+v", len(shifts), shifts)
	}
	assertShift(t, shifts[0], 1, utc(t, "2026-06-01T09:00:00Z"), utc(t, "2026-06-08T09:00:00Z"))
	assertShift(t, shifts[1], 2, utc(t, "2026-06-08T09:00:00Z"), utc(t, "2026-06-15T09:00:00Z"))
}

func TestCustomIntervalRotation(t *testing.T) {
	tz := mustLoad(t, "UTC")
	layer := &models.OncallLayer{
		Id:            "l_1",
		Name:          "Primary",
		RotationType:  RotationCustom,
		HandoffTime:   "00:00",
		IntervalDays:  3,
		RotationStart: "2026-06-01",
		UserIds:       []int{7, 8},
	}
	from := utc(t, "2026-06-01T00:00:00Z")
	to := utc(t, "2026-06-07T00:00:00Z")
	shifts := ResolveLayerRange(layer, tz, from, to)
	if len(shifts) != 2 {
		t.Fatalf("expected 2 shifts, got %d", len(shifts))
	}
	assertShift(t, shifts[0], 7, utc(t, "2026-06-01T00:00:00Z"), utc(t, "2026-06-04T00:00:00Z"))
	assertShift(t, shifts[1], 8, utc(t, "2026-06-04T00:00:00Z"), utc(t, "2026-06-07T00:00:00Z"))
}

func TestSingleUserLayerCoalesces(t *testing.T) {
	tz := mustLoad(t, "UTC")
	layer := &models.OncallLayer{
		Id:            "l_1",
		Name:          "Solo",
		RotationType:  RotationDaily,
		HandoffTime:   "09:00",
		RotationStart: "2026-06-01",
		UserIds:       []int{5},
	}
	from := utc(t, "2026-06-01T00:00:00Z")
	to := utc(t, "2026-06-08T00:00:00Z")
	shifts := ResolveLayerRange(layer, tz, from, to)
	if len(shifts) != 1 {
		t.Fatalf("expected a single coalesced shift, got %d: %+v", len(shifts), shifts)
	}
	assertShift(t, shifts[0], 5, from, to)
}

func TestDSTSpringForwardHandoffStaysLocal(t *testing.T) {
	tz := mustLoad(t, "America/New_York")
	layer := &models.OncallLayer{
		Id:            "l_1",
		Name:          "Primary",
		RotationType:  RotationDaily,
		HandoffTime:   "09:00",
		RotationStart: "2026-03-06",
		UserIds:       []int{1, 2},
	}
	// DST starts 2026-03-08 in the US: 07 09:00 EST -> 08 09:00 EDT is 23h.
	from := local(t, tz, "2026-03-07 09:00").UTC()
	to := local(t, tz, "2026-03-09 09:00").UTC()
	shifts := ResolveLayerRange(layer, tz, from, to)
	if len(shifts) != 2 {
		t.Fatalf("expected 2 shifts, got %d: %+v", len(shifts), shifts)
	}
	first := shifts[0].End.Sub(shifts[0].Start)
	if first != 23*time.Hour {
		t.Errorf("spring-forward day length = %v, want 23h", first)
	}
	for _, shift := range shifts {
		if localTime := shift.Start.In(tz); localTime.Hour() != 9 || localTime.Minute() != 0 {
			t.Errorf("handoff drifted to %02d:%02d local, want 09:00", localTime.Hour(), localTime.Minute())
		}
	}
}

func TestDSTFallBackDayIs25Hours(t *testing.T) {
	tz := mustLoad(t, "America/New_York")
	layer := &models.OncallLayer{
		Id:            "l_1",
		Name:          "Primary",
		RotationType:  RotationDaily,
		HandoffTime:   "09:00",
		RotationStart: "2026-10-30",
		UserIds:       []int{1, 2},
	}
	// DST ends 2026-11-01: the 10-31 09:00 -> 11-01 09:00 period is 25h.
	from := local(t, tz, "2026-10-31 09:00").UTC()
	to := local(t, tz, "2026-11-01 09:00").UTC()
	shifts := ResolveLayerRange(layer, tz, from, to)
	if len(shifts) != 1 {
		t.Fatalf("expected 1 shift, got %d", len(shifts))
	}
	if length := shifts[0].End.Sub(shifts[0].Start); length != 25*time.Hour {
		t.Errorf("fall-back day length = %v, want 25h", length)
	}
}

func TestDSTSkippedHandoffTimeNormalizesForward(t *testing.T) {
	tz := mustLoad(t, "America/New_York")
	layer := &models.OncallLayer{
		Id:            "l_1",
		Name:          "Primary",
		RotationType:  RotationDaily,
		HandoffTime:   "02:30",
		RotationStart: "2026-03-07",
		UserIds:       []int{1, 2},
	}
	// 02:30 does not exist on 2026-03-08; time.Date resolves it to the same
	// instant under one of the two offsets. The invariants that matter: the
	// transition period is 23h, and the handoff is back at 02:30 local the
	// next day.
	from := local(t, tz, "2026-03-07 02:30").UTC()
	to := local(t, tz, "2026-03-09 02:30").UTC()
	shifts := ResolveLayerRange(layer, tz, from, to)
	if len(shifts) != 2 {
		t.Fatalf("expected 2 shifts, got %d: %+v", len(shifts), shifts)
	}
	if length := shifts[0].End.Sub(shifts[0].Start); length != 23*time.Hour {
		t.Errorf("transition period length = %v, want 23h", length)
	}
	nextDay := shifts[1].End.In(tz)
	if nextDay.Hour() != 2 || nextDay.Minute() != 30 {
		t.Errorf("handoff after transition at %02d:%02d local, want 02:30", nextDay.Hour(), nextDay.Minute())
	}
}

// A rotation start whose handoff time falls inside a DST gap must not shift
// the handoff of every later period.
func TestRotationStartInsideDSTGapKeepsHandoffTime(t *testing.T) {
	tz := mustLoad(t, "America/New_York")
	layer := &models.OncallLayer{
		Id:            "l_1",
		Name:          "Primary",
		RotationType:  RotationDaily,
		HandoffTime:   "02:30",
		RotationStart: "2026-03-08", // 02:00 -> 03:00 that morning; 02:30 does not exist
		UserIds:       []int{1, 2, 3},
	}
	from := local(t, tz, "2026-06-10 00:00").UTC()
	shifts := ResolveLayerRange(layer, tz, from, from.AddDate(0, 0, 5))
	if len(shifts) == 0 {
		t.Fatal("expected shifts")
	}
	for _, shift := range shifts[1:] {
		start := shift.Start.In(tz)
		if start.Hour() != 2 || start.Minute() != 30 {
			t.Errorf("handoff at %s, want 02:30 local", start.Format("2006-01-02 15:04 MST"))
		}
	}
}

// Same defect on the weekly path: the handoff weekday must not move.
func TestRotationStartInsideDSTGapKeepsHandoffWeekday(t *testing.T) {
	tz := mustLoad(t, "America/Havana")
	layer := &models.OncallLayer{
		Id:            "l_1",
		Name:          "Primary",
		RotationType:  RotationWeekly,
		HandoffDay:    1, // Monday
		HandoffTime:   "00:00",
		RotationStart: "2026-03-08", // local midnight does not exist that day
		UserIds:       []int{1, 2},
	}
	from := local(t, tz, "2026-06-01 00:00").UTC()
	shifts := ResolveLayerRange(layer, tz, from, from.AddDate(0, 0, 28))
	if len(shifts) == 0 {
		t.Fatal("expected shifts")
	}
	for _, shift := range shifts[1:] {
		start := shift.Start.In(tz)
		if start.Weekday() != time.Monday {
			t.Errorf("handoff on %s, want Monday", start.Format("Mon 2006-01-02 15:04 MST"))
		}
	}
}

// Far-future dates must resolve arithmetically, not by walking periods.
func TestExtremeDatesResolveInConstantTime(t *testing.T) {
	tz := mustLoad(t, "Europe/Berlin")
	for _, rotationStart := range []string{"2026-01-05", "9999-12-31", "0001-01-01"} {
		layer := &models.OncallLayer{
			Id:            "l_1",
			Name:          "Primary",
			RotationType:  RotationDaily,
			HandoffTime:   "09:00",
			RotationStart: rotationStart,
			UserIds:       []int{1, 2},
		}
		for _, from := range []time.Time{
			local(t, tz, "2026-06-01 00:00").UTC(),
			time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC),
		} {
			start := time.Now()
			ResolveLayerRange(layer, tz, from, from.AddDate(0, 0, MaxTimelineRangeDays))
			if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
				t.Errorf("rotationStart=%s from=%s took %v", rotationStart, from.Format("2006-01-02"), elapsed)
			}
		}
	}
}
