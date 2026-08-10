package oncall

import (
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
)

// daytimeLayer covers 09:00-17:00 every day via a daily restriction.
func daytimeLayer(id string, userIds []int) models.OncallLayer {
	return models.OncallLayer{
		Id:            id,
		Name:          "Daytime",
		RotationType:  RotationWeekly,
		HandoffTime:   "09:00",
		HandoffDay:    1,
		RotationStart: "2026-06-01",
		UserIds:       userIds,
		Restrictions: []models.OncallRestriction{
			{Type: RestrictionDaily, StartTime: "09:00", EndTime: "17:00"},
		},
	}
}

func baseLayer(id string, userIds []int) models.OncallLayer {
	return models.OncallLayer{
		Id:            id,
		Name:          "Base",
		RotationType:  RotationWeekly,
		HandoffTime:   "09:00",
		HandoffDay:    1,
		RotationStart: "2026-06-01",
		UserIds:       userIds,
	}
}

func TestDailyRestrictionBusinessHours(t *testing.T) {
	tz := mustLoad(t, "UTC")
	layer := &models.OncallLayer{
		Id: "l_1", Name: "Days", RotationType: RotationDaily, HandoffTime: "09:00",
		RotationStart: "2026-06-01", UserIds: []int{1, 2},
		Restrictions: []models.OncallRestriction{{Type: RestrictionDaily, StartTime: "09:00", EndTime: "17:00"}},
	}
	from := utc(t, "2026-06-01T00:00:00Z")
	to := utc(t, "2026-06-03T00:00:00Z")
	shifts := ResolveLayerRange(layer, tz, from, to)
	if len(shifts) != 2 {
		t.Fatalf("expected 2 shifts, got %d: %+v", len(shifts), shifts)
	}
	assertShift(t, shifts[0], 1, utc(t, "2026-06-01T09:00:00Z"), utc(t, "2026-06-01T17:00:00Z"))
	assertShift(t, shifts[1], 2, utc(t, "2026-06-02T09:00:00Z"), utc(t, "2026-06-02T17:00:00Z"))
}

func TestDailyRestrictionWrapsMidnight(t *testing.T) {
	tz := mustLoad(t, "UTC")
	layer := &models.OncallLayer{
		Id: "l_1", Name: "Nights", RotationType: RotationDaily, HandoffTime: "22:00",
		RotationStart: "2026-06-01", UserIds: []int{1},
		Restrictions: []models.OncallRestriction{{Type: RestrictionDaily, StartTime: "22:00", EndTime: "06:00"}},
	}
	from := utc(t, "2026-06-01T00:00:00Z")
	to := utc(t, "2026-06-03T00:00:00Z")
	shifts := ResolveLayerRange(layer, tz, from, to)
	// 00:00-06:00 (tail of prior night), 22:00-06:00, 22:00-24:00 clamp.
	if len(shifts) != 3 {
		t.Fatalf("expected 3 shifts, got %d: %+v", len(shifts), shifts)
	}
	assertShift(t, shifts[0], 1, utc(t, "2026-06-01T00:00:00Z"), utc(t, "2026-06-01T06:00:00Z"))
	assertShift(t, shifts[1], 1, utc(t, "2026-06-01T22:00:00Z"), utc(t, "2026-06-02T06:00:00Z"))
	assertShift(t, shifts[2], 1, utc(t, "2026-06-02T22:00:00Z"), utc(t, "2026-06-03T00:00:00Z"))
}

