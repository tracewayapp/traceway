package dbtrace

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/traceway"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	_ "modernc.org/sqlite"
)

func openTraced(t *testing.T) (*sql.DB, *tracetest.SpanRecorder) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	t.Cleanup(traceway.Install(traceway.Providers{
		TracerProvider: sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)),
	}))
	conn, err := Open("sqlite", ":memory:", Options{System: "sqlite", Namespace: "main"})
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { conn.Close() })
	if _, err := conn.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatal(err)
	}
	return conn, recorder
}

func dbSpans(recorder *tracetest.SpanRecorder) []sdktrace.ReadOnlySpan {
	var out []sdktrace.ReadOnlySpan
	for _, span := range recorder.Ended() {
		if span.SpanKind() == trace.SpanKindClient {
			out = append(out, span)
		}
	}
	return out
}

func attr(span sdktrace.ReadOnlySpan, key string) string {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.Emit()
		}
	}
	return ""
}

func TestQueriesWithoutAParentAreNotTraced(t *testing.T) {
	conn, recorder := openTraced(t)

	if _, err := conn.Exec("INSERT INTO items (name) VALUES ('a')"); err != nil {
		t.Fatal(err)
	}
	if n := len(dbSpans(recorder)); n != 0 {
		t.Fatalf("background queries must not start root spans, got %d", n)
	}
}

