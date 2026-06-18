//go:build !pgch

package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type exceptionRow struct {
	Id                 uuid.UUID     `lit:"id"`
	ProjectId          uuid.UUID     `lit:"project_id"`
	TraceId            *uuid.UUID    `lit:"trace_id"`
	TraceType          string        `lit:"trace_type"`
	ExceptionHash      string        `lit:"exception_hash"`
	StackTrace         string        `lit:"stack_trace"`
	RecordedAt         SQLiteTime    `lit:"recorded_at"`
	Attributes         SQLiteJSONMap `lit:"attributes"`
	AppVersion         string        `lit:"app_version"`
	ServerName         string        `lit:"server_name"`
	IsMessage          bool          `lit:"is_message"`
	DistributedTraceId *uuid.UUID    `lit:"distributed_trace_id"`
	SessionId          *uuid.UUID    `lit:"session_id"`
}

type exceptionRowNaming struct{ lit.DefaultDbNamingStrategy }

func (exceptionRowNaming) GetTableNameFromStructName(string) string {
	return "exception_stack_traces"
}

type exceptionGroupRow struct {
	ExceptionHash string `lit:"exception_hash"`
	StackTrace    string `lit:"stack_trace"`
	LastSeen      string `lit:"last_seen"`
	FirstSeen     string `lit:"first_seen"`
	Count         uint64 `lit:"count"`
	MaxArchivedAt *string `lit:"max_archived_at"`
}

