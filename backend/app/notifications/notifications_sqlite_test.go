//go:build !pgch

package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	_ "modernc.org/sqlite"
)

func TestCountOccurrences(t *testing.T) {
	result := countOccurrences([]string{"a", "b", "a", "c", "a", "b"})
	if len(result) != 3 {
		t.Fatalf("expected 3 distinct hashes, got %d", len(result))
	}
	if result["a"] != 3 {
		t.Errorf("expected a counted 3 times, got %d", result["a"])
	}
	if result["b"] != 2 {
		t.Errorf("expected b counted 2 times, got %d", result["b"])
	}
	if result["c"] != 1 {
		t.Errorf("expected c counted 1 time, got %d", result["c"])
	}

	unique := countOccurrences([]string{"x", "y"})
	if len(unique) != 2 || unique["x"] != 1 || unique["y"] != 1 {
		t.Errorf("unique hashes not preserved: %v", unique)
	}

	empty := countOccurrences(nil)
	if len(empty) != 0 {
		t.Errorf("expected empty map for empty slice, got %v", empty)
	}
}

func TestCooldownSeed(t *testing.T) {
	tracker := &cooldownTracker{fired: make(map[int]time.Time)}
	t1 := time.Now().Add(-2 * time.Hour)
	t2 := time.Now().Add(-1 * time.Hour)

	tracker.seed(map[int]time.Time{1: t1, 2: t2})
	if got := tracker.fired[1]; !got.Equal(t1) {
		t.Errorf("seed on empty tracker: fired[1] = %v, expected %v", got, t1)
	}
	if got := tracker.fired[2]; !got.Equal(t2) {
		t.Errorf("seed on empty tracker: fired[2] = %v, expected %v", got, t2)
	}

	tracker.recordFire(3)
	recorded := tracker.fired[3]
	tracker.seed(map[int]time.Time{3: time.Now().Add(-3 * time.Hour)})
	if got := tracker.fired[3]; !got.Equal(recorded) {
		t.Errorf("seed overwrote newer recordFire time: got %v, expected %v", got, recorded)
	}

	newer := time.Now().Add(time.Hour)
	tracker.seed(map[int]time.Time{1: newer})
	if got := tracker.fired[1]; !got.Equal(newer) {
		t.Errorf("seed did not update with newer time: got %v, expected %v", got, newer)
	}
}

func TestWindowP99MatchesNearestRank(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "p99.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer conn.Close()
	conn.SetMaxOpenConns(1)

	if _, err := conn.Exec(`CREATE TABLE endpoints (
		endpoint TEXT NOT NULL,
		duration INTEGER NOT NULL,
		project_id TEXT NOT NULL,
		recorded_at DATETIME NOT NULL
	)`); err != nil {
		t.Fatalf("failed to create endpoints table: %v", err)
	}

	projectId := uuid.New().String()
	now := time.Now().UTC()
	from := now.Add(-time.Hour)
	ts := now.Add(-30 * time.Minute).Format(time.RFC3339Nano)

	durations := map[string][]int64{}
	for i := 0; i < 100; i++ {
		j := (i * 37) % 100
		durations["GET /alpha"] = append(durations["GET /alpha"], int64(1000+j*10))
		durations["GET /beta"] = append(durations["GET /beta"], int64(5000+j*37))
	}

	for ep, ds := range durations {
		for _, d := range ds {
			if _, err := conn.Exec(
				"INSERT INTO endpoints (endpoint, duration, project_id, recorded_at) VALUES (?, ?, ?, ?)",
				ep, d, projectId, ts); err != nil {
				t.Fatalf("failed to insert row: %v", err)
			}
		}
	}

	rows, err := conn.Query(`SELECT endpoint, duration FROM (
		SELECT endpoint, duration,
			ROW_NUMBER() OVER (PARTITION BY endpoint ORDER BY duration) AS rn,
			COUNT(*) OVER (PARTITION BY endpoint) AS cnt
		FROM endpoints WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
	) WHERE rn = CAST(0.99 * (cnt - 1) AS INTEGER) + 1`,
		projectId, from.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("window query failed: %v", err)
	}
	defer rows.Close()

	got := map[string]int64{}
	for rows.Next() {
		var ep string
		var d int64
		if err := rows.Scan(&ep, &d); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		got[ep] = d
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected p99 for 2 endpoints, got %d: %v", len(got), got)
	}

	for ep, ds := range durations {
		sorted := append([]int64(nil), ds...)
		sort.Slice(sorted, func(a, b int) bool { return sorted[a] < sorted[b] })
		expected := sorted[int(0.99*float64(len(sorted)-1))]
		if got[ep] != expected {
			t.Errorf("endpoint %s: window p99 = %d, nearest-rank-lower = %d", ep, got[ep], expected)
		}
	}
}

