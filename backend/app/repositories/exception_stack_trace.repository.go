package repositories

import (
	"backend/app/chdb"
	"backend/app/models"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
)

var ErrExceptionNotFound = errors.New("exception not found")

type exceptionStackTraceRepository struct{}

func (e *exceptionStackTraceRepository) InsertAsync(ctx context.Context, lines []models.ExceptionStackTrace) error {
	batch, err := (*chdb.Conn).PrepareBatch(clickhouse.Context(context.Background(), clickhouse.WithAsync(false)), "INSERT INTO exception_stack_traces (id, project_id, transaction_id, transaction_type, exception_hash, stack_trace, recorded_at, scope, app_version, server_name, is_message)")
	if err != nil {
		return err
	}
	for _, est := range lines {
		scopeJSON := "{}"
		if len(est.Scope) != 0 {
			if scopeBytes, err := json.Marshal(est.Scope); err == nil {
				scopeJSON = string(scopeBytes)
			}
		}
		isMessage := uint8(0)
		if est.IsMessage {
			isMessage = 1
		}
		transactionType := est.TransactionType
		if transactionType == "" {
			transactionType = "endpoint"
		}
		if err := batch.Append(est.Id, est.ProjectId, est.TransactionId, transactionType, est.ExceptionHash, est.StackTrace, est.RecordedAt, scopeJSON, est.AppVersion, est.ServerName, isMessage); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (e *exceptionStackTraceRepository) CountBetween(ctx context.Context, projectId uuid.UUID, start, end time.Time) (int64, error) {
	var count uint64
	err := (*chdb.Conn).QueryRow(ctx, "SELECT count() FROM exception_stack_traces WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?", projectId, start, end).Scan(&count)
	return int64(count), err
}

func (e *exceptionStackTraceRepository) FindGrouped(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy string, search string, searchType string, includeArchived bool) ([]models.ExceptionGroup, int64, error) {
	offset := (page - 1) * pageSize

	// Parse sort direction from orderBy (e.g., "last_seen_asc" -> "last_seen", "ASC")
	sortDirection := "DESC"
	if strings.HasSuffix(orderBy, "_asc") {
		orderBy = strings.TrimSuffix(orderBy, "_asc")
		sortDirection = "ASC"
	}

	allowedOrderBy := map[string]bool{
		"last_seen":  true,
		"first_seen": true,
		"count":      true,
	}

	if !allowedOrderBy[orderBy] {
		orderBy = "count"
	}

	// Build WHERE clause dynamically based on search filter
	whereClause := "e.project_id = ? AND e.recorded_at >= ? AND e.recorded_at <= ?"
	args := []interface{}{projectId, fromDate, toDate}

	if search != "" {
		whereClause += " AND positionCaseInsensitive(e.stack_trace, ?) > 0"
		args = append(args, search)
	}

	// Add searchType filter
	if searchType == "issues" {
		whereClause += " AND e.is_message = 0"
	} else if searchType == "messages" {
		whereClause += " AND e.is_message = 1"
	}
	// "all" or empty = no filter

	// Build HAVING clause for archive filtering
	// Show exceptions if: not archived OR last occurrence is after archive time
	havingClause := ""
	if !includeArchived {
		havingClause = " HAVING any(a.archived_at) IS NULL OR max(e.recorded_at) > any(a.archived_at)"
	}

	// Subquery to get max archived_at per exception hash
	archiveSubquery := `LEFT JOIN (
		SELECT exception_hash, max(archived_at) as archived_at
		FROM archived_exceptions FINAL
		WHERE project_id = ?
		GROUP BY exception_hash
	) a ON e.exception_hash = a.exception_hash`

	// Count query needs to wrap the grouped query to apply HAVING filter correctly
	countQuery := `SELECT count() FROM (
		SELECT e.exception_hash
		FROM exception_stack_traces e
		` + archiveSubquery + `
		WHERE ` + whereClause + `
		GROUP BY e.exception_hash` + havingClause + `
	)`

	countArgs := append([]interface{}{projectId}, args...)
	var count uint64
	err := (*chdb.Conn).QueryRow(ctx, countQuery, countArgs...).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	// Main query with archive-aware filtering
	fullQuery := `SELECT e.exception_hash, any(e.stack_trace), max(e.recorded_at) as last_seen, min(e.recorded_at) as first_seen, count() as count
		FROM exception_stack_traces e
		` + archiveSubquery + `
		WHERE ` + whereClause + `
		GROUP BY e.exception_hash` + havingClause + `
		ORDER BY ` + orderBy + ` ` + sortDirection + ` LIMIT ? OFFSET ?`

	queryArgs := append([]interface{}{projectId}, args...)
	queryArgs = append(queryArgs, pageSize, offset)
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

func (e *exceptionStackTraceRepository) FindByHash(ctx context.Context, projectId uuid.UUID, exceptionHash string, page, pageSize int) (*models.ExceptionGroup, []models.ExceptionStackTrace, int64, error) {
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

	// Get individual occurrences with pagination (including scope)
	rows, err := (*chdb.Conn).Query(ctx,
		"SELECT id, project_id, transaction_id, transaction_type, exception_hash, stack_trace, recorded_at, scope, app_version, server_name, is_message FROM exception_stack_traces WHERE project_id = ? AND exception_hash = ? ORDER BY recorded_at DESC LIMIT ? OFFSET ?",
		projectId, exceptionHash, pageSize, offset)
	if err != nil {
		return nil, nil, 0, err
	}
	defer rows.Close()

	var occurrences []models.ExceptionStackTrace
	for rows.Next() {
		var o models.ExceptionStackTrace
		var scopeJSON string
		var isMessage uint8
		if err := rows.Scan(&o.Id, &o.ProjectId, &o.TransactionId, &o.TransactionType, &o.ExceptionHash, &o.StackTrace, &o.RecordedAt, &scopeJSON, &o.AppVersion, &o.ServerName, &isMessage); err != nil {
			return nil, nil, 0, err
		}
		o.IsMessage = isMessage == 1
		// Parse scope JSON
		if scopeJSON != "" && scopeJSON != "{}" {
			if err := json.Unmarshal([]byte(scopeJSON), &o.Scope); err != nil {
				o.Scope = nil // If parsing fails, leave scope as nil
			}
		}
		occurrences = append(occurrences, o)
	}

	return &group, occurrences, int64(group.Count), nil
}

// CountByHour returns exception counts grouped by hour
func (e *exceptionStackTraceRepository) CountByHour(ctx context.Context, projectId uuid.UUID, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfHour(recorded_at) as hour,
		toFloat64(count()) as count
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

// CountByInterval returns exception counts grouped by configurable interval in minutes
func (e *exceptionStackTraceRepository) CountByInterval(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfInterval(recorded_at, INTERVAL ? MINUTE) as bucket,
		toFloat64(count()) as count
	FROM exception_stack_traces
	WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY bucket
	ORDER BY bucket ASC`

	rows, err := (*chdb.Conn).Query(ctx, query, intervalMinutes, projectId, start, end)
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

// GetHourlyTrendForHashes returns hourly counts for specific exception hashes
func (e *exceptionStackTraceRepository) GetHourlyTrendForHashes(ctx context.Context, projectId uuid.UUID, hashes []string, start, end time.Time) (map[string][]models.ExceptionTrendPoint, error) {
	if len(hashes) == 0 {
		return make(map[string][]models.ExceptionTrendPoint), nil
	}

	query := `SELECT
		exception_hash,
		toStartOfHour(recorded_at) as hour,
		count() as count
	FROM exception_stack_traces
	WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ? AND exception_hash IN (?)
	GROUP BY exception_hash, hour
	ORDER BY exception_hash, hour ASC`

	rows, err := (*chdb.Conn).Query(ctx, query, projectId, start, end, hashes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]models.ExceptionTrendPoint)
	for rows.Next() {
		var hash string
		var point models.ExceptionTrendPoint
		if err := rows.Scan(&hash, &point.Timestamp, &point.Count); err != nil {
			return nil, err
		}
		result[hash] = append(result[hash], point)
	}

	return result, nil
}

// ArchiveByHashes archives exceptions by their hashes
func (e *exceptionStackTraceRepository) ArchiveByHashes(ctx context.Context, projectId uuid.UUID, hashes []string) error {
	if len(hashes) == 0 {
		return nil
	}

	batch, err := (*chdb.Conn).PrepareBatch(ctx, "INSERT INTO archived_exceptions (project_id, exception_hash)")
	if err != nil {
		return err
	}

	for _, hash := range hashes {
		if err := batch.Append(projectId, hash); err != nil {
			return err
		}
	}

	return batch.Send()
}

// UnarchiveByHashes removes exceptions from the archive
func (e *exceptionStackTraceRepository) UnarchiveByHashes(ctx context.Context, projectId uuid.UUID, hashes []string) error {
	if len(hashes) == 0 {
		return nil
	}

	// In ClickHouse, we use ALTER TABLE DELETE for removing rows
	query := "ALTER TABLE archived_exceptions DELETE WHERE project_id = ? AND exception_hash IN (?)"
	return (*chdb.Conn).Exec(ctx, query, projectId, hashes)
}

// IsArchived checks if a specific exception hash is archived
func (e *exceptionStackTraceRepository) IsArchived(ctx context.Context, projectId uuid.UUID, hash string) (bool, error) {
	var count uint64
	err := (*chdb.Conn).QueryRow(ctx,
		"SELECT count() FROM archived_exceptions FINAL WHERE project_id = ? AND exception_hash = ?",
		projectId, hash).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (e *exceptionStackTraceRepository) FindExceptionByTransactionId(ctx context.Context, projectId uuid.UUID, transactionId uuid.UUID) (*models.ExceptionStackTrace, error) {
	var est models.ExceptionStackTrace
	var scopeJSON string
	var isMessage uint8

	err := (*chdb.Conn).QueryRow(ctx,
		`SELECT id, project_id, transaction_id, transaction_type, exception_hash, stack_trace, recorded_at, scope, app_version, server_name, is_message
		FROM exception_stack_traces
		WHERE project_id = ? AND transaction_id = ? AND is_message = false
		LIMIT 1`,
		projectId, transactionId).Scan(
		&est.Id, &est.ProjectId, &est.TransactionId, &est.TransactionType, &est.ExceptionHash, &est.StackTrace,
		&est.RecordedAt, &scopeJSON, &est.AppVersion, &est.ServerName, &isMessage)

	if err != nil {
		// No exception found for this transaction
		return nil, nil
	}

	est.IsMessage = isMessage == 1
	// Parse scope JSON
	if scopeJSON != "" && scopeJSON != "{}" {
		if err := json.Unmarshal([]byte(scopeJSON), &est.Scope); err != nil {
			est.Scope = nil
		}
	}

	return &est, nil
}

// FindAllByTransactionId returns all exceptions and messages associated with a specific transaction
func (e *exceptionStackTraceRepository) FindAllByTransactionId(ctx context.Context, projectId uuid.UUID, transactionId uuid.UUID) ([]models.ExceptionStackTrace, error) {
	rows, err := (*chdb.Conn).Query(ctx,
		`SELECT id, project_id, transaction_id, transaction_type, exception_hash, stack_trace, recorded_at, scope, app_version, server_name, is_message
		FROM exception_stack_traces
		WHERE project_id = ? AND transaction_id = ?
		ORDER BY recorded_at ASC`,
		projectId, transactionId)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.ExceptionStackTrace
	for rows.Next() {
		var est models.ExceptionStackTrace
		var scopeJSON string
		var isMessage uint8

		if err := rows.Scan(&est.Id, &est.ProjectId, &est.TransactionId, &est.TransactionType, &est.ExceptionHash, &est.StackTrace,
			&est.RecordedAt, &scopeJSON, &est.AppVersion, &est.ServerName, &isMessage); err != nil {
			return nil, err
		}

		est.IsMessage = isMessage == 1
		if scopeJSON != "" && scopeJSON != "{}" {
			if err := json.Unmarshal([]byte(scopeJSON), &est.Scope); err != nil {
				est.Scope = nil
			}
		}
		results = append(results, est)
	}

	return results, nil
}

// FindById returns a single exception by its ID
func (e *exceptionStackTraceRepository) FindById(ctx context.Context, projectId uuid.UUID, id uuid.UUID) (*models.ExceptionStackTrace, error) {
	var est models.ExceptionStackTrace
	var scopeJSON string
	var isMessage uint8

	err := (*chdb.Conn).QueryRow(ctx,
		`SELECT id, project_id, transaction_id, transaction_type, exception_hash, stack_trace, recorded_at, scope, app_version, server_name, is_message
		FROM exception_stack_traces
		WHERE project_id = ? AND id = ?
		LIMIT 1`,
		projectId, id).Scan(
		&est.Id, &est.ProjectId, &est.TransactionId, &est.TransactionType, &est.ExceptionHash, &est.StackTrace,
		&est.RecordedAt, &scopeJSON, &est.AppVersion, &est.ServerName, &isMessage)

	if err != nil {
		return nil, ErrExceptionNotFound
	}

	est.IsMessage = isMessage == 1
	// Parse scope JSON
	if scopeJSON != "" && scopeJSON != "{}" {
		if err := json.Unmarshal([]byte(scopeJSON), &est.Scope); err != nil {
			est.Scope = nil
		}
	}

	return &est, nil
}

var ExceptionStackTraceRepository = exceptionStackTraceRepository{}