func TestDailyRestrictionInsideSpringForwardGap(t *testing.T) {
	tz := mustLoad(t, "America/New_York")
	layer := &models.OncallLayer{
		Id: "l_1", Name: "Gap", RotationType: RotationDaily, HandoffTime: "00:00",
		RotationStart: "2026-03-01", UserIds: []int{1},
		// 02:00 does not exist on 2026-03-08 (DST starts at 02:00) and Go
		// normalizes it to 01:00 EST — before the window's 01:30 start. An
		// instant-based wrap check would treat that as a midnight-wrapping
		// window and inflate it to ~24h; the window must instead vanish for
		// the transition day.
		Restrictions: []models.OncallRestriction{{Type: RestrictionDaily, StartTime: "01:30", EndTime: "02:00"}},
	}
	from := local(t, tz, "2026-03-07 00:00").UTC()
	to := local(t, tz, "2026-03-09 12:00").UTC()
	shifts := ResolveLayerRange(layer, tz, from, to)
	if len(shifts) != 2 {
		t.Fatalf("expected 2 shifts (03-07 and 03-09, nothing on the transition day), got %d: %+v", len(shifts), shifts)
	}
	assertShift(t, shifts[0], 1, local(t, tz, "2026-03-07 01:30").UTC(), local(t, tz, "2026-03-07 02:00").UTC())
	assertShift(t, shifts[1], 1, local(t, tz, "2026-03-09 01:30").UTC(), local(t, tz, "2026-03-09 02:00").UTC())
	for _, shift := range shifts {
		if shift.End.Sub(shift.Start) > time.Hour {
			t.Errorf("restriction window inflated to %v, want <= 1h", shift.End.Sub(shift.Start))
		}
	}
}

func TestWeeklyRestrictionSpansDays(t *testing.T) {
	tz := mustLoad(t, "UTC")
	layer := &models.OncallLayer{
		Id: "l_1", Name: "Weekend", RotationType: RotationWeekly, HandoffTime: "09:00", HandoffDay: 1,
		RotationStart: "2026-06-01", UserIds: []int{4},
		// Friday 17:00 -> Monday 09:00.
		Restrictions: []models.OncallRestriction{{Type: RestrictionWeekly, StartDay: 5, StartTime: "17:00", EndDay: 1, EndTime: "09:00"}},
	}
	from := utc(t, "2026-06-01T00:00:00Z") // Monday
	to := utc(t, "2026-06-09T00:00:00Z")
	shifts := ResolveLayerRange(layer, tz, from, to)
	// Tail of the prior weekend (Mon 00:00-09:00) + Fri 17:00 -> Mon 09:00... clamped at `to` Mon 00:00? No: to is Tue 00:00. 2026-06-05 is Friday.
	if len(shifts) != 2 {
		t.Fatalf("expected 2 shifts, got %d: %+v", len(shifts), shifts)
	}
	assertShift(t, shifts[0], 4, utc(t, "2026-06-01T00:00:00Z"), utc(t, "2026-06-01T09:00:00Z"))
	assertShift(t, shifts[1], 4, utc(t, "2026-06-05T17:00:00Z"), utc(t, "2026-06-08T09:00:00Z"))
}

// A weekly window that lands inside a spring-forward gap covers nothing that
// week. Only that week may be lost: bailing out of the expansion would leave
// the rest of the month with no coverage at all.
func TestWeeklyRestrictionSurvivesADSTGapWeek(t *testing.T) {
	tz := mustLoad(t, "America/New_York")
	layer := &models.OncallLayer{
		Id: "l_1", Name: "Sunday night", RotationType: RotationWeekly, HandoffTime: "09:00", HandoffDay: 1,
		RotationStart: "2026-02-01", UserIds: []int{4},
		// Sunday 01:30 -> 02:30 local; on 2026-03-08 that hour does not exist.
		Restrictions: []models.OncallRestriction{{Type: RestrictionWeekly, StartDay: 7, StartTime: "01:30", EndDay: 7, EndTime: "02:30"}},
	}
	from := utc(t, "2026-03-01T00:00:00Z")
	to := utc(t, "2026-04-01T00:00:00Z")
	shifts := ResolveLayerRange(layer, tz, from, to)
	// Five Sundays in the range, minus the one swallowed by the DST gap.
	if len(shifts) != 4 {
		t.Fatalf("expected 4 weekly windows (5 Sundays less the DST-gap week), got %d: %+v", len(shifts), shifts)
	}
}