func setupNotificationsTestDB(t *testing.T) {
	t.Helper()

	prevDB := db.DB
	prevTelemetryDB := db.TelemetryDB
	prevDriver := db.Driver
	t.Cleanup(func() {
		db.DB = prevDB
		db.TelemetryDB = prevTelemetryDB
		db.Driver = prevDriver
	})

	dir := t.TempDir()

	mainDB, err := sql.Open("sqlite", filepath.Join(dir, "main.db"))
	if err != nil {
		t.Fatalf("failed to open main sqlite: %v", err)
	}
	mainDB.SetMaxOpenConns(1)

	telemetryDB, err := sql.Open("sqlite", filepath.Join(dir, "telemetry.db"))
	if err != nil {
		t.Fatalf("failed to open telemetry sqlite: %v", err)
	}
	telemetryDB.SetMaxOpenConns(1)

	t.Cleanup(func() {
		mainDB.Close()
		telemetryDB.Close()
	})

	db.DB = mainDB
	db.TelemetryDB = telemetryDB
	db.Driver = lit.SQLite
	models.Init(db.Driver)

	if _, err := telemetryDB.Exec(`CREATE TABLE tasks (
		id TEXT NOT NULL,
		project_id TEXT NOT NULL,
		task_name TEXT NOT NULL DEFAULT '',
		duration INTEGER NOT NULL DEFAULT 0,
		recorded_at DATETIME NOT NULL
	)`); err != nil {
		t.Fatalf("failed to create tasks table: %v", err)
	}

	if _, err := telemetryDB.Exec(`CREATE TABLE exception_stack_traces (
		id TEXT NOT NULL,
		project_id TEXT NOT NULL,
		trace_id TEXT,
		trace_type TEXT NOT NULL DEFAULT 'endpoint',
		recorded_at DATETIME NOT NULL
	)`); err != nil {
		t.Fatalf("failed to create exception_stack_traces table: %v", err)
	}
}

func TestEvaluateTaskFailureRate(t *testing.T) {
	setupNotificationsTestDB(t)

	ctx := context.Background()
	projectId := uuid.New()
	pid := projectId.String()
	now := time.Now().UTC()

	var taskIds []string
	for i := 0; i < 5; i++ {
		id := uuid.New().String()
		taskIds = append(taskIds, id)
		ts := now.Add(-time.Duration(i+1) * time.Minute).Format(time.RFC3339Nano)
		if _, err := db.TelemetryDB.Exec(
			"INSERT INTO tasks (id, project_id, task_name, duration, recorded_at) VALUES (?, ?, ?, ?, ?)",
			id, pid, "etl-job", 1000000, ts); err != nil {
			t.Fatalf("failed to insert task: %v", err)
		}
	}

	for i := 0; i < 2; i++ {
		ts := now.Add(-time.Duration(i+1) * time.Minute).Format(time.RFC3339Nano)
		if _, err := db.TelemetryDB.Exec(
			"INSERT INTO exception_stack_traces (id, project_id, trace_id, trace_type, recorded_at) VALUES (?, ?, ?, ?, ?)",
			uuid.New().String(), pid, taskIds[i], "task", ts); err != nil {
			t.Fatalf("failed to insert exception: %v", err)
		}
	}

	t.Run("fires above threshold", func(t *testing.T) {
		rule := &models.NotificationRule{
			Config: json.RawMessage(`{"taskName":"etl-job","thresholdPercent":20,"lookbackMinutes":60,"minExecutions":5}`),
		}
		result, err := evaluateTaskFailureRate(ctx, rule, projectId)
		if err != nil {
			t.Fatalf("evaluateTaskFailureRate failed: %v", err)
		}
		if !result.Fired {
			t.Fatal("expected rule to fire")
		}
		if !strings.Contains(result.Message.Subject, "40.0%") {
			t.Errorf("subject %q does not contain 40.0%%", result.Message.Subject)
		}
		if !strings.Contains(result.Message.Body, "2 of 5") {
			t.Errorf("body %q does not contain 2 of 5", result.Message.Body)
		}
	})

	t.Run("gated below min executions", func(t *testing.T) {
		rule := &models.NotificationRule{
			Config: json.RawMessage(`{"taskName":"etl-job","thresholdPercent":20,"lookbackMinutes":60,"minExecutions":10}`),
		}
		result, err := evaluateTaskFailureRate(ctx, rule, projectId)
		if err != nil {
			t.Fatalf("evaluateTaskFailureRate failed: %v", err)
		}
		if result.Fired {
			t.Error("expected rule not to fire below minExecutions")
		}
	})

	t.Run("wildcard task name", func(t *testing.T) {
		rule := &models.NotificationRule{
			Config: json.RawMessage(`{"taskName":"*","thresholdPercent":20,"lookbackMinutes":60,"minExecutions":5}`),
		}
		result, err := evaluateTaskFailureRate(ctx, rule, projectId)
		if err != nil {
			t.Fatalf("evaluateTaskFailureRate failed: %v", err)
		}
		if !result.Fired {
			t.Fatal("expected wildcard rule to fire")
		}
		if !strings.Contains(result.Message.Subject, "all tasks") {
			t.Errorf("subject %q does not mention all tasks", result.Message.Subject)
		}
		if !strings.Contains(result.Message.Body, "2 of 5") {
			t.Errorf("body %q does not contain 2 of 5", result.Message.Body)
		}
	})
}
