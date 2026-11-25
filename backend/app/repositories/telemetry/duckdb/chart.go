//go:build telemetry_duckdb

package duckdb

import (
	"context"

	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
)

// The charts of log attributes, span durations and span attributes all read a
// values subquery of (bucket, v) rows with positional placeholders, v being
// NULL when the row's value is not a number.

// chartNumber turns the text column raw into a number, or NULL when it is not one.
const chartNumber = `CASE WHEN regexp_full_match(raw, '` + shared.ChartNumberPattern + `')
		AND abs(TRY_CAST(raw AS DOUBLE)) <= ` + shared.ChartNumberLimit + ` THEN TRY_CAST(raw AS DOUBLE) END`

func queryChartStats(ctx context.Context, values string, args []any) ([]shared.ChartStatsRow, error) {
	rows, err := db.TelemetryDB.QueryContext(ctx, `SELECT bucket, CAST(COUNT(*) AS BIGINT) AS present, CAST(COUNT(v) AS BIGINT) AS numeric,
		CAST(COUNT(CASE WHEN v = floor(v) THEN 1 END) AS BIGINT) AS integral,
		COALESCE(SUM(v), 0) AS sum, COALESCE(MIN(v), 0) AS min, COALESCE(MAX(v), 0) AS max,
		COALESCE(quantile_cont(v, 0.5), 0) AS p50, COALESCE(quantile_cont(v, 0.95), 0) AS p95, COALESCE(quantile_cont(v, 0.99), 0) AS p99
	FROM (`+values+`) AS chart
	GROUP BY bucket`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []shared.ChartStatsRow
	for rows.Next() {
		var row shared.ChartStatsRow
		if err := rows.Scan(&row.Bucket, &row.Present, &row.Numeric, &row.Integral, &row.Sum, &row.Min, &row.Max, &row.P50, &row.P95, &row.P99); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func queryChartBins(ctx context.Context, values string, args []any, binning shared.ChartBinning) ([]shared.ChartBinRow, error) {
	bin, binArgs := binning.BinExpression("v", "greatest")
	rows, err := db.TelemetryDB.QueryContext(ctx, `SELECT CAST(greatest(0, least(CAST(`+bin+` AS BIGINT), ?)) AS BIGINT) AS bin,
		CAST(COUNT(*) AS BIGINT) AS count
	FROM (`+values+`) AS chart WHERE v IS NOT NULL
	GROUP BY bin`, append(append(binArgs, int64(binning.Count-1)), args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []shared.ChartBinRow
	for rows.Next() {
		var row shared.ChartBinRow
		var index int64
		if err := rows.Scan(&index, &row.Count); err != nil {
			return nil, err
		}
		row.Index = int(index)
		out = append(out, row)
	}
	return out, rows.Err()
}

// spanChartValues reads what a span chart measures: the duration in
// milliseconds, or the attribute's value (attribute non-empty).
func spanChartValues(search shared.OtelSpanSearch, attribute string, step int64) (string, []any) {
	source, args := shared.OtelChartSource(search, attribute, duckdbSearchDialect)
	bucket := "CAST(0 AS BIGINT)"
	if step > 0 {
		bucket = epochBucket("recorded_at", int(step))
	}
	if attribute == "" {
		return "SELECT " + bucket + " AS bucket, duration / 1e6 AS v FROM " + source, args
	}
	return `SELECT bucket, ` + chartNumber + ` AS v
		FROM (SELECT ` + bucket + ` AS bucket, ` + duckdbSpanAttribute + ` AS raw FROM ` + source + `) AS attr
		WHERE raw IS NOT NULL`, append([]any{attribute}, args...)
}

func (r *otelSpanRepository) ChartStats(ctx context.Context, search shared.OtelSpanSearch, attribute string, step int64) ([]shared.ChartStatsRow, error) {
	values, args := spanChartValues(search, attribute, step)
	return queryChartStats(ctx, values, args)
}

func (r *otelSpanRepository) ChartBins(ctx context.Context, search shared.OtelSpanSearch, attribute string, binning shared.ChartBinning) ([]shared.ChartBinRow, error) {
	values, args := spanChartValues(search, attribute, 0)
	return queryChartBins(ctx, values, args, binning)
}
