package telemetry

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestSessionTraceIdentityRoundTrip(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	const traceID = "0102030405060708090a0b0c0d0e0f10"
	session := models.Session{Id: uuid.New(), ProjectId: uuid.New(), StartedAt: now, TraceId: traceID}
	if err := SessionRepository.Upsert(ctx, []models.Session{session}); err != nil {
		t.Fatal(err)
	}
	got, err := SessionRepository.FindById(ctx, session.ProjectId, session.Id, &now)
	if err != nil || got == nil || got.TraceId != traceID {
		t.Fatalf("session trace did not round trip: %+v: %v", got, err)
	}
	encoded, err := json.Marshal(got)
	if err != nil || !strings.Contains(string(encoded), `"traceId":"`+traceID+`"`) || strings.Contains(string(encoded), "distributedTraceId") {
		t.Fatalf("session JSON must expose the canonical trace ID: %s: %v", encoded, err)
	}
	rows, total, err := SessionRepository.FindAll(ctx, session.ProjectId, now.Add(-time.Second), now.Add(time.Second), 1, 10, "started_at", "asc", "", nil)
	if err != nil || total != 1 || len(rows) != 1 || rows[0].TraceId != traceID {
		t.Fatalf("session search lost trace identity: %+v total=%d: %v", rows, total, err)
	}
}

