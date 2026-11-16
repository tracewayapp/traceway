//go:build telemetry_ch

package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/chdb"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
)

type metricHistogramRepository struct{}

const maxExemplarRows = 50

var histogramInsert = "INSERT INTO metric_histograms (project_id, name, recorded_at, " + strings.Join(shared.HistogramDimensionColumnOrder, ", ") + ", tags, cnt, total, min_val, max_val, zero_cnt, buckets)"

// InsertCells writes one row per cell. Rows of the same cell, from another flush or
// another instance, are merged by the table engine, and the hourly and daily rollups
// are filled by materialized views on this insert.
func (r *metricHistogramRepository) InsertCells(ctx context.Context, cells []shared.MetricHistogramCell) error {
	if len(cells) == 0 {
		return nil
	}
	return chdb.SendBatch(ctx, histogramInsert, func(batch driver.Batch) error {
		for _, cell := range cells {
			columns, tags := shared.SplitHistogramDims(cell.Dims)
			indexes, counts := cell.Hist.BucketArrays()
			values := make([]any, 0, len(columns)+10)
			values = append(values, cell.ProjectId, cell.Name, cell.RecordedAt.UTC())
			for _, column := range columns {
				values = append(values, column)
			}
			values = append(values, tags, cell.Hist.Count, cell.Hist.Sum, cell.Hist.Min, cell.Hist.Max, cell.Hist.Zero, []any{indexes, counts})
			if err := batch.Append(values...); err != nil {
				return err
			}
		}
		return nil
	})
}

// histogramTable picks the rollup by range, inside each table's TTL: minutes are
// kept 7 days, hours 90 days, days 2 years.
func histogramTable(rangeDuration time.Duration) string {
	switch {
	case rangeDuration <= 72*time.Hour:
		return "metric_histograms"
	case rangeDuration <= 60*24*time.Hour:
		return "metric_histograms_1h"
	default:
		return "metric_histograms_1d"
	}
}

func histogramDimExpr(key string, args *[]any) string {
	if column, ok := shared.HistogramDimensionColumns[key]; ok {
		return column
	}
	*args = append(*args, key)
	return "tags[?]"
}

func histogramFilters(filters map[string]string, args *[]any) string {
	var clause strings.Builder
	for _, key := range shared.SortedKeys(filters) {
		clause.WriteString(" AND " + histogramDimExpr(key, args) + " = ?")
		*args = append(*args, filters[key])
	}
	return clause.String()
}