func TestHandoffInsideRestrictionChangesUser(t *testing.T) {
	tz := mustLoad(t, "UTC")
	layer := &models.OncallLayer{
		Id: "l_1", Name: "Days", RotationType: RotationDaily, HandoffTime: "12:00",
		RotationStart: "2026-06-01", UserIds: []int{1, 2},
		Restrictions: []models.OncallRestriction{{Type: RestrictionDaily, StartTime: "09:00", EndTime: "17:00"}},
	}
	from := utc(t, "2026-06-01T00:00:00Z")
	to := utc(t, "2026-06-02T00:00:00Z")
	shifts := ResolveLayerRange(layer, tz, from, to)
	if len(shifts) != 2 {
		t.Fatalf("expected 2 shifts (split at the 12:00 handoff), got %d: %+v", len(shifts), shifts)
	}
	// The 09:00-12:00 stretch belongs to the period that began May 31 12:00
	// (index -1, floor-mod -> user 2); the handoff at 12:00 rotates to user 1.
	assertShift(t, shifts[0], 2, utc(t, "2026-06-01T09:00:00Z"), utc(t, "2026-06-01T12:00:00Z"))
	assertShift(t, shifts[1], 1, utc(t, "2026-06-01T12:00:00Z"), utc(t, "2026-06-01T17:00:00Z"))
}

func TestLayerStackingLaterLayerWins(t *testing.T) {
	tz := mustLoad(t, "UTC")
	def := &models.OncallScheduleDefinition{
		SchemaVersion: 1,
		Layers: []models.OncallLayer{
			baseLayer("l_base", []int{1}),
			daytimeLayer("l_week", []int{2}),
		},
	}
	from := utc(t, "2026-06-01T09:00:00Z") // Monday 09:00
	to := utc(t, "2026-06-02T09:00:00Z")
	shifts := ResolveRange(def, tz, nil, from, to)
	// Weekday layer covers Mon 09:00-17:00, base fills 17:00-Tue 09:00.
	if len(shifts) != 2 {
		t.Fatalf("expected 2 shifts, got %d: %+v", len(shifts), shifts)
	}
	assertShift(t, shifts[0], 2, utc(t, "2026-06-01T09:00:00Z"), utc(t, "2026-06-01T17:00:00Z"))
	assertShift(t, shifts[1], 1, utc(t, "2026-06-01T17:00:00Z"), utc(t, "2026-06-02T09:00:00Z"))
}

func TestOverrideSplitsShift(t *testing.T) {
	tz := mustLoad(t, "UTC")
	def := &models.OncallScheduleDefinition{
		SchemaVersion: 1,
		Layers:        []models.OncallLayer{baseLayer("l_base", []int{1})},
	}
	overrides := []*models.OncallOverride{{
		Id: 1, UserId: 9,
		StartAt:   utc(t, "2026-06-01T12:00:00Z"),
		EndAt:     utc(t, "2026-06-01T14:00:00Z"),
		CreatedAt: utc(t, "2026-05-30T00:00:00Z"),
	}}
	from := utc(t, "2026-06-01T09:00:00Z")
	to := utc(t, "2026-06-01T17:00:00Z")
	shifts := ResolveRange(def, tz, overrides, from, to)
	if len(shifts) != 3 {
		t.Fatalf("expected 3 shifts, got %d: %+v", len(shifts), shifts)
	}
	assertShift(t, shifts[0], 1, utc(t, "2026-06-01T09:00:00Z"), utc(t, "2026-06-01T12:00:00Z"))
	assertShift(t, shifts[1], 9, utc(t, "2026-06-01T12:00:00Z"), utc(t, "2026-06-01T14:00:00Z"))
	assertShift(t, shifts[2], 1, utc(t, "2026-06-01T14:00:00Z"), utc(t, "2026-06-01T17:00:00Z"))
	if !shifts[1].IsOverride {
		t.Error("middle shift should be an override")
	}
}