func TestSessionRepository_SearchAttributes(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectID := uuid.New()
	userID := uuid.New().String()
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	alice := models.Session{Id: uuid.New(), ProjectId: projectID, StartedAt: now, ClientIP: "192.0.2.1", Attributes: map[string]string{"user.id": "u_42", "user.uuid": userID, "email": "Alice@Example.com", "tenant": "acme", "empty": "", `odd."key\`: "literal%_value"}}
	bob := models.Session{Id: uuid.New(), ProjectId: projectID, StartedAt: now.Add(time.Minute), Attributes: map[string]string{"user.id": "u_43", "email": "bob@example.com", "tenant": "other"}}
	anonymous := models.Session{Id: uuid.New(), ProjectId: projectID, StartedAt: now.Add(2 * time.Minute)}
	otherProject := alice
	otherProject.Id, otherProject.ProjectId = uuid.New(), uuid.New()
	outsideRange := alice
	outsideRange.Id, outsideRange.StartedAt = uuid.New(), now.Add(-24*time.Hour)
	if err := SessionRepository.Upsert(ctx, []models.Session{alice, bob, anonymous, otherProject, outsideRange}); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, search string
		filters      []SessionAttributeFilter
		want         int64
	}{
		{name: "unfiltered", want: 3},
		{name: "full UUID", search: alice.Id.String(), want: 1},
		{name: "short UUID", search: alice.Id.String()[:8], want: 1},
		{name: "UUID user search", search: userID, want: 1},
		{name: "user search", search: "u_42", want: 1},
		{name: "case insensitive search", search: "ALICE@EXAMPLE", want: 1},
		{name: "unknown search", search: "nobody", want: 0},
		{name: "IP", search: "192.0.2.1", want: 1},
		{name: "exact dotted key", filters: []SessionAttributeFilter{{Key: "user.id", Value: "u_42"}}, want: 1},
		{name: "case sensitive equals", filters: []SessionAttributeFilter{{Key: "email", Value: "alice@example.com"}}, want: 0},
		{name: "contains", filters: []SessionAttributeFilter{{Key: "email", Value: "ALICE@", Contains: true}}, want: 1},
		{name: "exclude includes absent", filters: []SessionAttributeFilter{{Key: "user.id", Value: "u_42", Exclude: true}}, want: 2},
		{name: "not contains", filters: []SessionAttributeFilter{{Key: "email", Value: "EXAMPLE", Contains: true, Exclude: true}}, want: 1},
		{name: "empty differs from missing", filters: []SessionAttributeFilter{{Key: "empty", Value: ""}}, want: 1},
		{name: "exclude empty", filters: []SessionAttributeFilter{{Key: "empty", Value: "", Exclude: true}}, want: 2},
		{name: "contains empty requires key", filters: []SessionAttributeFilter{{Key: "empty", Value: "", Contains: true}}, want: 1},
		{name: "literal special key", filters: []SessionAttributeFilter{{Key: `odd."key\`, Value: "literal%_value"}}, want: 1},
		{name: "literal contains", filters: []SessionAttributeFilter{{Key: `odd."key\`, Value: "%_", Contains: true}}, want: 1},
		{name: "SQL in key", filters: []SessionAttributeFilter{{Key: `' OR 1=1 --`, Value: ""}}, want: 0},
		{name: "AND filters", filters: []SessionAttributeFilter{{Key: "user.id", Value: "u_42"}, {Key: "tenant", Value: "other"}}, want: 0},
		{name: "search AND filter", search: "bob", filters: []SessionAttributeFilter{{Key: "tenant", Value: "acme"}}, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, total, err := SessionRepository.FindAll(ctx, projectID, now.Add(-time.Hour), now.Add(time.Hour), 1, 1, "started_at", "desc", tc.search, tc.filters)
			if err != nil {
				t.Fatal(err)
			}
			if total != tc.want {
				t.Fatalf("total = %d, want %d", total, tc.want)
			}
			if len(rows) != int(min(tc.want, 1)) {
				t.Fatalf("page length = %d, total = %d", len(rows), total)
			}
		})
	}
}

func TestSessionRepository_EndFromRecordingActivity(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectID := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	ptr := func(v time.Time) *time.Time { return &v }

	idle := models.Session{Id: uuid.New(), ProjectId: projectID, StartedAt: ago(60 * time.Minute)}
	closedByPagehide := models.Session{Id: uuid.New(), ProjectId: projectID, StartedAt: ago(40 * time.Minute), EndedAt: ptr(ago(38 * time.Minute)), Duration: int64(2 * time.Minute)}
	closedByLateTimer := models.Session{Id: uuid.New(), ProjectId: projectID, StartedAt: ago(90 * time.Minute), EndedAt: ptr(ago(30 * time.Minute)), Duration: int64(60 * time.Minute)}
	live := models.Session{Id: uuid.New(), ProjectId: projectID, StartedAt: ago(10 * time.Minute)}
	unrecorded := models.Session{Id: uuid.New(), ProjectId: projectID, StartedAt: ago(70 * time.Minute)}
	skewedClock := models.Session{Id: uuid.New(), ProjectId: projectID, StartedAt: ago(30 * time.Minute), EndedAt: ptr(ago(16 * time.Minute)), Duration: int64(14 * time.Minute)}
	if err := SessionRepository.Upsert(ctx, []models.Session{idle, closedByPagehide, closedByLateTimer, live, unrecorded, skewedClock}); err != nil {
		t.Fatal(err)
	}

	segment := func(s models.Session, index int32, end time.Time) models.SessionRecording {
		return models.SessionRecording{Id: uuid.New(), ProjectId: projectID, SessionId: &s.Id, SegmentIndex: index, FilePath: "k", RecordedAt: end, EndedAt: ptr(end)}
	}
	if err := SessionRecordingRepository.InsertAsync(ctx, []models.SessionRecording{
		segment(idle, 0, ago(55*time.Minute)),
		segment(idle, 1, ago(50*time.Minute)),
		segment(closedByPagehide, 0, ago(38*time.Minute-400*time.Millisecond)),
		segment(closedByLateTimer, 0, ago(85*time.Minute)),
		segment(live, 0, ago(time.Minute)),
		segment(skewedClock, 0, ago(32*time.Minute)),
	}); err != nil {
		t.Fatal(err)
	}

	want := []struct {
		session  models.Session
		duration time.Duration
		ended    bool
		recorded bool
	}{
		{skewedClock, 14 * time.Minute, true, true},
		{idle, 10 * time.Minute, true, true},
		{live, 9 * time.Minute, false, true},
		{closedByLateTimer, 5 * time.Minute, true, true},
		{closedByPagehide, 2 * time.Minute, true, true},
		{unrecorded, 0, false, false},
	}

	rows, total, err := SessionRepository.FindAll(ctx, projectID, ago(2*time.Hour), now, 1, 10, "duration", "desc", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != int64(len(want)) || len(rows) != len(want) {
		t.Fatalf("got %d rows (total %d), want %d", len(rows), total, len(want))
	}
	for i, w := range want {
		got := rows[i]
		if got.Id != w.session.Id {
			t.Fatalf("row %d is %s, want %s", i, got.Id, w.session.Id)
		}
		if time.Duration(got.Duration).Round(time.Second) != w.duration || (got.EndedAt != nil) != w.ended || got.HasRecording != w.recorded {
			t.Fatalf("row %d: duration=%v ended=%v recorded=%v, want %v %v %v", i, time.Duration(got.Duration), got.EndedAt != nil, got.HasRecording, w.duration, w.ended, w.recorded)
		}

		byId, err := SessionRepository.FindById(ctx, projectID, w.session.Id, &w.session.StartedAt)
		if err != nil || byId == nil {
			t.Fatalf("FindById(%s): %+v: %v", w.session.Id, byId, err)
		}
		if byId.Duration != got.Duration || (byId.EndedAt != nil) != w.ended || byId.HasRecording != w.recorded {
			t.Fatalf("FindById disagrees with FindAll for row %d: %+v vs %+v", i, byId, got)
		}
	}

	asc, _, err := SessionRepository.FindAll(ctx, projectID, ago(2*time.Hour), now, 1, 10, "duration", "asc", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := range asc {
		if asc[i].Id != want[len(want)-1-i].session.Id {
			t.Fatalf("ascending row %d is %s, want %s", i, asc[i].Id, want[len(want)-1-i].session.Id)
		}
	}
}