type exceptionTrendRow struct {
	Hash      string `lit:"exception_hash"`
	Hour      string `lit:"hour"`
	Count     uint64 `lit:"count"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModelWithNaming[exceptionRow](driver, exceptionRowNaming{})
		lit.RegisterModel[exceptionGroupRow](driver)
		lit.RegisterModel[exceptionTrendRow](driver)
	})
}

func exceptionToRow(est models.ExceptionStackTrace) exceptionRow {
	traceType := est.TraceType
	if traceType == "" {
		traceType = "endpoint"
	}
	return exceptionRow{
		Id:                 est.Id,
		ProjectId:          est.ProjectId,
		TraceId:            est.TraceId,
		TraceType:          traceType,
		ExceptionHash:      est.ExceptionHash,
		StackTrace:         est.StackTrace,
		RecordedAt:         NewSQLiteTime(est.RecordedAt),
		Attributes:         NewSQLiteJSONMap(est.Attributes),
		AppVersion:         est.AppVersion,
		ServerName:         est.ServerName,
		IsMessage:          est.IsMessage,
		DistributedTraceId: est.DistributedTraceId,
		SessionId:          est.SessionId,
	}
}

func (r *exceptionRow) toModel() models.ExceptionStackTrace {
	est := models.ExceptionStackTrace{
		Id:                 r.Id,
		ProjectId:          r.ProjectId,
		TraceId:            r.TraceId,
		TraceType:          r.TraceType,
		ExceptionHash:      r.ExceptionHash,
		StackTrace:         r.StackTrace,
		RecordedAt:         r.RecordedAt.Time,
		AppVersion:         r.AppVersion,
		ServerName:         r.ServerName,
		IsMessage:          r.IsMessage,
		DistributedTraceId: r.DistributedTraceId,
		SessionId:          r.SessionId,
	}
	if r.Attributes != nil {
		est.Attributes = map[string]string(r.Attributes)
	}
	return est
}

type exceptionStackTraceRepository struct{}

func (e *exceptionStackTraceRepository) InsertAsync(ctx context.Context, lines []models.ExceptionStackTrace) error {
	if len(lines) == 0 {
		return nil
	}

	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, est := range lines {
		row := exceptionToRow(est)
		if err := lit.InsertExistingUuid(tx, &row); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (e *exceptionStackTraceRepository) CountBetween(ctx context.Context, projectId uuid.UUID, start, end time.Time) (int64, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		"SELECT COUNT(*) AS count FROM exception_stack_traces WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to",
		lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)})
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return int64(result.Count), nil
}

func (e *exceptionStackTraceRepository) FindGrouped(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy string, search string, searchType string, includeArchived bool) ([]models.ExceptionGroup, int64, error) {
	offset := (page - 1) * pageSize

	sortDirection := "DESC"
	if strings.HasSuffix(orderBy, "_asc") {
		orderBy = strings.TrimSuffix(orderBy, "_asc")
		sortDirection = "ASC"
	}

	allowedOrderBy := map[string]bool{"last_seen": true, "first_seen": true, "count": true}
	if !allowedOrderBy[orderBy] {
		orderBy = "count"
	}

	params := lit.P{"project_id": projectId, "from": NewSQLiteTime(fromDate), "to": NewSQLiteTime(toDate)}

	whereClause := "e.project_id = :project_id AND e.recorded_at >= :from AND e.recorded_at <= :to"
	if search != "" {
		whereClause += " AND INSTR(LOWER(e.stack_trace), LOWER(:search)) > 0"
		params["search"] = search
	}
	if searchType == "issues" {
		whereClause += " AND e.is_message = 0"
	} else if searchType == "messages" {
		whereClause += " AND e.is_message = 1"
	}

	havingClause := ""
	if !includeArchived {
		havingClause = " HAVING max_archived_at IS NULL OR MAX(e.recorded_at) > max_archived_at"
	}

	archiveSubquery := `LEFT JOIN (
		SELECT exception_hash, MAX(archived_at) as archived_at
		FROM archived_exceptions WHERE project_id = :archive_project_id
		GROUP BY exception_hash
	) a ON e.exception_hash = a.exception_hash`
	params["archive_project_id"] = projectId

	countQuery := `SELECT COUNT(*) AS count FROM (
		SELECT e.exception_hash, MAX(a.archived_at) as max_archived_at
		FROM exception_stack_traces e ` + archiveSubquery + `
		WHERE ` + whereClause + `
		GROUP BY e.exception_hash` + havingClause + `)`

	countResult, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB, countQuery, params)
	if err != nil {
		return nil, 0, err
	}
	count := int64(0)
	if countResult != nil {
		count = int64(countResult.Count)
	}

	fullQuery := `SELECT e.exception_hash, e.stack_trace, MAX(e.recorded_at) as last_seen, MIN(e.recorded_at) as first_seen, COUNT(*) as count, MAX(a.archived_at) as max_archived_at
		FROM exception_stack_traces e ` + archiveSubquery + `
		WHERE ` + whereClause + `
		GROUP BY e.exception_hash` + havingClause + `
		ORDER BY ` + orderBy + ` ` + sortDirection + ` LIMIT :limit OFFSET :offset`
	params["limit"] = pageSize
	params["offset"] = offset

	groupRows, err := lit.SelectNamed[exceptionGroupRow](db.TelemetryDB, fullQuery, params)
	if err != nil {
		return nil, 0, err
	}

	groups := make([]models.ExceptionGroup, 0, len(groupRows))
	for _, g := range groupRows {
		lastSeen, _ := time.Parse(time.RFC3339Nano, g.LastSeen)
		firstSeen, _ := time.Parse(time.RFC3339Nano, g.FirstSeen)
		groups = append(groups, models.ExceptionGroup{
			ExceptionHash: g.ExceptionHash,
			StackTrace:    g.StackTrace,
			LastSeen:      lastSeen,
			FirstSeen:     firstSeen,
			Count:         g.Count,
		})
	}

	return groups, count, nil
}

func (e *exceptionStackTraceRepository) FindByHash(ctx context.Context, projectId uuid.UUID, exceptionHash string, page, pageSize int) (*models.ExceptionGroup, []models.ExceptionStackTrace, int64, error) {
	offset := (page - 1) * pageSize
	params := lit.P{"project_id": projectId, "exception_hash": exceptionHash}

	groupRow, err := lit.SelectSingleNamed[exceptionGroupRow](db.TelemetryDB,
		`SELECT exception_hash, stack_trace, MAX(recorded_at) as last_seen, MIN(recorded_at) as first_seen, COUNT(*) as count, NULL as max_archived_at
		FROM exception_stack_traces WHERE project_id = :project_id AND exception_hash = :exception_hash
		GROUP BY exception_hash`,
		params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, 0, nil
		}
		return nil, nil, 0, err
	}
	if groupRow == nil {
		return nil, nil, 0, nil
	}

	lastSeen, _ := time.Parse(time.RFC3339Nano, groupRow.LastSeen)
	firstSeen, _ := time.Parse(time.RFC3339Nano, groupRow.FirstSeen)
	group := &models.ExceptionGroup{
		ExceptionHash: groupRow.ExceptionHash,
		StackTrace:    groupRow.StackTrace,
		LastSeen:      lastSeen,
		FirstSeen:     firstSeen,
		Count:         groupRow.Count,
	}

	rows, err := lit.SelectNamed[exceptionRow](db.TelemetryDB,
		`SELECT id, project_id, trace_id, trace_type, exception_hash, stack_trace, recorded_at, attributes, app_version, server_name, is_message, distributed_trace_id, session_id
		FROM exception_stack_traces WHERE project_id = :project_id AND exception_hash = :exception_hash
		ORDER BY recorded_at DESC LIMIT :limit OFFSET :offset`,
		lit.P{"project_id": projectId, "exception_hash": exceptionHash, "limit": pageSize, "offset": offset})
	if err != nil {
		return nil, nil, 0, err
	}

	occurrences := make([]models.ExceptionStackTrace, 0, len(rows))
	for _, row := range rows {
		occurrences = append(occurrences, row.toModel())
	}

	return group, occurrences, int64(group.Count), nil
}

func (e *exceptionStackTraceRepository) CountByHour(ctx context.Context, projectId uuid.UUID, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	results, err := lit.SelectNamed[timeSeriesResult](db.TelemetryDB,
		`SELECT strftime('%Y-%m-%d %H:00:00', recorded_at) as bucket, CAST(COUNT(*) AS REAL) as agg_value
		FROM exception_stack_traces WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY bucket ORDER BY bucket ASC`,
		lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)})
	if err != nil {
		return nil, err
	}
	return timeSeriesResultsToPoints(results), nil
}

func (e *exceptionStackTraceRepository) CountByInterval(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	intervalSeconds := intervalMinutes * 60
	results, err := lit.SelectNamed[timeSeriesResult](db.TelemetryDB,
		fmt.Sprintf(`SELECT datetime((strftime('%%s', recorded_at) / %d) * %d, 'unixepoch') as bucket, CAST(COUNT(*) AS REAL) as agg_value
		FROM exception_stack_traces WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY bucket ORDER BY bucket ASC`, intervalSeconds, intervalSeconds),
		lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)})
	if err != nil {
		return nil, err
	}
	return timeSeriesResultsToPoints(results), nil
}

func (e *exceptionStackTraceRepository) GetHourlyTrendForHashes(ctx context.Context, projectId uuid.UUID, hashes []string, start, end time.Time) (map[string][]models.ExceptionTrendPoint, error) {
	if len(hashes) == 0 {
		return make(map[string][]models.ExceptionTrendPoint), nil
	}

	params := lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)}
	placeholders := make([]string, len(hashes))
	for i, h := range hashes {
		key := fmt.Sprintf("hash_%d", i)
		placeholders[i] = ":" + key
		params[key] = h
	}

	query := `SELECT exception_hash, strftime('%Y-%m-%d %H:00:00', recorded_at) as hour, COUNT(*) as count
		FROM exception_stack_traces
		WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		AND exception_hash IN (` + strings.Join(placeholders, ",") + `)
		GROUP BY exception_hash, hour ORDER BY exception_hash, hour ASC`

	trendRows, err := lit.SelectNamed[exceptionTrendRow](db.TelemetryDB, query, params)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]models.ExceptionTrendPoint)
	for _, row := range trendRows {
		ts, _ := time.Parse("2006-01-02 15:04:05", row.Hour)
		result[row.Hash] = append(result[row.Hash], models.ExceptionTrendPoint{
			Timestamp: ts,
			Count:     row.Count,
		})
	}

	return result, nil
}

func (e *exceptionStackTraceRepository) ArchiveByHashes(ctx context.Context, projectId uuid.UUID, hashes []string) error {
	if len(hashes) == 0 {
		return nil
	}

	now := NewSQLiteTime(time.Now().UTC())
	for _, hash := range hashes {
		query, args, err := lit.ParseNamedQuery(db.Driver,
			"INSERT OR REPLACE INTO archived_exceptions (project_id, exception_hash, archived_at) VALUES (:project_id, :exception_hash, :archived_at)",
			lit.P{"project_id": projectId, "exception_hash": hash, "archived_at": now})
		if err != nil {
			return err
		}
		if _, err := db.TelemetryDB.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}

	return nil
}

func (e *exceptionStackTraceRepository) UnarchiveByHashes(ctx context.Context, projectId uuid.UUID, hashes []string) error {
	if len(hashes) == 0 {
		return nil
	}

	params := lit.P{"project_id": projectId}
	placeholders := make([]string, len(hashes))
	for i, h := range hashes {
		key := fmt.Sprintf("hash_%d", i)
		placeholders[i] = ":" + key
		params[key] = h
	}

	query := "DELETE FROM archived_exceptions WHERE project_id = :project_id AND exception_hash IN (" + strings.Join(placeholders, ",") + ")"
	parsedQuery, args, err := lit.ParseNamedQuery(db.Driver, query, params)
	if err != nil {
		return err
	}

	_, err = db.TelemetryDB.ExecContext(ctx, parsedQuery, args...)
	return err
}

func (e *exceptionStackTraceRepository) IsArchived(ctx context.Context, projectId uuid.UUID, hash string) (bool, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		"SELECT COUNT(*) AS count FROM archived_exceptions WHERE project_id = :project_id AND exception_hash = :exception_hash",
		lit.P{"project_id": projectId, "exception_hash": hash})
	if err != nil {
		return false, err
	}
	if result == nil {
		return false, nil
	}
	return result.Count > 0, nil
}

func (e *exceptionStackTraceRepository) FindExceptionByTraceId(ctx context.Context, projectId uuid.UUID, traceId uuid.UUID) (*models.ExceptionStackTrace, error) {
	row, err := lit.SelectSingleNamed[exceptionRow](db.TelemetryDB,
		`SELECT id, project_id, trace_id, trace_type, exception_hash, stack_trace, recorded_at, attributes, app_version, server_name, is_message, distributed_trace_id, session_id
		FROM exception_stack_traces WHERE project_id = :project_id AND trace_id = :trace_id AND is_message = 0 LIMIT 1`,
		lit.P{"project_id": projectId, "trace_id": traceId})
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	est := row.toModel()
	return &est, nil
}

func (e *exceptionStackTraceRepository) FindAllByTraceId(ctx context.Context, projectId uuid.UUID, traceId uuid.UUID, recordedAt *time.Time) ([]models.ExceptionStackTrace, error) {
	query := `SELECT id, project_id, trace_id, trace_type, exception_hash, stack_trace, recorded_at, attributes, app_version, server_name, is_message, distributed_trace_id, session_id
		FROM exception_stack_traces WHERE project_id = :project_id AND trace_id = :trace_id`
	params := lit.P{"project_id": projectId, "trace_id": traceId}
	if recordedAt != nil {
		from, to := traceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= :from AND recorded_at <= :to`
		params["from"] = NewSQLiteTime(from)
		params["to"] = NewSQLiteTime(to)
	}
	query += ` ORDER BY recorded_at ASC`

	rows, err := lit.SelectNamed[exceptionRow](db.TelemetryDB, query, params)
	if err != nil {
		return nil, err
	}

	results := make([]models.ExceptionStackTrace, 0, len(rows))
	for _, row := range rows {
		results = append(results, row.toModel())
	}
	return results, nil
}

