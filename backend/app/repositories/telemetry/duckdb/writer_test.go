//go:build telemetry_duckdb

package duckdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
)

func TestWriterPoisonRowDropped(t *testing.T) {
	connector, err := duckdb.NewConnector("", nil)
	if err != nil {
		t.Fatalf("failed to open in-memory duckdb: %v", err)
	}
	sqlDB := sql.OpenDB(connector)
	t.Cleanup(func() { sqlDB.Close() })

	if _, err := sqlDB.Exec(`CREATE TABLE spans (
		id VARCHAR NOT NULL,
		trace_id VARCHAR NOT NULL,
		project_id VARCHAR NOT NULL,
		name VARCHAR NOT NULL DEFAULT '',
		start_time TIMESTAMP NOT NULL,
		duration BIGINT NOT NULL DEFAULT 0,
		recorded_at TIMESTAMP NOT NULL,
		parent_span_id VARCHAR,
		attributes VARCHAR NOT NULL DEFAULT '{}'
	)`); err != nil {
		t.Fatalf("failed to create spans table: %v", err)
	}

	db.DuckDBConnector = connector
	db.TelemetryDB = sqlDB

	StartWriters(context.Background(), WriterOptions{FlushRows: 1 << 20, FlushInterval: time.Hour})
	t.Cleanup(StopWriters)

	_, droppedBefore, _ := db.GetTelemetryIngestCounters()

	ctx := context.Background()
	now := time.Now().UTC()
	good := []driver.Value{"id-1", "trace-1", "project-1", "ok", now, int64(1), now, nil, "{}"}
	poison := []driver.Value{struct{}{}, "trace-1", "project-1", "bad", now, int64(1), now, nil, "{}"}

	if err := insertRows(ctx, "spans", [][]driver.Value{good, poison}); err != nil {
		t.Fatalf("insertRows failed: %v", err)
	}
	if err := FlushWriters(ctx); err != nil {
		t.Fatalf("FlushWriters failed: %v", err)
	}

	var count int
	if err := sqlDB.QueryRow("SELECT COUNT(*) FROM spans").Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 flushed row (poison dropped), got %d", count)
	}

	_, droppedAfter, _ := db.GetTelemetryIngestCounters()
	if droppedAfter != droppedBefore+1 {
		t.Fatalf("expected dropped counter to grow by 1, went %d -> %d", droppedBefore, droppedAfter)
	}
}
