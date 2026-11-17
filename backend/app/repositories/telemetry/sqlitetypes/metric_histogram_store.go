package sqlitetypes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
)

// HistogramStore is the metric_histograms repository of the two embedded telemetry
// backends. They store one append-only row per cell per flush, with the dimensions and
// the buckets as JSON, and merge in Go when reading: both builds are single-instance,
// so the in-process aggregator already wrote one row per cell per flush.
type HistogramStore struct {
	// TimeArg renders a time the way the backend compares its timestamp column.
	TimeArg func(time.Time) any
	// JSONExtract names the function that reads a JSON string value as text.
	JSONExtract func() string
	// InsertCells overrides the row-by-row insert (DuckDB appends).
	InsertCellsFunc func(ctx context.Context, cells []shared.MetricHistogramCell) error
}

const (
	histogramGrainMinute = 60
	histogramGrainHour   = 3600
	maxExemplarRows      = 50
)

const histogramInsert = "INSERT INTO metric_histograms (project_id, name, recorded_at, grain_seconds, dims, cnt, total, min_val, max_val, zero_cnt, buckets, weight_squares) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"

func encodeDims(dims map[string]string) string {
	if len(dims) == 0 {
		return "{}"
	}
	b, _ := json.Marshal(dims)
	return string(b)
}

func decodeDims(raw string) map[string]string {
	dims := make(map[string]string)
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &dims)
	}
	return dims
}

// HistogramRowValues is one stored row, in column order after project_id.
func HistogramRowValues(cell shared.MetricHistogramCell, grainSeconds int) (string, string, time.Time, int, string, float64, float64, float64, float64, float64, string, float64) {
	h := cell.Hist
	return cell.ProjectId.String(), cell.Name, cell.RecordedAt.UTC(), grainSeconds, encodeDims(cell.Dims), h.Count, h.Sum, h.Min, h.Max, h.Zero, h.EncodeBuckets(), h.WeightSquareSum()
}

func (s *HistogramStore) InsertCells(ctx context.Context, cells []shared.MetricHistogramCell) error {
	if len(cells) == 0 {
		return nil
	}
	if s.InsertCellsFunc != nil {
		return s.InsertCellsFunc(ctx, cells)
	}
	return s.InsertCellsRows(ctx, cells)
}

// InsertCellsRows inserts row by row in one transaction.
func (s *HistogramStore) InsertCellsRows(ctx context.Context, cells []shared.MetricHistogramCell) error {
	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.insertCells(ctx, tx, cells, histogramGrainMinute); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *HistogramStore) insertCells(ctx context.Context, tx *sql.Tx, cells []shared.MetricHistogramCell, grainSeconds int) error {
	stmt, err := tx.PrepareContext(ctx, histogramInsert)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, cell := range cells {
		projectId, name, recordedAt, grain, dims, cnt, total, minVal, maxVal, zero, buckets, weightSquares := HistogramRowValues(cell, grainSeconds)
		if _, err := stmt.ExecContext(ctx, projectId, name, s.TimeArg(recordedAt), grain, dims, cnt, total, minVal, maxVal, zero, buckets, weightSquares); err != nil {
			return err
		}
	}
	return nil
}

func (s *HistogramStore) dimExpr() string {
	return s.JSONExtract() + "(dims, '$.\"' || ? || '\"')"
}

func (s *HistogramStore) filterClause(filters map[string]string) (string, []any) {
	var clause strings.Builder
	var args []any
	for _, key := range shared.SortedKeys(filters) {
		if filters[key] == "" {
			clause.WriteString(" AND COALESCE(" + s.dimExpr() + ", '') = ''")
			args = append(args, key)
			continue
		}
		clause.WriteString(" AND " + s.dimExpr() + " = ?")
		args = append(args, key, filters[key])
	}
	return clause.String(), args
}

func scanHistogram(recordedAt *SQLiteTime, dims *string, h *shared.Histogram, buckets *string) []any {
	return []any{recordedAt, dims, &h.Count, &h.Sum, &h.Min, &h.Max, &h.Zero, buckets, &h.WeightSquares}
}

