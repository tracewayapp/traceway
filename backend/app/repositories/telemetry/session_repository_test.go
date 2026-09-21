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
