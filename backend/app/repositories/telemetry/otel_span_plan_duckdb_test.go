//go:build telemetry_duckdb

package telemetry

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

func TestOtelSpanIndexedQueries(t *testing.T) {
	setupTestDB(t)
	project := uuid.New()
	spans := make([]models.OtelSpan, 10000)
	for i := range spans {
		spans[i] = canonicalSpan(project, uuid.New(), uuid.New(), nil)
	}
	start := time.Now()
	if _, err := OtelSpanRepository.InsertAsync(context.Background(), spans); err != nil {
		t.Fatal(err)
	}
	t.Logf("DuckDB canonical ingest: %d spans in %s", len(spans), time.Since(start))
	lookup := shared.SpanLookup{ProjectId: project, TraceId: spans[0].TraceId, SpanId: spans[0].SpanId, RecordedAt: &spans[0].RecordedAt}
	query, args := shared.OtelTopologyQuery([]shared.SpanLookup{lookup}, true, 0, func(t time.Time) any { return t.UTC() })
	rows, err := db.TelemetryDB.Query("EXPLAIN ANALYZE "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	var plan string
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			t.Fatal(err)
		}
		plan += value
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan, "Index Scan") {
		t.Fatalf("unindexed source lookup: %s", plan)
	}
	t.Log("source lookup uses Index Scan")

	start = time.Now()
	for range 100 {
		if _, err := findSpans(context.Background(), spans[0], &spans[0].RecordedAt); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("DuckDB canonical lookup: %s per detail (100 reads, 10000 stored spans)", time.Since(start)/100)
	for _, statement := range []string{"DROP INDEX idx_spans_v2_trace_key", "DELETE FROM spans_v2"} {
		if _, err := db.TelemetryDB.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	start = time.Now()
	if _, err := OtelSpanRepository.InsertAsync(context.Background(), spans); err != nil {
		t.Fatal(err)
	}
	t.Logf("canonical ingest without lookup indexes: %d spans in %s", len(spans), time.Since(start))

}
