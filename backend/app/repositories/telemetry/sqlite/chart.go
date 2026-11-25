//go:build !telemetry_ch && !telemetry_duckdb

package sqlite

import (
	"context"
	"fmt"
	"strings"

	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
)

// The charts of log attributes, span durations and span attributes all read a
// values subquery of (bucket, v) rows with positional placeholders, v being
// NULL when the row's value is not a number.

// chartNumber turns the text column raw into a number, or NULL when it is not a
// JSON number: the grammar the other backends match with
// shared.ChartNumberPattern. json_valid alone would also accept the whitespace
// JSON allows around a value.
const chartNumber = `CASE WHEN raw NOT GLOB '*[^0-9eE.+-]*' AND json_valid(raw) AND json_type(raw) IN ('integer', 'real')
		AND abs(CAST(raw AS REAL)) <= ` + shared.ChartNumberLimit + ` THEN CAST(raw AS REAL) END`

// queryChartStats summarises the values per bucket. SQLite has no quantile
// aggregate, so a second read takes the values at the sorted positions the
// percentiles interpolate between.
func queryChartStats(ctx context.Context, values string, args []any) ([]shared.ChartStatsRow, error) {
	rows, err := db.TelemetryDB.QueryContext(ctx, `SELECT bucket, COUNT(*) AS present, COUNT(v) AS numeric,
		COUNT(CASE WHEN v = CAST(v AS INTEGER) THEN 1 END) AS integral,
		COALESCE(SUM(v), 0) AS sum, COALESCE(MIN(v), 0) AS min, COALESCE(MAX(v), 0) AS max
	FROM (`+values+`)
	GROUP BY bucket`, args...)
	if err != nil {
		return nil, err
	}
	var out []shared.ChartStatsRow
	for rows.Next() {
		var row shared.ChartStatsRow
		if err := rows.Scan(&row.Bucket, &row.Present, &row.Numeric, &row.Integral, &row.Sum, &row.Min, &row.Max); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, row)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	positions := make([]string, 0, 2*len(shared.ChartQuantiles))
	for _, q := range shared.ChartQuantiles {
		low := fmt.Sprintf("CAST((n - 1) * %v AS INTEGER)", q)
		positions = append(positions, low, low+" + 1")
	}
	rows, err = db.TelemetryDB.QueryContext(ctx, `SELECT bucket, n, rn, v FROM (
		SELECT bucket, v, ROW_NUMBER() OVER (PARTITION BY bucket ORDER BY v) - 1 AS rn, COUNT(*) OVER (PARTITION BY bucket) AS n
		FROM (`+values+`) WHERE v IS NOT NULL
	) WHERE rn IN (`+strings.Join(positions, ", ")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var at []shared.ChartQuantileRow
	for rows.Next() {
		var v shared.ChartQuantileRow
		if err := rows.Scan(&v.Bucket, &v.N, &v.Position, &v.Value); err != nil {
			return nil, err
		}
		at = append(at, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	shared.ApplyChartQuantiles(out, at)
	return out, nil
}

func queryChartBins(ctx context.Context, values string, args []any, binning shared.ChartBinning) ([]shared.ChartBinRow, error) {
	bin, binArgs := binning.BinExpression("v", "max")
	rows, err := db.TelemetryDB.QueryContext(ctx, `SELECT max(0, min(CAST(`+bin+` AS INTEGER), ?)) AS bin, COUNT(*) AS count
	FROM (`+values+`) WHERE v IS NOT NULL
	GROUP BY bin`, append(append(binArgs, int64(binning.Count-1)), args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []shared.ChartBinRow
	for rows.Next() {
		var row shared.ChartBinRow
		if err := rows.Scan(&row.Index, &row.Count); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// spanChartValues reads what a span chart measures: the duration in
// milliseconds, or the attribute's value (attribute non-empty).
func spanChartValues(search shared.OtelSpanSearch, attribute string, step int64) (string, []any) {
	source, args := shared.OtelChartSource(search, attribute, sqliteSearchDialect)
	bucket := "0"
	if step > 0 {
		bucket = epochBucket("recorded_at", int(step))
	}
	if attribute == "" {
		return "SELECT " + bucket + " AS bucket, duration / 1e6 AS v FROM " + source, args
	}
	return `SELECT bucket, ` + chartNumber + ` AS v
		FROM (SELECT ` + bucket + ` AS bucket, ` + sqliteSpanAttribute + ` AS raw FROM ` + source + `)
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