func (r *metricHistogramRepository) Query(ctx context.Context, projectId uuid.UUID, name string, from, to time.Time, intervalMinutes int, tagFilters map[string]string, groupBy []string, maxGroups int) (map[string][]shared.HistogramSeriesPoint, bool, error) {
	if intervalMinutes < 1 {
		intervalMinutes = 1
	}
	var args []any
	groupExpr := "''"
	if len(groupBy) > 0 {
		parts := make([]string, len(groupBy))
		for i, key := range groupBy {
			parts[i] = histogramDimExpr(key, &args)
		}
		groupExpr = strings.Join(parts, " || '"+shared.GroupKeySeparator+"' || ")
	}
	query := "SELECT " + epochBucket("recorded_at", intervalMinutes*60) + " AS bucket, " + groupExpr + " AS group_key, " +
		"sum(cnt), sum(total), min(min_val), max(max_val), sum(zero_cnt), (sumMap(buckets.1, buckets.2) AS merged).1, merged.2 " +
		"FROM " + histogramTable(to.Sub(from)) + " WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?"
	args = append(args, projectId, name, from.UTC(), to.UTC())
	query += histogramFilters(tagFilters, &args) + " GROUP BY bucket, group_key ORDER BY group_key ASC, bucket ASC"
	if len(groupBy) > 0 && maxGroups > 0 {
		query += fmt.Sprintf(" LIMIT %d", shared.TimeSeriesRowLimit(maxGroups))
	}

	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	series := shared.NewHistogramSeries(intervalMinutes, groupBy, maxGroups)
	for rows.Next() {
		var bucket int64
		var groupKey string
		var indexes []int32
		var counts []float64
		h := shared.NewHistogram()
		if err := rows.Scan(&bucket, &groupKey, &h.Count, &h.Sum, &h.Min, &h.Max, &h.Zero, &indexes, &counts); err != nil {
			return nil, false, err
		}
		for i, index := range indexes {
			if i < len(counts) {
				h.Buckets[index] = counts[i]
			}
		}
		switch {
		case len(groupBy) == 0:
			groupKey = "__all__"
		case groupKey == "":
			groupKey = "(empty)"
		}
		series.AddKeyed(time.Unix(bucket, 0).UTC(), groupKey, h)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return series.Result(), series.Truncated, nil
}

func (r *metricHistogramRepository) Discover(ctx context.Context, projectId uuid.UUID, from, to time.Time) ([]models.DiscoveredMetric, error) {
	flags := make([]string, len(shared.HistogramDimensionColumnOrder))
	for i, column := range shared.HistogramDimensionColumnOrder {
		flags[i] = "max(" + column + " != '')"
	}
	rows, err := chdb.Conn.Query(ctx,
		"SELECT name, groupUniqArrayArray(mapKeys(tags)), "+strings.Join(flags, ", ")+
			" FROM metric_histograms_1h WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ? GROUP BY name",
		projectId, from.UTC().Truncate(time.Hour), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columnKeys := make(map[string]string, len(shared.HistogramDimensionColumns))
	for key, column := range shared.HistogramDimensionColumns {
		columnKeys[column] = key
	}
	keys := make(map[string]map[string]struct{})
	for rows.Next() {
		var name string
		var tagKeys []string
		present := make([]uint8, len(flags))
		targets := []any{&name, &tagKeys}
		for i := range present {
			targets = append(targets, &present[i])
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		keys[name] = make(map[string]struct{})
		for _, key := range tagKeys {
			keys[name][key] = struct{}{}
		}
		for i, column := range shared.HistogramDimensionColumnOrder {
			if present[i] != 0 {
				keys[name][columnKeys[column]] = struct{}{}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return shared.DiscoveredHistogramMetrics(keys), nil
}

func (r *metricHistogramRepository) DiscoverTagValues(ctx context.Context, projectId uuid.UUID, name, key string, from, to time.Time) ([]string, error) {
	var args []any
	expr := histogramDimExpr(key, &args)
	args = append(args, projectId, name, from.UTC().Truncate(time.Hour), to.UTC())
	rows, err := chdb.Conn.Query(ctx,
		"SELECT DISTINCT "+expr+" AS v FROM metric_histograms_1h WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ? AND v != '' ORDER BY v ASC LIMIT 1000", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r *metricHistogramRepository) InsertExemplars(ctx context.Context, exemplars []shared.MetricExemplar) error {
	if len(exemplars) == 0 {
		return nil
	}
	return chdb.SendBatch(ctx, "INSERT INTO metric_exemplars (project_id, name, recorded_at, dims, value, session_id, target)", func(batch driver.Batch) error {
		for _, e := range exemplars {
			dims := e.Dims
			if dims == nil {
				dims = map[string]string{}
			}
			if err := batch.Append(e.ProjectId, e.Name, e.RecordedAt.UTC(), dims, e.Value, e.SessionId, e.Target); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *metricHistogramRepository) FindExemplars(ctx context.Context, projectId uuid.UUID, name string, from, to time.Time, tagFilters map[string]string, limit int) ([]shared.MetricExemplar, error) {
	if limit <= 0 || limit > maxExemplarRows {
		limit = maxExemplarRows
	}
	args := []any{projectId, name, from.UTC(), to.UTC()}
	var filter strings.Builder
	for _, key := range shared.SortedKeys(tagFilters) {
		filter.WriteString(" AND dims[?] = ?")
		args = append(args, key, tagFilters[key])
	}
	rows, err := chdb.Conn.Query(ctx,
		"SELECT recorded_at, dims, value, session_id, target FROM metric_exemplars WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?"+
			filter.String()+fmt.Sprintf(" ORDER BY value DESC LIMIT %d", limit), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	exemplars := []shared.MetricExemplar{}
	for rows.Next() {
		e := shared.MetricExemplar{ProjectId: projectId, Name: name}
		if err := rows.Scan(&e.RecordedAt, &e.Dims, &e.Value, &e.SessionId, &e.Target); err != nil {
			return nil, err
		}
		exemplars = append(exemplars, e)
	}
	return exemplars, rows.Err()
}

// HotSeries returns the series of every metric that has at least minSeries of them
// since from, at most limit series each, from the hourly rollup: it holds what every
// instance wrote, and a day of it is a small read.
func (r *metricHistogramRepository) HotSeries(ctx context.Context, from time.Time, minSeries, limit int) ([]shared.MetricSeriesSet, error) {
	dimensions := strings.Join(shared.HistogramDimensionColumnOrder, ", ")
	from = from.UTC().Truncate(time.Hour)
	rows, err := chdb.Conn.Query(ctx,
		"SELECT project_id, name FROM metric_histograms_1h WHERE recorded_at >= ? GROUP BY project_id, name HAVING uniq("+dimensions+", tags) >= ?"+
			fmt.Sprintf(" LIMIT %d", shared.MaxHotSeriesMetrics), from, uint64(minSeries))
	if err != nil {
		return nil, err
	}
	var sets []shared.MetricSeriesSet
	for rows.Next() {
		var set shared.MetricSeriesSet
		if err := rows.Scan(&set.ProjectId, &set.Name); err != nil {
			rows.Close()
			return nil, err
		}
		sets = append(sets, set)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range sets {
		series, err := chdb.Conn.Query(ctx,
			"SELECT DISTINCT "+dimensions+", tags FROM metric_histograms_1h WHERE project_id = ? AND name = ? AND recorded_at >= ?"+fmt.Sprintf(" LIMIT %d", limit),
			sets[i].ProjectId, sets[i].Name, from)
		if err != nil {
			return nil, err
		}
		for series.Next() {
			columns := make([]string, len(shared.HistogramDimensionColumnOrder))
			var tags map[string]string
			dest := make([]any, 0, len(columns)+1)
			for c := range columns {
				dest = append(dest, &columns[c])
			}
			if err := series.Scan(append(dest, &tags)...); err != nil {
				series.Close()
				return nil, err
			}
			sets[i].Dims = append(sets[i].Dims, shared.JoinHistogramDims(columns, tags))
		}
		series.Close()
		if err := series.Err(); err != nil {
			return nil, err
		}
	}
	return sets, nil
}

// Compact does nothing here: the table engine merges rows of a cell, and the rollup
// tables are what long ranges read.
func (r *metricHistogramRepository) Compact(ctx context.Context, cutoff time.Time) (int, error) {
	return 0, nil
}

var MetricHistogramRepository = &metricHistogramRepository{}
