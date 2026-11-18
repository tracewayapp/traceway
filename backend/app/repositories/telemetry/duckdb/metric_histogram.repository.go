//go:build telemetry_duckdb

package duckdb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/sqlitetypes"
)

// The dialect is decided when a query runs: tests of other packages open a SQLite
// telemetry database under this build tag. The driver of the open database is what
// tells them apart; the flag in the db package is only set by the server's own open.
func telemetryIsDuckDB() bool {
	if db.IsDuckDBTelemetry() {
		return true
	}
	if db.TelemetryDB == nil {
		return false
	}
	return strings.Contains(strings.ToLower(fmt.Sprintf("%T", db.TelemetryDB.Driver())), "duckdb")
}

func appendHistogramCells(ctx context.Context, cells []shared.MetricHistogramCell) error {
	if !telemetryIsDuckDB() || db.DuckDBConnector == nil {
		return MetricHistogramRepository.InsertCellsRows(ctx, cells)
	}
	return withAppender(ctx, "metric_histograms", func(appender *duckdb.Appender) {
		for _, cell := range cells {
			projectId, name, recordedAt, grain, dims, cnt, total, minVal, maxVal, zero, buckets, weightSquares := sqlitetypes.HistogramRowValues(cell, 60)
			if err := appender.AppendRow(projectId, name, recordedAt, int64(grain), dims, cnt, total, minVal, maxVal, zero, buckets, weightSquares); err != nil {
				captureDroppedRow("metric_histograms", err)
			}
		}
	})
}

var MetricHistogramRepository *sqlitetypes.HistogramStore

func init() {
	MetricHistogramRepository = &sqlitetypes.HistogramStore{
		TimeArg: func(t time.Time) any {
			if !telemetryIsDuckDB() {
				return sqlitetypes.NewSQLiteTime(t)
			}
			return t.UTC()
		},
		JSONExtract: func() string {
			if !telemetryIsDuckDB() {
				return "json_extract"
			}
			return "json_extract_string"
		},
		InsertCellsFunc: appendHistogramCells,
	}
}
