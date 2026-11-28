//go:build !telemetry_ch && !telemetry_duckdb

package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

// TW-3: on ClickHouse (a ReplacingMergeTree read without FINAL, partitioned by month) a trigger revision that fired
// before and after the server relabelled its steps is two rows until a merge, and forever across months. A session
// still gets one reason per trigger revision, which the dashboard keys its chips by. SQLite's primary key cannot
// hold such rows, so the table is rebuilt without it, the way ClickHouse stores them.
func TestLoadSessionRecordingReasonsKeepsOneRowPerRevision(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	for _, statement := range []string{
		"CREATE TABLE reasons_unkeyed AS SELECT * FROM session_recording_reasons WHERE 0",
		"DROP TABLE session_recording_reasons",
		"ALTER TABLE reasons_unkeyed RENAME TO session_recording_reasons",
	} {
		if _, err := db.TelemetryDB.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	project, session := uuid.New(), uuid.New()
	before := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC).UnixMilli()
	for i, sequence := range []string{"event", "exception"} {
		if _, err := db.TelemetryDB.Exec("INSERT INTO session_recording_reasons (project_id, session_id, trigger_id, revision, name, sequence, fired_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			project.String(), session.String(), "default-exception", "r1", "default-exception", sequence, before+int64(i)*60_000); err != nil {
			t.Fatal(err)
		}
	}
	sessions := []models.Session{{Id: session, ProjectId: project}}
	if err := LoadSessionRecordingReasons(ctx, project, sessions); err != nil {
		t.Fatal(err)
	}
	reasons := sessions[0].RecordingReasons
	if len(reasons) != 1 {
		t.Fatalf("want one reason for the trigger revision, got %+v", reasons)
	}
	if reasons[0].FiredAt.UnixMilli() != before || reasons[0].Sequence == "" {
		t.Errorf("reason = %+v, want the first fire", reasons[0])
	}
}