func (e *exceptionStackTraceRepository) FindById(ctx context.Context, projectId uuid.UUID, id uuid.UUID, recordedAt *time.Time) (*models.ExceptionStackTrace, error) {
	query := `SELECT id, project_id, trace_id, trace_type, exception_hash, stack_trace, recorded_at, attributes, app_version, server_name, is_message, distributed_trace_id, session_id
		FROM exception_stack_traces WHERE project_id = :project_id AND id = :id`
	params := lit.P{"project_id": projectId, "id": id}
	if recordedAt != nil {
		from, to := traceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= :from AND recorded_at <= :to`
		params["from"] = NewSQLiteTime(from)
		params["to"] = NewSQLiteTime(to)
	}
	query += ` LIMIT 1`

	row, err := lit.SelectSingleNamed[exceptionRow](db.TelemetryDB, query, params)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	est := row.toModel()
	return &est, nil
}

func (e *exceptionStackTraceRepository) FindByDistributedTraceId(ctx context.Context, distributedTraceId uuid.UUID, projectIds []uuid.UUID, recordedAt *time.Time) ([]models.ExceptionStackTrace, error) {
	if len(projectIds) == 0 {
		return nil, nil
	}
	params := lit.P{"trace_id": distributedTraceId}
	placeholders := make([]string, len(projectIds))
	for i, pid := range projectIds {
		key := fmt.Sprintf("pid_%d", i)
		placeholders[i] = ":" + key
		params[key] = pid
	}

	query := `SELECT id, project_id, trace_id, trace_type, exception_hash, stack_trace, recorded_at, attributes, app_version, server_name, is_message, distributed_trace_id, session_id
		FROM exception_stack_traces WHERE distributed_trace_id = :trace_id AND project_id IN (` + strings.Join(placeholders, ",") + `) AND is_message = 0`
	if recordedAt != nil {
		from, to := distributedTraceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= :from AND recorded_at <= :to`
		params["from"] = NewSQLiteTime(from)
		params["to"] = NewSQLiteTime(to)
	}
	query += ` ORDER BY recorded_at ASC`

	parsedQuery, args, err := lit.ParseNamedQuery(db.Driver, query, params)
	if err != nil {
		return nil, err
	}

	sqlRows, err := db.TelemetryDB.QueryContext(ctx, parsedQuery, args...)
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()

	var results []models.ExceptionStackTrace
	for sqlRows.Next() {
		var row exceptionRow
		if err := sqlRows.Scan(&row.Id, &row.ProjectId, &row.TraceId, &row.TraceType, &row.ExceptionHash, &row.StackTrace,
			&row.RecordedAt, &row.Attributes, &row.AppVersion, &row.ServerName, &row.IsMessage, &row.DistributedTraceId, &row.SessionId); err != nil {
			return nil, err
		}
		results = append(results, row.toModel())
	}
	return results, nil
}

