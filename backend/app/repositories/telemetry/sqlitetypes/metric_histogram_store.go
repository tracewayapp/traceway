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
	// compactBatchRows bounds the memory of one compaction step.
	compactBatchRows = 20000
	maxExemplarRows  = 50
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
		"SELECT "+histogramColumns+" FROM metric_histograms WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?"+filter, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	series := shared.NewHistogramSeries(intervalMinutes, groupBy, maxGroups)
	for rows.Next() {
		var recordedAt SQLiteTime
		var dims, buckets string
		h := &shared.Histogram{}
		if err := rows.Scan(scanHistogram(&recordedAt, &dims, h, &buckets)...); err != nil {
			return nil, false, err
		}
		if h.Buckets, err = shared.DecodeBuckets(buckets); err != nil {
			return nil, false, fmt.Errorf("metric_histograms buckets: %w", err)
		}
		series.Add(recordedAt.Time, decodeDims(dims), h)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return series.Result(), series.Truncated, nil
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
		"SELECT project_id, name FROM metric_histograms WHERE recorded_at >= ? GROUP BY project_id, name HAVING COUNT(DISTINCT dims) >= ? LIMIT ?",
		s.TimeArg(from), minSeries, shared.MaxHotSeriesMetrics)
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

type compactKey struct {
	projectId, name, dims string
	hour                  int64
}

// Compact folds minute rows older than cutoff into one row per cell per hour, so a
// read over weeks merges thousands of rows and not millions. It works through whole
// hours, oldest first, and returns the number of minute rows it replaced.
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
		"SELECT project_id, name, "+histogramColumns+" FROM metric_histograms WHERE grain_seconds = ? AND recorded_at >= ? AND recorded_at < ? LIMIT ?",
		histogramGrainMinute, s.TimeArg(from), s.TimeArg(to), compactBatchRows)
	if err != nil {
		return 0, err
	}
	merged := make(map[compactKey]*shared.Histogram)
	count := 0
	for rows.Next() {
		var projectId, name, dims, buckets string
		var recordedAt SQLiteTime
		h := &shared.Histogram{}
		if err := rows.Scan(append([]any{&projectId, &name}, scanHistogram(&recordedAt, &dims, h, &buckets)...)...); err != nil {
			rows.Close()
			return 0, err
		}
		if h.Buckets, err = shared.DecodeBuckets(buckets); err != nil {
			rows.Close()
			return 0, err
		}
		key := compactKey{projectId, name, dims, from.Unix()}
		if merged[key] == nil {
			merged[key] = shared.NewHistogram()
		}
		merged[key].Merge(h)
		count++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	// A partial hour would be merged twice on the next step.
	if count >= compactBatchRows {
		return 0, fmt.Errorf("metric_histograms: hour %s holds more than %d minute rows, not compacted", from.Format(time.RFC3339), compactBatchRows)
	}

	keys := make([]compactKey, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].projectId+keys[i].name+keys[i].dims < keys[j].projectId+keys[j].name+keys[j].dims
	})
	cells := make([]shared.MetricHistogramCell, 0, len(keys))
	for _, key := range keys {
		projectId, err := uuid.Parse(key.projectId)
		if err != nil {
			continue
		}
		cells = append(cells, shared.MetricHistogramCell{ProjectId: projectId, Name: key.name, RecordedAt: from, Dims: decodeDims(key.dims), Hist: merged[key]})
	}

	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM metric_histograms WHERE grain_seconds = ? AND recorded_at >= ? AND recorded_at < ?", histogramGrainMinute, s.TimeArg(from), s.TimeArg(to)); err != nil {
		return 0, err
	}
	if err := s.insertCells(ctx, tx, cells, histogramGrainHour); err != nil {
		return 0, err
	}
	return count, tx.Commit()
}
