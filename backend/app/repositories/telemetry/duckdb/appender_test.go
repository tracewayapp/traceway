//go:build telemetry_duckdb

package duckdb

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
)

func openAppenderTestTable(t *testing.T) driver.Conn {
	t.Helper()
	connector, err := duckdb.NewConnector("", nil)
	if err != nil {
		t.Fatal(err)
	}
	previous := db.DuckDBConnector
	db.DuckDBConnector = connector
	conn, err := connector.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn.Close()
		connector.Close()
		db.DuckDBConnector = previous
	})
	if _, err := conn.(driver.ExecerContext).ExecContext(context.Background(), "CREATE TABLE probe (id VARCHAR, n BIGINT)", nil); err != nil {
		t.Fatal(err)
	}
	return conn
}

func probeIds(t *testing.T, conn driver.Conn) string {
	t.Helper()
	rows, err := conn.(driver.QueryerContext).QueryContext(context.Background(), "SELECT coalesce(string_agg(id, ',' ORDER BY id), '') FROM probe", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	values := make([]driver.Value, 1)
	if err := rows.Next(values); err != nil {
		t.Fatal(err)
	}
	return values[0].(string)
}

func TestAppenderKeepsRowsAroundARejectedOne(t *testing.T) {
	conn := openAppenderTestTable(t)
	var rejected error
	err := withAppenderColumns(context.Background(), "probe", []string{"id", "n"}, func(appender *duckdb.Appender) {
		if err := appender.AppendRow("first", int64(1)); err != nil {
			t.Fatal(err)
		}
		rejected = appender.AppendRow("bad", "not a number")
		if err := appender.AppendRow("last", int64(3)); err != nil {
			t.Fatalf("the appender must stay usable after rejecting a row: %v", err)
		}
	})
	if err != nil || rejected == nil {
		t.Fatalf("flush error %v, rejected row error %v", err, rejected)
	}
	if ids := probeIds(t, conn); ids != "first,last" {
		t.Fatalf("rows around a rejected one must persist, got %q", ids)
	}
}

func TestAppenderFailureIsABatchFailure(t *testing.T) {
	openAppenderTestTable(t)
	_, _, before := db.GetTelemetryIngestCounters()
	ran := false
	err := withAppenderColumns(context.Background(), "missing_table", []string{"id"}, func(*duckdb.Appender) { ran = true })
	_, _, after := db.GetTelemetryIngestCounters()
	if err == nil || ran || after != before+1 {
		t.Fatalf("a batch that cannot start must fail and be counted: err=%v ran=%v failures %d -> %d", err, ran, before, after)
	}
}
