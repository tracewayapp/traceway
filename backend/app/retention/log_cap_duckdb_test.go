//go:build telemetry_duckdb

package retention

import (
	"context"
	"database/sql"
	"testing"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
)

func TestRunLogRecordsCap_DuckDBDeletesOldestBeyondCap(t *testing.T) {
	connector, err := duckdb.NewConnector("", nil)
	if err != nil {
		t.Fatalf("open in-memory duckdb: %v", err)
	}
	telDB := sql.OpenDB(connector)

	if _, err := telDB.Exec("CREATE TABLE log_records (id VARCHAR NOT NULL, timestamp TIMESTAMP NOT NULL)"); err != nil {
		t.Fatalf("ddl: %v", err)
	}
	if _, err := telDB.Exec("INSERT INTO log_records SELECT 'r' || i, TIMESTAMP '2026-01-01' + to_seconds(i) FROM range(15) t(i)"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	prevTelDB, prevDriver := db.TelemetryDB, db.Driver
	db.TelemetryDB = telDB
	db.Driver = lit.SQLite
	t.Cleanup(func() {
		telDB.Close()
		db.TelemetryDB = prevTelDB
		db.Driver = prevDriver
	})

	runLogRecordsCap(context.Background(), 10)

	var n int
	if err := telDB.QueryRow("SELECT count(*) FROM log_records").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 10 {
		t.Errorf("got %d rows after cap, want 10", n)
	}

	// Rows i=5..14 are the 10 newest; nothing older than i=5 may survive.
	var older int
	if err := telDB.QueryRow("SELECT count(*) FROM log_records WHERE timestamp < TIMESTAMP '2026-01-01' + to_seconds(5)").Scan(&older); err != nil {
		t.Fatalf("count older: %v", err)
	}
	if older != 0 {
		t.Errorf("expected 0 rows older than the cap boundary, got %d", older)
	}
}
