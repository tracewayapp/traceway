//go:build !pgch

package clientcontrollers

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/pprof/profile"
	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	_ "modernc.org/sqlite"
)

func setupProfilingDB(t *testing.T) {
	t.Helper()
	telemetryDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	telemetryDB.SetMaxOpenConns(1)
	ddl := []string{
		`CREATE TABLE profiling_stacks (
			project_id TEXT NOT NULL, service_name TEXT NOT NULL DEFAULT '',
			stack_hash INTEGER NOT NULL, stack TEXT NOT NULL DEFAULT '[]',
			last_seen DATETIME NOT NULL, UNIQUE(project_id, service_name, stack_hash))`,
		`CREATE TABLE profiling_samples (
			project_id TEXT NOT NULL, profile_id TEXT NOT NULL, service_name TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT '', start_time DATETIME NOT NULL, end_time DATETIME NOT NULL,
			stack_hash INTEGER NOT NULL, value INTEGER NOT NULL DEFAULT 0, labels TEXT NOT NULL DEFAULT '{}',
			server_name TEXT NOT NULL DEFAULT '', app_version TEXT NOT NULL DEFAULT '',
			trace_id TEXT NOT NULL DEFAULT '', span_id TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE profiles (
			id TEXT NOT NULL, project_id TEXT NOT NULL, recorded_at DATETIME NOT NULL,
			duration INTEGER NOT NULL DEFAULT 0, service_name TEXT NOT NULL DEFAULT '',
			profile_type TEXT NOT NULL DEFAULT '', sample_count INTEGER NOT NULL DEFAULT 0,
			total_value INTEGER NOT NULL DEFAULT 0, server_name TEXT NOT NULL DEFAULT '',
			app_version TEXT NOT NULL DEFAULT '', attributes TEXT NOT NULL DEFAULT '{}',
			storage_key TEXT NOT NULL DEFAULT '', trace_id TEXT NOT NULL DEFAULT '',
			span_id TEXT NOT NULL DEFAULT '', distributed_trace_id TEXT DEFAULT NULL)`,
	}
	for _, stmt := range ddl {
		if _, err := telemetryDB.Exec(stmt); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}
	prevDB, prevDriver := db.TelemetryDB, db.Driver
	db.TelemetryDB = telemetryDB
	db.Driver = lit.SQLite
	t.Cleanup(func() {
		telemetryDB.Close()
		db.TelemetryDB = prevDB
		db.Driver = prevDriver
	})
}

func cpuPprof(t *testing.T) []byte {
	t.Helper()
	main := &profile.Function{ID: 1, Name: "main.main", Filename: "main.go"}
	work := &profile.Function{ID: 2, Name: "main.work", Filename: "work.go"}
	idle := &profile.Function{ID: 3, Name: "main.idle", Filename: "idle.go"}
	lMain := &profile.Location{ID: 1, Line: []profile.Line{{Function: main, Line: 10}}}
	lWork := &profile.Location{ID: 2, Line: []profile.Line{{Function: work, Line: 20}}}
	lIdle := &profile.Location{ID: 3, Line: []profile.Line{{Function: idle, Line: 30}}}
	p := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "samples", Unit: "count"},
			{Type: "cpu", Unit: "nanoseconds"},
		},
		PeriodType:    &profile.ValueType{Type: "cpu", Unit: "nanoseconds"},
		Period:        10_000_000,
		TimeNanos:     1_700_000_000_000_000_000,
		DurationNanos: 30_000_000_000,
		Sample: []*profile.Sample{
			{Location: []*profile.Location{lWork, lMain}, Value: []int64{1, 300}},
			{Location: []*profile.Location{lIdle, lMain}, Value: []int64{1, 100}},
		},
		Function: []*profile.Function{main, work, idle},
		Location: []*profile.Location{lMain, lWork, lIdle},
	}
	var buf bytes.Buffer
	if err := p.Write(&buf); err != nil {
		t.Fatalf("write pprof: %v", err)
	}
	return buf.Bytes()
}

func newIngestContext(t *testing.T, projectId uuid.UUID, projectName, target string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	c.Set(middleware.ProjectIdContextKey, projectId)
	c.Set(middleware.ProjectContextKey, &models.Project{Id: projectId, Name: projectName})
	return c, w
}

func TestProfileIngest_StoresExplodedRows(t *testing.T) {
	setupProfilingDB(t)
	projectId := uuid.New()
	c, w := newIngestContext(t, projectId, "checkout-svc", "/profiles/ingest?serverName=pod-a&appVersion=9.9.9", cpuPprof(t))

	ProfileIngestController.Ingest(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var stacks, samples, profiles int
	db.TelemetryDB.QueryRow("SELECT COUNT(*) FROM profiling_stacks").Scan(&stacks)
	db.TelemetryDB.QueryRow("SELECT COUNT(*) FROM profiling_samples").Scan(&samples)
	db.TelemetryDB.QueryRow("SELECT COUNT(*) FROM profiles").Scan(&profiles)
	if stacks != 2 {
		t.Errorf("profiling_stacks = %d, want 2", stacks)
	}
	if samples != 2 {
		t.Errorf("profiling_samples = %d, want 2", samples)
	}
	if profiles != 1 {
		t.Errorf("profiles = %d, want 1 (cpu only)", profiles)
	}

	var service, server, version string
	db.TelemetryDB.QueryRow("SELECT service_name, server_name, app_version FROM profiles LIMIT 1").Scan(&service, &server, &version)
	if service != "checkout-svc" {
		t.Errorf("service_name = %q, want project-name default %q", service, "checkout-svc")
	}
	if server != "pod-a" || version != "9.9.9" {
		t.Errorf("provenance from query params wrong: server=%q version=%q", server, version)
	}
}

func TestProfileIngest_ExplicitServiceQueryOverridesDefault(t *testing.T) {
	setupProfilingDB(t)
	projectId := uuid.New()
	c, w := newIngestContext(t, projectId, "checkout-svc", "/profiles/ingest?service=billing", cpuPprof(t))

	ProfileIngestController.Ingest(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var service string
	db.TelemetryDB.QueryRow("SELECT service_name FROM profiles LIMIT 1").Scan(&service)
	if service != "billing" {
		t.Errorf("service_name = %q, want explicit query value %q", service, "billing")
	}
}

func TestProfileIngest_RejectsGarbage(t *testing.T) {
	setupProfilingDB(t)
	projectId := uuid.New()
	c, w := newIngestContext(t, projectId, "checkout-svc", "/profiles/ingest", []byte("not a pprof payload"))

	ProfileIngestController.Ingest(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for garbage body", w.Code)
	}
}
