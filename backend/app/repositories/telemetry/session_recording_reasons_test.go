package telemetry

import (
	"context"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"testing"
	"time"
)

func TestSessionRecordingReasons(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	project, other := uuid.New(), uuid.New()
	session := models.Session{Id: uuid.New(), ProjectId: project, StartedAt: now}
	if err := SessionRepository.Upsert(ctx, []models.Session{session}); err != nil {
		t.Fatal(err)
	}
	record := models.LogRecord{ProjectId: project, Timestamp: now, LogAttributes: map[string]string{"event.name": "traceway.trigger.fired", "session.id": session.Id.String(), "traceway.trigger.id": "journey", "traceway.trigger.name": "User opened", "traceway.trigger.replay": "true", "traceway.trigger.steps": `[{"match":{"event":{"type":"click","value":"#open"}}},{"match":{"event":{"type":"log","value":"loaded"}},"withinSeconds":30}]`}}
	if err := StoreSessionRecordingReasons(ctx, []models.LogRecord{record, record}); err != nil {
		t.Fatal(err)
	}
	record.LogAttributes["traceway.trigger.name"] = "Renamed journey"
	if err := StoreSessionRecordingReasons(ctx, []models.LogRecord{record}); err != nil {
		t.Fatal(err)
	}
	record.LogAttributes["traceway.trigger.id"] = "another"
	if err := StoreSessionRecordingReasons(ctx, []models.LogRecord{record}); err != nil {
		t.Fatal(err)
	}
	// A second project's reason must never tag this project's session or influence its filters.
	record.ProjectId = other
	record.LogAttributes["traceway.trigger.id"] = "foreign"
	if err := StoreSessionRecordingReasons(ctx, []models.LogRecord{record}); err != nil {
		t.Fatal(err)
	}
	record.ProjectId = project
	record.LogAttributes["traceway.trigger.id"] = "not-recorded"
	record.LogAttributes["traceway.trigger.replay"] = "false"
	if err := StoreSessionRecordingReasons(ctx, []models.LogRecord{record}); err != nil {
		t.Fatal(err)
	}
	record.LogAttributes["traceway.trigger.replay"] = "true"
	record.LogAttributes["traceway.trigger.steps"] = `[{"match":{"event":{"type":"click","value":":has(*)"}}}]`
	if err := StoreSessionRecordingReasons(ctx, []models.LogRecord{record}); err != nil {
		t.Fatal(err)
	}
	options, err := ListSessionRecordingReasons(ctx, project)
	if err != nil || len(options) != 3 {
		t.Fatalf("recording reason options: %+v, %v", options, err)
	}
	sessions := []models.Session{session}
	if err := LoadSessionRecordingReasons(ctx, project, sessions); err != nil {
		t.Fatal(err)
	}
	if len(sessions[0].RecordingReasons) != 3 {
		t.Fatalf("expected three distinct reason snapshots, got %+v", sessions[0].RecordingReasons)
	}
	if sessions[0].RecordingReasons[0].Sequence != "click(#open) → log(loaded)" {
		t.Fatal(sessions[0].RecordingReasons)
	}
	for _, tc := range []struct {
		id   string
		want int64
	}{{"journey", 1}, {"another", 1}, {"foreign", 0}, {"missing", 0}, {"missing,another", 1}, {"journey,another", 1}, {"missing,foreign", 0}} {
		rows, total, err := SessionRepository.FindAll(ctx, project, now.Add(-time.Minute), now.Add(time.Minute), 1, 10, "started_at", "asc", "", []SessionAttributeFilter{{Key: "__recording_trigger", Value: tc.id}})
		if err != nil || total != tc.want || int64(len(rows)) != tc.want {
			t.Fatalf("filter %s: total %d rows %d err %v", tc.id, total, len(rows), err)
		}
	}
}
