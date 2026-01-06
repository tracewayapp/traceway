package repositories

import (
	"backend/app/chdb"
	"backend/app/models"
	"context"
	"errors"
	"time"
)

var ErrExceptionNotFound = errors.New("exception not found")

type exceptionStackTraceRepository struct{}

func (e *exceptionStackTraceRepository) InsertAsync(ctx context.Context, lines []models.ExceptionStackTrace) error {
	batch, err := (*chdb.Conn).PrepareBatch(ctx, "INSERT INTO exception_stack_traces (project_id, transaction_id, exception_hash, stack_trace, recorded_at)")
	if err != nil {
		return err
	}
	for _, est := range lines {
		if err := batch.Append(est.ProjectId, est.TransactionId, est.ExceptionHash, est.StackTrace, est.RecordedAt); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (e *exceptionStackTraceRepository) CountBetween(ctx context.Context, projectId string, start, end time.Time) (int64, error) {
	var count uint64
	err := (*chdb.Conn).QueryRow(ctx, "SELECT count() FROM exception_stack_traces WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?", projectId, start, end).Scan(&count)
	return int64(count), err
}

func (e *exceptionStackTraceRepository) FindGrouped(ctx context.Context, projectId string, fromDate, toDate time.Time, page, pageSize int, orderBy string, search string) ([]models.ExceptionGroup, int64, error) {
	offset := (page - 1) * pageSize

	allowedOrderBy := map[string]bool{
		"last_seen":  true,
		"first_seen": true,
		"count":      true,
	}

	if !allowedOrderBy[orderBy] {
		orderBy = "count"
	}

	// Build WHERE clause dynamically based on search
	whereClause := "project_id = ? AND recorded_at >= ? AND recorded_at <= ?"
	args := []interface{}{projectId, fromDate, toDate}

	if search != "" {
		whereClause += " AND positionCaseInsensitive(stack_trace, ?) > 0"
		args = append(args, search)
	}

	// Count unique hashes with search filter
	countQuery := "SELECT uniq(exception_hash) FROM exception_stack_traces WHERE " + whereClause
	var count uint64
	err := (*chdb.Conn).QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	// Main query with search filter
	fullQuery := "SELECT exception_hash, any(stack_trace), max(recorded_at) as last_seen, min(recorded_at) as first_seen, count() as count FROM exception_stack_traces WHERE " + whereClause + " GROUP BY exception_hash ORDER BY " + orderBy + " DESC LIMIT ? OFFSET ?"

	queryArgs := append(args, pageSize, offset)
	rows, err := (*chdb.Conn).Query(ctx, fullQuery, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var groups []models.ExceptionGroup
	for rows.Next() {
		var g models.ExceptionGroup
		if err := rows.Scan(&g.ExceptionHash, &g.StackTrace, &g.LastSeen, &g.FirstSeen, &g.Count); err != nil {
			return nil, 0, err
		}
		groups = append(groups, g)
	}

	return groups, int64(count), nil
}

func (e *exceptionStackTraceRepository) FindByHash(ctx context.Context, projectId string, exceptionHash string, page, pageSize int) (*models.ExceptionGroup, []models.ExceptionStackTrace, int64, error) {
	offset := (page - 1) * pageSize

	// Get grouped info
	var group models.ExceptionGroup
	err := (*chdb.Conn).QueryRow(ctx,
		"SELECT exception_hash, any(stack_trace), max(recorded_at) as last_seen, min(recorded_at) as first_seen, count() as count FROM exception_stack_traces WHERE project_id = ? AND exception_hash = ? GROUP BY exception_hash",
		projectId, exceptionHash).Scan(&group.ExceptionHash, &group.StackTrace, &group.LastSeen, &group.FirstSeen, &group.Count)
	if err != nil {
		// ClickHouse returns error when no rows found in QueryRow
		return nil, nil, 0, ErrExceptionNotFound
	}

	// Get individual occurrences with pagination
	rows, err := (*chdb.Conn).Query(ctx,
		"SELECT project_id, transaction_id, exception_hash, stack_trace, recorded_at FROM exception_stack_traces WHERE project_id = ? AND exception_hash = ? ORDER BY recorded_at DESC LIMIT ? OFFSET ?",
		projectId, exceptionHash, pageSize, offset)
	if err != nil {
		return nil, nil, 0, err
	}
	defer rows.Close()

	var occurrences []models.ExceptionStackTrace
	for rows.Next() {
		var o models.ExceptionStackTrace
		if err := rows.Scan(&o.ProjectId, &o.TransactionId, &o.ExceptionHash, &o.StackTrace, &o.RecordedAt); err != nil {
			return nil, nil, 0, err
		}
		occurrences = append(occurrences, o)
	}

	return &group, occurrences, int64(group.Count), nil
}

// CountByHour returns exception counts grouped by hour
func (e *exceptionStackTraceRepository) CountByHour(ctx context.Context, projectId string, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfHour(recorded_at) as hour,
		count() as count
	FROM exception_stack_traces
	WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY hour
	ORDER BY hour ASC`

	rows, err := (*chdb.Conn).Query(ctx, query, projectId, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.TimeSeriesPoint
	for rows.Next() {
		var p models.TimeSeriesPoint
		if err := rows.Scan(&p.Timestamp, &p.Value); err != nil {
			return nil, err
		}
		points = append(points, p)
	}

	return points, nil
}

var ExceptionStackTraceRepository = exceptionStackTraceRepository{}
