//go:build telemetry_ch

package clickhouse

import (
	"context"

	"github.com/tracewayapp/traceway/backend/v2/app/chdb"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
)

// The charts of log attributes, span durations and span attributes all read a
// values subquery of (bucket, num, v) rows: num says whether the row's value is
// a number, v is that number. settings is appended to the query.

// chartNumber turns the text column raw into num and v.
const chartNumber = `match(raw, '` + shared.ChartNumberPattern + `') AND isFinite(toFloat64OrZero(raw)) AS num, if(num, toFloat64OrZero(raw), 0) AS v`

func queryChartStats(ctx context.Context, values string, args []any, settings string) ([]shared.ChartStatsRow, error) {
	query := `SELECT bucket, toInt64(count()) AS present, toInt64(countIf(num)) AS numeric,
		toInt64(countIf(num AND v = floor(v))) AS integral,
		sumIf(v, num) AS sum, minIf(v, num) AS min, maxIf(v, num) AS max,
		quantilesIf(0.5, 0.95, 0.99)(v, num) AS q
	FROM (` + values + `)
	GROUP BY bucket` + settings
	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []shared.ChartStatsRow
	for rows.Next() {
		var row shared.ChartStatsRow
		var q []float64
		if err := rows.Scan(&row.Bucket, &row.Present, &row.Numeric, &row.Integral, &row.Sum, &row.Min, &row.Max, &q); err != nil {
			return nil, err
		}
		if row.Numeric > 0 && len(q) == len(shared.ChartQuantiles) {
			row.P50, row.P95, row.P99 = q[0], q[1], q[2]
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func queryChartBins(ctx context.Context, values string, args []any, binning shared.ChartBinning, settings string) ([]shared.ChartBinRow, error) {
	bin, binArgs := binning.BinExpression("v", "greatest")
	query := `SELECT toInt64(greatest(0, least(toInt64(` + bin + `), ?))) AS bin, toInt64(count()) AS count
	FROM (` + values + `) WHERE num
	GROUP BY bin` + settings
	args = append(append(binArgs, int64(binning.Count-1)), args...)
	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []shared.ChartBinRow
	for rows.Next() {
		var index, count int64
		if err := rows.Scan(&index, &count); err != nil {
			return nil, err
		}
		out = append(out, shared.ChartBinRow{Index: int(index), Count: count})
	}
	return out, rows.Err()
}

// spanChartValues reads what a span chart measures: the duration in
// milliseconds, or the attribute's value (attribute non-empty).
func spanChartValues(search shared.OtelSpanSearch, attribute string, step int64) (string, []any) {
	bucket := "toInt64(0)"
	if step > 0 {
		bucket = epochBucket("recorded_at", int(step))
	}
	source, args := shared.OtelClickHouseChartSource(search, attribute, clickhouseSearchDialect)
	if attribute == "" {
		return "SELECT " + bucket + " AS bucket, toUInt8(1) AS num, duration / 1e6 AS v FROM " + source, args
	}
	return `SELECT bucket, ` + chartNumber + `
		FROM (SELECT ` + bucket + ` AS bucket, chart_raw AS raw FROM ` + source + `)`, args
}

func (r *otelSpanRepository) ChartStats(ctx context.Context, search shared.OtelSpanSearch, attribute string, step int64) ([]shared.ChartStatsRow, error) {
	values, args := spanChartValues(search, attribute, step)
	return queryChartStats(ctx, values, args, otelSearchSettings)
}

func (r *otelSpanRepository) ChartBins(ctx context.Context, search shared.OtelSpanSearch, attribute string, binning shared.ChartBinning) ([]shared.ChartBinRow, error) {
	values, args := spanChartValues(search, attribute, 0)
	return queryChartBins(ctx, values, args, binning, otelSearchSettings)
}
