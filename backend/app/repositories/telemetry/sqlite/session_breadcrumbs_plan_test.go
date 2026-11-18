//go:build !telemetry_ch && !telemetry_duckdb

package sqlite

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/dbtest"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/sqlitetypes"
)

func queryPlan(t *testing.T, query string, args ...any) string {
	t.Helper()
	rows, err := db.TelemetryDB.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return strings.Join(plan, "; ")
}

// The event-name filter and either read direction must keep both session reads on their expression indexes; the log
// read also walks the index in timestamp order, so it never sorts.
func TestSessionBreadcrumbReadsUseTheSessionIndexes(t *testing.T) {
	dbtest.SetupSQLite(t)
	now := time.Now().UTC()
	for _, order := range []string{"ASC", "DESC"} {
		query, args, err := lit.ParseNamedQuery(lit.SQLite, sessionLogRecordsQuery(order), lit.P{"project_id": uuid.New(), "session_id": uuid.NewString(),
			"from": sqlitetypes.NewSQLiteTime(now.Add(-time.Hour)), "to": sqlitetypes.NewSQLiteTime(now), "limit": 2000})
		if err != nil {
			t.Fatal(err)
		}
		plan := queryPlan(t, query, args...)
		if !strings.Contains(plan, "USING INDEX idx_log_records_session") || strings.Contains(plan, "TEMP B-TREE") {
			t.Errorf("log read %s: %s", order, plan)
		}

		plan = queryPlan(t, sessionClientSpansQuery(order), uuid.NewString(), uuid.NewString(),
			sqlitetypes.NewSQLiteTime(now.Add(-time.Hour)), sqlitetypes.NewSQLiteTime(now), 2000)
		if !strings.Contains(plan, "USING INDEX idx_spans_v2_session") {
			t.Errorf("span read %s: %s", order, plan)
		}
	}
}