func TestContextFreeQueriesInATransactionNestUnderItsSpan(t *testing.T) {
	conn, recorder := openTraced(t)
	ctx, request := traceway.Tracer().Start(context.Background(), "POST /api/items")

	tx, err := conn.BeginTx(context.WithoutCancel(ctx), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("INSERT INTO items (name) VALUES (?)", "a"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM items").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	request.End()

	spans := dbSpans(recorder)
	if len(spans) != 2 {
		t.Fatalf("want 2 statement spans, got %d", len(spans))
	}
	for _, span := range spans {
		if span.Parent().SpanID() != request.SpanContext().SpanID() {
			t.Fatalf("%s is not a child of the request span", span.Name())
		}
	}
	if spans[0].Name() != "INSERT items" || attr(spans[0], "db.collection.name") != "items" || attr(spans[0], "db.system.name") != "sqlite" || attr(spans[0], "db.namespace") != "main" {
		t.Fatalf("got %s %q %q", spans[0].Name(), attr(spans[0], "db.system.name"), attr(spans[0], "db.namespace"))
	}
}

func TestRepeatedStatementsInATransactionFoldIntoOneSpan(t *testing.T) {
	conn, recorder := openTraced(t)
	ctx, request := traceway.Tracer().Start(context.Background(), "POST /api/otel/v1/logs")

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 500; i++ {
		if _, err := tx.Exec("INSERT INTO items (name) VALUES (?)", "row"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec("DELETE FROM items WHERE name = 'nothing'"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	request.End()

	spans := dbSpans(recorder)
	if len(spans) != 2 {
		t.Fatalf("500 identical inserts must fold into one span, got %d spans", len(spans))
	}
	if attr(spans[0], "db.operation.batch.size") != "500" || attr(spans[1], "db.operation.batch.size") != "" {
		t.Fatalf("batch sizes = %q, %q", attr(spans[0], "db.operation.batch.size"), attr(spans[1], "db.operation.batch.size"))
	}
}

func TestAGinContextLendsItsRequestSpan(t *testing.T) {
	conn, recorder := openTraced(t)
	requestCtx, request := traceway.Tracer().Start(context.Background(), "GET /api/items")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/items", nil).WithContext(requestCtx)

	rows, err := conn.QueryContext(c, "SELECT id FROM items")
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	request.End()

	spans := dbSpans(recorder)
	if len(spans) != 1 || spans[0].Parent().SpanID() != request.SpanContext().SpanID() {
		t.Fatalf("the query must be a child of the request span, got %d spans", len(spans))
	}
}

func TestFailedStatementsMarkTheSpanWithoutAnException(t *testing.T) {
	conn, recorder := openTraced(t)
	ctx, request := traceway.Tracer().Start(context.Background(), "GET /api/items")

	if _, err := conn.ExecContext(ctx, "INSERT INTO missing_table VALUES (1)"); err == nil {
		t.Fatal("expected an error")
	}
	request.End()

	span := dbSpans(recorder)[0]
	if span.Status().Code.String() != "Error" {
		t.Fatalf("status = %v", span.Status())
	}
	if len(span.Events()) != 0 {
		t.Fatal("a failed query is not an issue on its own")
	}
}

func TestTruncateKeepsRunesWhole(t *testing.T) {
	if got := truncate("aé", 2); got != "a" {
		t.Fatalf("got %q", got)
	}
}

func TestReadSpansLastUntilTheRowsAreClosed(t *testing.T) {
	conn, recorder := openTraced(t)
	for i := 0; i < 50; i++ {
		if _, err := conn.Exec("INSERT INTO items (name) VALUES ('row')"); err != nil {
			t.Fatal(err)
		}
	}
	ctx, request := traceway.Tracer().Start(context.Background(), "GET /api/items")

	rows, err := conn.QueryContext(ctx, "SELECT name FROM items")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(dbSpans(recorder)); n != 0 {
		t.Fatalf("the read must stay open while rows are being read, %d spans ended", n)
	}
	read := 0
	for rows.Next() {
		read++
	}
	rows.Close()
	request.End()

	spans := dbSpans(recorder)
	if read != 50 || len(spans) != 1 || spans[0].Name() != "SELECT items" {
		t.Fatalf("read %d rows, %d spans", read, len(spans))
	}
}

func TestColumnTypesSurviveTheWrapper(t *testing.T) {
	conn, _ := openTraced(t)
	ctx, request := traceway.Tracer().Start(context.Background(), "GET /api/items")
	defer request.End()

	rows, err := conn.QueryContext(ctx, "SELECT id, name FROM items")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	types, err := rows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}
	if types[0].DatabaseTypeName() != "INTEGER" || types[1].DatabaseTypeName() != "TEXT" {
		t.Fatalf("got %q %q", types[0].DatabaseTypeName(), types[1].DatabaseTypeName())
	}
}

func TestPreparedStatementsAreTracedAndFolded(t *testing.T) {
	conn, recorder := openTraced(t)
	ctx, request := traceway.Tracer().Start(context.Background(), "POST /api/otel/v1/traces")

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO items (name) VALUES (?)")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if _, err := stmt.ExecContext(ctx, "row"); err != nil {
			t.Fatal(err)
		}
	}
	stmt.Close()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	request.End()

	spans := dbSpans(recorder)
	if len(spans) != 1 || attr(spans[0], "db.operation.batch.size") != "20" {
		t.Fatalf("want one folded span of 20, got %d spans", len(spans))
	}
}

func TestFoldedSpansCountOnlyDriverTime(t *testing.T) {
	conn, recorder := openTraced(t)
	ctx, request := traceway.Tracer().Start(context.Background(), "POST /api/otel/v1/logs")

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := tx.Exec("INSERT INTO items (name) VALUES ('row')"); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	wall := time.Since(started)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	request.End()

	span := dbSpans(recorder)[0]
	if took := span.EndTime().Sub(span.StartTime()); took >= wall/2 {
		t.Fatalf("folded span took %v of %v wall time; the pauses between rows must not count", took, wall)
	}
}

func TestSummarize(t *testing.T) {
	cases := []struct{ query, operation, collection string }{
		{"select * from endpoints_v2 where id = ?", "SELECT", "endpoints_v2"},
		{"SELECT COUNT(*) AS count FROM (SELECT 1 FROM spans_v2) t", "SELECT", "spans_v2"},
		{"SELECT a FROM (SELECT * FROM (SELECT *, ROW_NUMBER() OVER (PARTITION BY id ORDER BY d) AS v FROM spans_v2 WHERE x = ?) ranked) t", "SELECT", "spans_v2"},
		{"SELECT n FROM (SELECT 1 AS n) t", "SELECT", ""},
		{"SELECT name FROM items WHERE id IN (SELECT id FROM other)", "SELECT", "items"},
		{"\n  WITH recent AS (SELECT * FROM logs) SELECT * FROM recent", "SELECT", "recent"},
		{"INSERT INTO \"spans_v2\" (a, b) VALUES (?, ?)", "INSERT", "spans_v2"},
		{"INSERT OR IGNORE INTO metric_registry VALUES (1)", "INSERT", "metric_registry"},
		{"UPDATE projects SET name = 'from x' WHERE id = $1", "UPDATE", "projects"},
		{"DELETE FROM log_records WHERE timestamp < ?", "DELETE", "log_records"},
		{"SELECT 'FROM fake' AS label", "SELECT", ""},
		{"OPTIMIZE TABLE traceway.spans_v2 FINAL", "OPTIMIZE", "traceway.spans_v2"},
		{"PRAGMA foreign_keys = ON", "PRAGMA", ""},
		{"-- comment\nSELECT 1 FROM `metric_points`", "SELECT", "metric_points"},
		{"", "QUERY", ""},
	}
	for _, tc := range cases {
		operation, collection := Summarize(tc.query)
		if operation != tc.operation || collection != tc.collection {
			t.Errorf("Summarize(%q) = %q %q, want %q %q", tc.query, operation, collection, tc.operation, tc.collection)
		}
	}
}