func TestOverlappingOverridesLatestCreatedWins(t *testing.T) {
	tz := mustLoad(t, "UTC")
	def := &models.OncallScheduleDefinition{
		SchemaVersion: 1,
		Layers:        []models.OncallLayer{baseLayer("l_base", []int{1})},
	}
	overrides := []*models.OncallOverride{
		{Id: 1, UserId: 8, StartAt: utc(t, "2026-06-01T10:00:00Z"), EndAt: utc(t, "2026-06-01T16:00:00Z"), CreatedAt: utc(t, "2026-05-29T00:00:00Z")},
		{Id: 2, UserId: 9, StartAt: utc(t, "2026-06-01T12:00:00Z"), EndAt: utc(t, "2026-06-01T14:00:00Z"), CreatedAt: utc(t, "2026-05-30T00:00:00Z")},
	}
	from := utc(t, "2026-06-01T09:00:00Z")
	to := utc(t, "2026-06-01T17:00:00Z")
	shifts := ResolveRange(def, tz, overrides, from, to)
	if len(shifts) != 5 {
		t.Fatalf("expected 5 shifts, got %d: %+v", len(shifts), shifts)
	}
	assertShift(t, shifts[1], 8, utc(t, "2026-06-01T10:00:00Z"), utc(t, "2026-06-01T12:00:00Z"))
	assertShift(t, shifts[2], 9, utc(t, "2026-06-01T12:00:00Z"), utc(t, "2026-06-01T14:00:00Z"))
	assertShift(t, shifts[3], 8, utc(t, "2026-06-01T14:00:00Z"), utc(t, "2026-06-01T16:00:00Z"))
}

func TestEmptyDefinitionResolvesToNobody(t *testing.T) {
	tz := mustLoad(t, "UTC")
	def := &models.OncallScheduleDefinition{SchemaVersion: 1}
	if shifts := ResolveRange(def, tz, nil, utc(t, "2026-06-01T00:00:00Z"), utc(t, "2026-06-02T00:00:00Z")); len(shifts) != 0 {
		t.Errorf("expected no shifts, got %+v", shifts)
	}
	if users := ResolveAt(def, tz, nil, utc(t, "2026-06-01T00:00:00Z")); len(users) != 0 {
		t.Errorf("expected nobody on call, got %v", users)
	}
}

// ResolveAt must agree with the timeline ResolveRange renders: whoever the
// stack puts on top, and nobody else. Anything looser pages the people an
// override or a higher layer was meant to relieve.
func TestResolveAtAppliesFullPrecedence(t *testing.T) {
	tz := mustLoad(t, "UTC")
	def := &models.OncallScheduleDefinition{
		SchemaVersion: 1,
		Layers: []models.OncallLayer{
			baseLayer("l_base", []int{1}),
			daytimeLayer("l_week", []int{2}),
		},
	}
	overrides := []*models.OncallOverride{{
		Id: 1, UserId: 9,
		StartAt:   utc(t, "2026-06-01T00:00:00Z"),
		EndAt:     utc(t, "2026-06-02T00:00:00Z"),
		CreatedAt: utc(t, "2026-05-30T00:00:00Z"),
	}}
	// Monday 10:00: override beats the weekday layer, which beats the base.
	users := ResolveAt(def, tz, overrides, utc(t, "2026-06-01T10:00:00Z"))
	if len(users) != 1 || users[0] != 9 {
		t.Errorf("expected only the override user [9], got %v", users)
	}
	// Same instant with no override: the weekday layer covers, not the base.
	users = ResolveAt(def, tz, nil, utc(t, "2026-06-01T10:00:00Z"))
	if len(users) != 1 || users[0] != 2 {
		t.Errorf("expected only the weekday layer user [2], got %v", users)
	}
	// Monday 20:00: weekday layer restricted out, base takes over.
	users = ResolveAt(def, tz, nil, utc(t, "2026-06-01T20:00:00Z"))
	if len(users) != 1 || users[0] != 1 {
		t.Errorf("expected [1], got %v", users)
	}
}

