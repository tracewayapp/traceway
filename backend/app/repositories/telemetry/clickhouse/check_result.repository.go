//go:build telemetry_ch

package clickhouse

import (
	"context"
	"time"

	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type checkResultRepository struct{}

func (r *checkResultRepository) Insert(ctx context.Context, result shared.CheckResult) error {
	batch, err := chdb.Conn.PrepareBatch(
		chdb.BatchCtx(),
		"INSERT INTO check_results (project_id, check_id, run_id, check_type, recorded_at, executed_at, status, latency_ms, status_code, error_message, executed_by, screenshot_key, output_key, tls_days_remaining)",
	)
	if err != nil {
		return err
	}
	if err := batch.Append(
		result.ProjectId, int64(result.CheckId), int64(result.RunId), result.CheckType,
		result.RecordedAt, result.ExecutedAt, result.Status, result.LatencyMs,
		int32(result.StatusCode), result.ErrorMsg, result.ExecutedBy,
		result.ScreenshotKey, result.OutputKey, int32(result.TlsDaysRemaining),
	); err != nil {
		return err
	}
	return batch.Send()
}

func (r *checkResultRepository) FindByCheck(ctx context.Context, projectId uuid.UUID, checkId int, from *time.Time, to *time.Time, status string, page int, pageSize int) ([]*models.CheckResultEntry, int64, error) {
	where := "WHERE project_id = ? AND check_id = ?"
	args := []interface{}{projectId, int64(checkId)}

	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}

	if from != nil {
		where += " AND recorded_at >= ?"
		args = append(args, *from)
	}
	if to != nil {
		where += " AND recorded_at <= ?"
		args = append(args, *to)
	}

	var total uint64
	if err := chdb.Conn.QueryRow(ctx, "SELECT count() FROM check_results "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT run_id, check_type, recorded_at, executed_at, status, latency_ms, status_code, error_message, executed_by, screenshot_key, output_key, tls_days_remaining FROM check_results " +
		where + " ORDER BY recorded_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []*models.CheckResultEntry
	for rows.Next() {
		entry := &models.CheckResultEntry{}
		var runId int64
		var statusCode, tlsDaysRemaining int32
		if err := rows.Scan(
			&runId, &entry.CheckType, &entry.RecordedAt, &entry.ExecutedAt, &entry.Status,
			&entry.LatencyMs, &statusCode, &entry.ErrorMessage, &entry.ExecutedBy,
			&entry.ScreenshotKey, &entry.OutputKey, &tlsDaysRemaining,
		); err != nil {
			return nil, 0, err
		}
		entry.RunId = int(runId)
		entry.StatusCode = int(statusCode)
		entry.TlsDaysRemaining = int(tlsDaysRemaining)
		items = append(items, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, int64(total), nil
}

// AggregatesByProject returns per-check result summaries for the range.
func (r *checkResultRepository) AggregatesByProject(ctx context.Context, projectId uuid.UUID, from time.Time, to time.Time) (map[int]*models.CheckAggregates, error) {
	// avgIf over zero matching rows is NaN (not NULL), which encoding/json
	// rejects; the OrNull combinator makes coalesce actually apply.
	query := `SELECT check_id,
		count(),
		countIf(status = 'up'),
		countIf(status = 'down'),
		countIf(status = 'missed'),
		coalesce(avgOrNullIf(latency_ms, status != 'missed'), 0)
	FROM check_results
	WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY check_id`

	rows, err := chdb.Conn.Query(ctx, query, projectId, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]*models.CheckAggregates)
	for rows.Next() {
		var checkId int64
		var total, up, down, missed uint64
		var avgLatency float64
		if err := rows.Scan(&checkId, &total, &up, &down, &missed, &avgLatency); err != nil {
			return nil, err
		}
		result[int(checkId)] = &models.CheckAggregates{
			Total:        int64(total),
			Up:           int64(up),
			Down:         int64(down),
			Missed:       int64(missed),
			AvgLatencyMs: avgLatency,
		}
	}
	return result, rows.Err()
}

// SeriesByCheck buckets one check's results for the latency chart and uptime
// bars.
func (r *checkResultRepository) SeriesByCheck(ctx context.Context, projectId uuid.UUID, checkId int, from time.Time, to time.Time, intervalMinutes int) ([]*models.CheckSeriesPoint, error) {
	// OrNull for the same NaN reason as AggregatesByProject: an all-missed
	// bucket must yield 0, not NaN.
	query := `SELECT toStartOfInterval(recorded_at, INTERVAL ? minute) AS bucket,
		count(),
		countIf(status = 'up'),
		countIf(status = 'missed'),
		coalesce(avgOrNullIf(latency_ms, status != 'missed'), 0),
		coalesce(maxOrNullIf(latency_ms, status != 'missed'), 0)
	FROM check_results
	WHERE project_id = ? AND check_id = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY bucket ORDER BY bucket ASC`

	rows, err := chdb.Conn.Query(ctx, query, intervalMinutes, projectId, int64(checkId), from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []*models.CheckSeriesPoint
	for rows.Next() {
		var total, up, missed uint64
		point := &models.CheckSeriesPoint{}
		if err := rows.Scan(&point.Bucket, &total, &up, &missed, &point.AvgLatencyMs, &point.MaxLatencyMs); err != nil {
			return nil, err
		}
		point.Total = int64(total)
		point.Up = int64(up)
		point.Missed = int64(missed)
		points = append(points, point)
	}
	return points, rows.Err()
}

var CheckResultRepository = &checkResultRepository{}
