//go:build !telemetry_ch && !telemetry_duckdb

package telemetry

import (
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/dbtest"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	dbtest.SetupSQLite(t)
}

// legacyExec writes to the tables V2 replaced, which no repository writes any more.
func legacyExec(t *testing.T, query string, args ...any) {
	t.Helper()
	for i, arg := range args {
		switch value := arg.(type) {
		case uuid.UUID:
			args[i] = value.String()
		case *uuid.UUID:
			args[i] = nil
			if value != nil {
				args[i] = value.String()
			}
		case time.Time:
			args[i] = sqlitetypes.NewSQLiteTime(value)
		}
	}
	if _, err := db.TelemetryDB.Exec(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func legacyReset(t *testing.T) {
	t.Helper()
	for _, statement := range []string{"DELETE FROM endpoints", "DELETE FROM tasks", "DELETE FROM ai_traces", "DELETE FROM exception_stack_traces",
		"DELETE FROM spans"} {
		legacyExec(t, statement)
	}
}

func moveOverRowCount(t *testing.T, table string) int64 {
	t.Helper()
	var count int64
	if err := db.TelemetryDB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
