//go:build telemetry_firebolt

package telemetry

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	_ "github.com/firebolt-db/firebolt-go-sdk"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/migrations"
	"github.com/tracewayapp/traceway/backend/app/models"
)

// setupTestDB runs the shared telemetry test suite against a live Firebolt
// engine, mirroring the ClickHouse helper's opt-in style: without
// TEST_FIREBOLT_URL the suite is skipped. The main DB stays an in-memory
// stand-in only insofar as tests need db.DB — Firebolt tests exercise the
// telemetry repositories, which never touch it.
func setupTestDB(t *testing.T) {
	t.Helper()

	engineURL := os.Getenv("TEST_FIREBOLT_URL")
	if engineURL == "" {
		t.Skip("TEST_FIREBOLT_URL not set, skipping Firebolt tests")
	}

	database := strings.TrimSpace(os.Getenv("TEST_FIREBOLT_DATABASE"))
	dbPart := ""
	if database != "" {
		dbPart = "/" + url.PathEscape(database)
		bootstrap, err := sql.Open("firebolt", fmt.Sprintf("firebolt://?url=%s&client_side_lb=false", url.QueryEscape(engineURL)))
		if err != nil {
			t.Fatalf("failed to open firebolt: %v", err)
		}
		if _, err := bootstrap.Exec("CREATE DATABASE IF NOT EXISTS " + database); err != nil {
			bootstrap.Close()
			t.Fatalf("failed to create test database: %v", err)
		}
		bootstrap.Close()
	}
	dsn := fmt.Sprintf("firebolt://%s?url=%s&client_side_lb=false", dbPart, url.QueryEscape(engineURL))

	d, err := sql.Open("firebolt", dsn)
	if err != nil {
		t.Fatalf("failed to open firebolt: %v", err)
	}
	if err := d.Ping(); err != nil {
		t.Fatalf("failed to ping firebolt at %s: %v", engineURL, err)
	}

	db.TelemetryDB = d
	models.Init(db.Driver)

	if err := migrations.RunTelemetryMigrationsForTest(); err != nil {
		t.Fatalf("firebolt telemetry migrations: %v", err)
	}

	t.Cleanup(func() {
		tables := []string{
			"endpoints", "tasks", "exception_stack_traces",
			"spans", "metric_points", "session_recordings",
			"archived_exceptions", "slow_endpoints", "fired_notifications",
			"ai_traces", "log_records", "sessions",
			"profiles", "profiling_samples", "profiling_stacks", "check_results",
		}
		for _, table := range tables {
			_, _ = d.Exec("TRUNCATE TABLE " + table)
		}
		d.Close()
		db.TelemetryDB = nil
	})
}
