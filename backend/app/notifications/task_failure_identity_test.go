//go:build !telemetry_ch

package notifications

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
)

func TestRegressionTaskFailuresKeepTraceIdentity(t *testing.T) {
	c, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	previous := db.TelemetryDB
	db.TelemetryDB = c
	defer func() { db.TelemetryDB = previous }()
	for _, q := range []string{"CREATE TABLE tasks_v2 (project_id TEXT,trace_id TEXT,span_id TEXT,task_name TEXT,recorded_at TEXT)", "CREATE TABLE exceptions_v2 (project_id TEXT,trace_id TEXT,span_id TEXT,trace_type TEXT,recorded_at TEXT)"} {
		if _, err := c.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	project := uuid.New()
	at := time.Now().UTC().Truncate(time.Second)
	for _, trace := range []string{"trace-a", "trace-b"} {
		if _, err := c.Exec("INSERT INTO tasks_v2 VALUES (?,?,?,?,?)", project.String(), trace, "reused-span", "task", at.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Exec("INSERT INTO exceptions_v2 VALUES (?,?,?,?,?)", project.String(), trace, "reused-span", "task", at.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := countFailedTaskExecutions(context.Background(), project, "task", true, at.Add(-time.Hour), at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if got != 2 {
		t.Errorf("two failed tasks in different traces counted as %d; want 2", got)
	}
	if _, err := c.Exec("DELETE FROM exceptions_v2 WHERE trace_id = 'trace-a'"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec("UPDATE tasks_v2 SET task_name='other' WHERE trace_id = 'trace-b'"); err != nil {
		t.Fatal(err)
	}
	got, err = countFailedTaskExecutions(context.Background(), project, "task", true, at.Add(-time.Hour), at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Errorf("successful named task inherits another trace's exception: got %d; want 0", got)
	}
}
