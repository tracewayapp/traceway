//go:build telemetry_duckdb

package duckdb

import (
	"context"
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type checkResultRepository struct{}

func (r *checkResultRepository) Insert(ctx context.Context, result shared.CheckResult) error {
	var rowErr error
	err := withAppender(ctx, "check_results", func(appender *duckdb.Appender) {
		// Column order matches the check_results DDL in duckdb_telemetry/0003
		// plus 0004 (ALTER ADD appends output_key AFTER tls_days_remaining).
		rowErr = appender.AppendRow(
			result.ProjectId.String(),
			int64(result.CheckId),
			int64(result.RunId),
			result.CheckType,
			result.RecordedAt.UTC(),
			result.ExecutedAt.UTC(),
			result.Status,
			result.LatencyMs,
			int64(result.StatusCode),
			result.ErrorMsg,
			result.ExecutedBy,
			result.ScreenshotKey,
			int64(result.TlsDaysRemaining),
			result.OutputKey,
		)
	})
	if rowErr != nil {
		return rowErr
	}
	return err
}

func (r *checkResultRepository) FindByCheck(ctx context.Context, projectId uuid.UUID, checkId int, from *time.Time, to *time.Time, status string, page int, pageSize int) ([]*models.CheckResultEntry, int64, error) {
	where := "WHERE project_id = ? AND check_id = ?"
	args := []interface{}{projectId.String(), checkId}

	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}

	if from != nil {
		where += " AND recorded_at >= ?"
		args = append(args, from.UTC())
	}
	if to != nil {
		where += " AND recorded_at <= ?"
		args = append(args, to.UTC())
	}

	var total int64
	if err := db.TelemetryDB.QueryRowContext(ctx, "SELECT CAST(COUNT(*) AS BIGINT) FROM check_results "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT CAST(run_id AS BIGINT), check_type, recorded_at, executed_at, status, latency_ms, CAST(status_code AS BIGINT), error_message, executed_by, screenshot_key, output_key, CAST(tls_days_remaining AS BIGINT) FROM check_results " +
		where + " ORDER BY recorded_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []*models.CheckResultEntry
	for rows.Next() {
		entry := &models.CheckResultEntry{}
		var recordedAt, executedAt sqlitetypes.SQLiteTime
		if err := rows.Scan(
			&entry.RunId, &entry.CheckType, &recordedAt, &executedAt, &entry.Status,
			&entry.LatencyMs, &entry.StatusCode, &entry.ErrorMessage, &entry.ExecutedBy,
			&entry.ScreenshotKey, &entry.OutputKey, &entry.TlsDaysRemaining,
		); err != nil {
			return nil, 0, err
		}
		entry.RecordedAt = recordedAt.Time
		entry.ExecutedAt = executedAt.Time
		items = append(items, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// AggregatesByProject returns per-check result summaries for the range.
func (r *checkResultRepository) AggregatesByProject(ctx context.Context, projectId uuid.UUID, from time.Time, to time.Time) (map[int]*models.CheckAggregates, error) {
	query := `SELECT CAST(check_id AS BIGINT),
		CAST(COUNT(*) AS BIGINT),
		CAST(SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END) AS BIGINT),
		CAST(SUM(CASE WHEN status = 'down' THEN 1 ELSE 0 END) AS BIGINT),
		CAST(SUM(CASE WHEN status = 'missed' THEN 1 ELSE 0 END) AS BIGINT),
		COALESCE(AVG(CASE WHEN status <> 'missed' THEN latency_ms END), 0)
	FROM check_results
	WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY check_id`

	rows, err := db.TelemetryDB.QueryContext(ctx, query, projectId.String(), from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]*models.CheckAggregates)
	for rows.Next() {
		var checkId int64
		agg := &models.CheckAggregates{}
		if err := rows.Scan(&checkId, &agg.Total, &agg.Up, &agg.Down, &agg.Missed, &agg.AvgLatencyMs); err != nil {
			return nil, err
		}
		result[int(checkId)] = agg
	}
	return result, rows.Err()
}

// SeriesByCheck buckets one check's results. The explicit epoch origin on
// time_bucket keeps sub-day buckets aligned with the SQLite backend's
// epoch-floored buckets.
func (r *checkResultRepository) SeriesByCheck(ctx context.Context, projectId uuid.UUID, checkId int, from time.Time, to time.Time, intervalMinutes int) ([]*models.CheckSeriesPoint, error) {
	bucketSeconds := intervalMinutes * 60
	query := fmt.Sprintf(`SELECT CAST(epoch(time_bucket(to_seconds(%d), recorded_at, TIMESTAMP '1970-01-01')) AS BIGINT) AS bucket,
		CAST(COUNT(*) AS BIGINT),
		CAST(SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END) AS BIGINT),
		CAST(SUM(CASE WHEN status = 'missed' THEN 1 ELSE 0 END) AS BIGINT),
		COALESCE(AVG(CASE WHEN status <> 'missed' THEN latency_ms END), 0),
		COALESCE(MAX(CASE WHEN status <> 'missed' THEN latency_ms END), 0)
	FROM check_results
	WHERE project_id = ? AND check_id = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY bucket ORDER BY bucket ASC`, bucketSeconds)

	rows, err := db.TelemetryDB.QueryContext(ctx, query, projectId.String(), checkId, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []*models.CheckSeriesPoint
	for rows.Next() {
		var bucket int64
		point := &models.CheckSeriesPoint{}
		if err := rows.Scan(&bucket, &point.Total, &point.Up, &point.Missed, &point.AvgLatencyMs, &point.MaxLatencyMs); err != nil {
			return nil, err
		}
		point.Bucket = time.Unix(bucket, 0).UTC()
		points = append(points, point)
	}
	return points, rows.Err()
}

var CheckResultRepository = &checkResultRepository{}