func TestCurrentAndNext(t *testing.T) {
	tz := mustLoad(t, "UTC")
	def := &models.OncallScheduleDefinition{
		SchemaVersion: 1,
		Layers: []models.OncallLayer{{
			Id: "l_1", Name: "Primary", RotationType: RotationDaily, HandoffTime: "09:00",
			RotationStart: "2026-06-01", UserIds: []int{1, 2},
		}},
	}
	now := utc(t, "2026-06-01T12:00:00Z")
	current, next := CurrentAndNext(def, tz, nil, now)
	if current == nil || current.UserId != 1 {
		t.Fatalf("current = %+v, want user 1", current)
	}
	if next == nil || next.UserId != 2 || !next.Start.Equal(utc(t, "2026-06-02T09:00:00Z")) {
		t.Fatalf("next = %+v, want user 2 starting 06-02 09:00", next)
	}
}

func TestParseDefinitionValidation(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{"bad json", `{`},
		{"bad schema version", `{"schemaVersion": 99}`},
		{"missing layer name", `{"layers":[{"rotationType":"daily","handoffTime":"09:00","rotationStart":"2026-06-01","userIds":[1]}]}`},
		{"bad rotation type", `{"layers":[{"name":"A","rotationType":"hourly","handoffTime":"09:00","rotationStart":"2026-06-01","userIds":[1]}]}`},
		{"bad handoff time", `{"layers":[{"name":"A","rotationType":"daily","handoffTime":"9am","rotationStart":"2026-06-01","userIds":[1]}]}`},
		{"bad rotation start", `{"layers":[{"name":"A","rotationType":"daily","handoffTime":"09:00","rotationStart":"June 1","userIds":[1]}]}`},
		{"no members", `{"layers":[{"name":"A","rotationType":"daily","handoffTime":"09:00","rotationStart":"2026-06-01","userIds":[]}]}`},
		{"duplicate members", `{"layers":[{"name":"A","rotationType":"daily","handoffTime":"09:00","rotationStart":"2026-06-01","userIds":[1,1]}]}`},
		{"weekly without handoff day", `{"layers":[{"name":"A","rotationType":"weekly","handoffTime":"09:00","rotationStart":"2026-06-01","userIds":[1]}]}`},
		{"custom without interval", `{"layers":[{"name":"A","rotationType":"custom","handoffTime":"09:00","rotationStart":"2026-06-01","userIds":[1]}]}`},
		{"bad restriction type", `{"layers":[{"name":"A","rotationType":"daily","handoffTime":"09:00","rotationStart":"2026-06-01","userIds":[1],"restrictions":[{"type":"monthly","startTime":"09:00","endTime":"17:00"}]}]}`},
		{"weekly restriction bad day", `{"layers":[{"name":"A","rotationType":"daily","handoffTime":"09:00","rotationStart":"2026-06-01","userIds":[1],"restrictions":[{"type":"weekly","startDay":0,"endDay":5,"startTime":"09:00","endTime":"17:00"}]}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseDefinition([]byte(tc.json)); err == nil {
				t.Errorf("expected validation error for %s", tc.name)
			}
		})
	}
}

func TestParseDefinitionAssignsLayerIds(t *testing.T) {
	def, err := ParseDefinition([]byte(`{"layers":[{"name":"A","rotationType":"daily","handoffTime":"09:00","rotationStart":"2026-06-01","userIds":[1]}]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.Layers[0].Id == "" {
		t.Error("expected a generated layer id")
	}
	if def.SchemaVersion != SchemaVersion {
		t.Errorf("schemaVersion = %d, want %d", def.SchemaVersion, SchemaVersion)
	}
}