const histogramColumns = "recorded_at, dims, cnt, total, min_val, max_val, zero_cnt, buckets, CASE WHEN weight_squares > 0 THEN weight_squares ELSE cnt * 1000 END"

// Query returns one merged histogram per group per time bucket, keyed like the metric
// point queries. The boolean reports that more groups existed than maxGroups.
func (s *HistogramStore) Query(ctx context.Context, projectId uuid.UUID, name string, from, to time.Time, intervalMinutes int, tagFilters map[string]string, groupBy []string, maxGroups int) (map[string][]shared.HistogramSeriesPoint, bool, error) {
	filter, filterArgs := s.filterClause(tagFilters)
	args := append([]any{projectId.String(), name, s.TimeArg(from), s.TimeArg(to)}, filterArgs...)
	rows, err := db.TelemetryDB.QueryContext(ctx,
		"SELECT grain_seconds, "+histogramColumns+" FROM metric_histograms WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?"+filter, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	series := shared.NewHistogramSeries(intervalMinutes, groupBy, maxGroups)
	for rows.Next() {
		var recordedAt SQLiteTime
		var dims, buckets string
		var grain int
		h := &shared.Histogram{}
		if err := rows.Scan(append([]any{&grain}, scanHistogram(&recordedAt, &dims, h, &buckets)...)...); err != nil {
			return nil, false, err
		}
		if h.Buckets, err = shared.DecodeBuckets(buckets); err != nil {
			return nil, false, fmt.Errorf("metric_histograms buckets: %w", err)
		}
		series.AddGrain(recordedAt.Time, shared.HistogramGroupKey(decodeDims(dims), groupBy), h, grain)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	result := series.Result()
	return result, series.Truncated, nil
}

// Discover lists the histogram metrics of a project with the dimension keys they carry.
func (s *HistogramStore) Discover(ctx context.Context, projectId uuid.UUID, from, to time.Time) ([]models.DiscoveredMetric, error) {
	rows, err := db.TelemetryDB.QueryContext(ctx,
		"SELECT DISTINCT name, dims FROM metric_histograms WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?",
		projectId.String(), s.TimeArg(from), s.TimeArg(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := make(map[string]map[string]struct{})
	for rows.Next() {
		var name, dims string
		if err := rows.Scan(&name, &dims); err != nil {
			return nil, err
		}
		if keys[name] == nil {
			keys[name] = make(map[string]struct{})
		}
		for key := range decodeDims(dims) {
			keys[name][key] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return shared.DiscoveredHistogramMetrics(keys), nil
}

func (s *HistogramStore) DiscoverTagValues(ctx context.Context, projectId uuid.UUID, name, key string, from, to time.Time) ([]string, error) {
	rows, err := db.TelemetryDB.QueryContext(ctx,
		"SELECT DISTINCT "+s.dimExpr()+" AS v FROM metric_histograms WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ? ORDER BY v ASC",
		key, projectId.String(), name, s.TimeArg(from), s.TimeArg(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []string{}
	for rows.Next() {
		var value sql.NullString
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		if value.Valid && value.String != "" {
			values = append(values, value.String)
		}
	}
	return values, rows.Err()
}

func (s *HistogramStore) InsertExemplars(ctx context.Context, exemplars []shared.MetricExemplar) error {
	if len(exemplars) == 0 {
		return nil
	}
	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO metric_exemplars (project_id, name, recorded_at, dims, value, session_id, target) VALUES (?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, e := range exemplars {
		if _, err := stmt.ExecContext(ctx, e.ProjectId.String(), e.Name, s.TimeArg(e.RecordedAt.UTC()), encodeDims(e.Dims), e.Value, e.SessionId, e.Target); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// FindExemplars returns the worst values in the range, largest first.
func (s *HistogramStore) FindExemplars(ctx context.Context, projectId uuid.UUID, name string, from, to time.Time, tagFilters map[string]string, limit int) ([]shared.MetricExemplar, error) {
	if limit <= 0 || limit > maxExemplarRows {
		limit = maxExemplarRows
	}
	filter, filterArgs := s.filterClause(tagFilters)
	args := append([]any{projectId.String(), name, s.TimeArg(from), s.TimeArg(to)}, filterArgs...)
	args = append(args, limit)
	rows, err := db.TelemetryDB.QueryContext(ctx,
		"SELECT recorded_at, dims, value, session_id, COALESCE(target, '') FROM metric_exemplars WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?"+filter+" ORDER BY value DESC LIMIT ?", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	exemplars := []shared.MetricExemplar{}
	for rows.Next() {
		var recordedAt SQLiteTime
		var dims string
		e := shared.MetricExemplar{ProjectId: projectId, Name: name}
		if err := rows.Scan(&recordedAt, &dims, &e.Value, &e.SessionId, &e.Target); err != nil {
			return nil, err
		}
		e.RecordedAt, e.Dims = recordedAt.Time.UTC(), decodeDims(dims)
		exemplars = append(exemplars, e)
	}
	return exemplars, rows.Err()
}

// HotSeries returns the series of every metric that has at least minSeries of them
// since from, at most limit series each. The client metrics guard reads it so that a
// series budget counts what is stored, not what one process has seen.
func (s *HistogramStore) HotSeries(ctx context.Context, from time.Time, minSeries, limit int) ([]shared.MetricSeriesSet, error) {
	rows, err := db.TelemetryDB.QueryContext(ctx,
		// The grain predicate changes nothing in the result; it lets SQLite use the (grain_seconds, recorded_at)
		// index for a query that runs every minute, instead of scanning the table.
		"SELECT project_id, name FROM metric_histograms WHERE grain_seconds IN (?, ?) AND recorded_at >= ? GROUP BY project_id, name HAVING COUNT(DISTINCT dims) >= ? LIMIT ?",
		histogramGrainMinute, histogramGrainHour, s.TimeArg(from), minSeries, shared.MaxHotSeriesMetrics)
	if err != nil {
		return nil, err
	}
	var sets []shared.MetricSeriesSet
	for rows.Next() {
		var projectId string
		var set shared.MetricSeriesSet
		if err := rows.Scan(&projectId, &set.Name); err != nil {
			rows.Close()
			return nil, err
		}
		if set.ProjectId, err = uuid.Parse(projectId); err == nil {
			sets = append(sets, set)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range sets {
		dims, err := db.TelemetryDB.QueryContext(ctx,
			"SELECT DISTINCT dims FROM metric_histograms WHERE project_id = ? AND name = ? AND recorded_at >= ? LIMIT ?",
			sets[i].ProjectId.String(), sets[i].Name, s.TimeArg(from), limit)
		if err != nil {
			return nil, err
		}
		for dims.Next() {
			var raw string
			if err := dims.Scan(&raw); err != nil {
				dims.Close()
				return nil, err
			}
			sets[i].Dims = append(sets[i].Dims, decodeDims(raw))
		}
		dims.Close()
		if err := dims.Err(); err != nil {
			return nil, err
		}
	}
	return sets, nil
}

// compactSlice is how much of an hour one DELETE ... RETURNING takes: the engine holds
// the returned rows in memory until the statement ends.
const compactSlice = 5 * time.Minute

// Compact folds minute rows older than cutoff into one row per cell per hour, so a
// read over weeks merges thousands of rows and not millions. It works through whole
// hours, oldest first, one metric of one project at a time, and returns the number
// of minute rows it replaced.
func (s *HistogramStore) Compact(ctx context.Context, cutoff time.Time) (int, error) {
	cutoff = cutoff.UTC().Truncate(time.Hour)
	replaced := 0
	for ctx.Err() == nil {
		n, err := s.compactStep(ctx, cutoff)
		if err != nil {
			return replaced, err
		}
		if n == 0 {
			return replaced, nil
		}
		replaced += n
	}
	return replaced, ctx.Err()
}

type compactUnit struct {
	projectId, name string
}

// compactStep compacts the oldest hour that still has minute rows.
func (s *HistogramStore) compactStep(ctx context.Context, cutoff time.Time) (int, error) {
	var oldest SQLiteTime
	err := db.TelemetryDB.QueryRowContext(ctx,
		"SELECT MIN(recorded_at) FROM metric_histograms WHERE grain_seconds = ? AND recorded_at < ?", histogramGrainMinute, s.TimeArg(cutoff)).Scan(&oldest)
	if err != nil || oldest.Time.IsZero() {
		return 0, err
	}
	from := oldest.Time.UTC().Truncate(time.Hour)
	to := from.Add(time.Hour)

	rows, err := db.TelemetryDB.QueryContext(ctx,
		"SELECT DISTINCT project_id, name FROM metric_histograms WHERE grain_seconds = ? AND recorded_at >= ? AND recorded_at < ?",
		histogramGrainMinute, s.TimeArg(from), s.TimeArg(to))
	if err != nil {
		return 0, err
	}
	var units []compactUnit
	for rows.Next() {
		var unit compactUnit
		if err := rows.Scan(&unit.projectId, &unit.name); err != nil {
			rows.Close()
			return 0, err
		}
		units = append(units, unit)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	replaced := 0
	for _, unit := range units {
		if ctx.Err() != nil {
			return replaced, ctx.Err()
		}
		n, err := s.compactUnit(ctx, unit, from, to)
		if err != nil {
			return replaced, fmt.Errorf("metric_histograms: compacting %s of project %s at %s: %w", unit.name, unit.projectId, from.Format(time.RFC3339), err)
		}
		replaced += n
	}
	return replaced, nil
}

// compactUnit replaces the minute rows of one metric in one hour with hour rows, in
// one transaction. The rows it merges are the rows it deleted, so a row that arrives
// late, while this runs, is neither lost nor counted twice: it stays a minute row
// for the next pass.
func (s *HistogramStore) compactUnit(ctx context.Context, unit compactUnit, from, to time.Time) (int, error) {
	projectId, err := uuid.Parse(unit.projectId)
	if err != nil {
		return 0, nil
	}
	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	merged := make(map[string]*shared.Histogram)
	count := 0
	for start := from; start.Before(to); start = start.Add(compactSlice) {
		rows, err := tx.QueryContext(ctx,
			"DELETE FROM metric_histograms WHERE project_id = ? AND name = ? AND grain_seconds = ? AND recorded_at >= ? AND recorded_at < ? RETURNING "+histogramColumns,
			unit.projectId, unit.name, histogramGrainMinute, s.TimeArg(start), s.TimeArg(start.Add(compactSlice)))
		if err != nil {
			return 0, err
		}
		for rows.Next() {
			var dims, buckets string
			var recordedAt SQLiteTime
			h := &shared.Histogram{}
			if err := rows.Scan(scanHistogram(&recordedAt, &dims, h, &buckets)...); err != nil {
				rows.Close()
				return 0, err
			}
			if h.Buckets, err = shared.DecodeBuckets(buckets); err != nil {
				rows.Close()
				return 0, err
			}
			if merged[dims] == nil {
				merged[dims] = shared.NewHistogram()
			}
			merged[dims].Merge(h)
			count++
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return 0, err
		}
	}
	if count == 0 {
		return 0, nil
	}

	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	cells := make([]shared.MetricHistogramCell, 0, len(keys))
	for _, key := range keys {
		cells = append(cells, shared.MetricHistogramCell{ProjectId: projectId, Name: unit.name, RecordedAt: from, Dims: decodeDims(key), Hist: merged[key]})
	}
	if err := s.insertCells(ctx, tx, cells, histogramGrainHour); err != nil {
		return 0, err
	}
	return count, tx.Commit()
}