// FindAllBySessionId returns all exceptions/messages stamped with the given session_id, ordered by time.
func (e *exceptionStackTraceRepository) FindAllBySessionId(ctx context.Context, projectId, sessionId uuid.UUID) ([]models.ExceptionStackTrace, error) {
	rows, err := lit.SelectNamed[exceptionRow](db.TelemetryDB,
		`SELECT id, project_id, trace_id, trace_type, exception_hash, stack_trace, recorded_at, attributes, app_version, server_name, is_message, distributed_trace_id, session_id
			FROM exception_stack_traces
			WHERE project_id = :project_id AND session_id = :session_id
			ORDER BY recorded_at ASC`,
		lit.P{"project_id": projectId, "session_id": sessionId})
	if err != nil {
		return nil, err
	}
	out := make([]models.ExceptionStackTrace, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toModel())
	}
	return out, nil
}

// GetSessionIdForException returns the session_id for the given exception or nil when not linked.
func (e *exceptionStackTraceRepository) GetSessionIdForException(ctx context.Context, projectId, exceptionId uuid.UUID) (*uuid.UUID, error) {
	row, err := lit.SelectSingleNamed[exceptionRow](db.TelemetryDB,
		`SELECT id, project_id, trace_id, trace_type, exception_hash, stack_trace, recorded_at, attributes, app_version, server_name, is_message, distributed_trace_id, session_id
			FROM exception_stack_traces
			WHERE project_id = :project_id AND id = :id LIMIT 1`,
		lit.P{"project_id": projectId, "id": exceptionId})
	if err != nil || row == nil {
		return nil, err
	}
	return row.SessionId, nil
}

var ExceptionStackTraceRepository = exceptionStackTraceRepository{}
