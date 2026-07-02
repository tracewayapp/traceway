//go:build duckdb && !pgch

package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/duckdb/duckdb-go/v2"
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

// In DuckDB the aggregated timestamp/count columns come back as native
// TIMESTAMP/BIGINT values, so these fields scan via SQLiteTime / sql.NullTime
// rather than the raw strings the SQLite driver returns.
type exceptionGroupRow struct {
	ExceptionHash string       `lit:"exception_hash"`
	StackTrace    string       `lit:"stack_trace"`
	LastSeen      SQLiteTime   `lit:"last_seen"`
	FirstSeen     SQLiteTime   `lit:"first_seen"`
	Count         uint64       `lit:"count"`
	MaxArchivedAt sql.NullTime `lit:"max_archived_at"`
}

type exceptionTrendRow struct {
	Hash  string     `lit:"exception_hash"`
	Hour  SQLiteTime `lit:"hour"`
	Count uint64     `lit:"count"`
}

type exceptionBucketRow struct {
	Bucket SQLiteTime `lit:"bucket"`
	Value  float64    `lit:"agg_value"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModelWithNaming[exceptionRow](driver, exceptionRowNaming{})
		lit.RegisterModel[exceptionGroupRow](driver)
		lit.RegisterModel[exceptionTrendRow](driver)
		lit.RegisterModel[exceptionBucketRow](driver)
	})
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

func exceptionBucketsToPoints(rows []*exceptionBucketRow) []models.TimeSeriesPoint {
	points := make([]models.TimeSeriesPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, models.TimeSeriesPoint{Timestamp: row.Bucket.Time, Value: row.Value})
	}
	return points
}

type exceptionStackTraceRepository struct{}

func (e *exceptionStackTraceRepository) InsertAsync(ctx context.Context, lines []models.ExceptionStackTrace) error {
	if len(lines) == 0 {
		return nil
	}

	conn, err := db.DuckDBConnector.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	appender, err := duckdb.NewAppenderFromConn(conn, "", "exception_stack_traces")
	if err != nil {
		return err
	}

	for _, est := range lines {
		traceType := est.TraceType
		if traceType == "" {
			traceType = "endpoint"
		}

		attributesJSON := "{}"
		if len(est.Attributes) > 0 {
			b, err := json.Marshal(est.Attributes)
			if err != nil {
				captureDroppedRow("exception_stack_traces", err)
				continue
			}
			attributesJSON = string(b)
		}

		var traceId *string
		if est.TraceId != nil {
			v := est.TraceId.String()
			traceId = &v
		}
		var distributedTraceId *string
		if est.DistributedTraceId != nil {
			v := est.DistributedTraceId.String()
			distributedTraceId = &v
		}
		var sessionId *string
		if est.SessionId != nil {
			v := est.SessionId.String()
			sessionId = &v
		}

		isMessage := int64(0)
		if est.IsMessage {
			isMessage = 1
		}

		if err := appender.AppendRow(
			est.Id.String(),
			est.ProjectId.String(),
			nullableString(traceId),
			traceType,
			est.ExceptionHash,
			est.StackTrace,
			est.RecordedAt.UTC(),
			attributesJSON,
			est.AppVersion,
			est.ServerName,
			isMessage,
			nullableString(distributedTraceId),
			nullableString(sessionId),
		); err != nil {
			captureDroppedRow("exception_stack_traces", err)
		}
	}

	return appender.Close()
}

func (e *exceptionStackTraceRepository) CountBetween(ctx context.Context, projectId uuid.UUID, start, end time.Time) (int64, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		"SELECT COUNT(*) AS count FROM exception_stack_traces WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to",
		lit.P{"project_id": projectId, "from": start.UTC(), "to": end.UTC()})
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

	params := lit.P{"project_id": projectId, "from": fromDate.UTC(), "to": toDate.UTC()}

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
		GROUP BY e.exception_hash` + havingClause + `) AS sub`

	countResult, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB, countQuery, params)
	if err != nil {
		return nil, 0, err
	}
	count := int64(0)
	if countResult != nil {
		count = int64(countResult.Count)
	}

	fullQuery := `SELECT e.exception_hash, ANY_VALUE(e.stack_trace) as stack_trace, MAX(e.recorded_at) as last_seen, MIN(e.recorded_at) as first_seen, COUNT(*) as count, MAX(a.archived_at) as max_archived_at
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
		groups = append(groups, models.ExceptionGroup{
			ExceptionHash: g.ExceptionHash,
			StackTrace:    g.StackTrace,
			LastSeen:      g.LastSeen.Time,
			FirstSeen:     g.FirstSeen.Time,
			Count:         g.Count,
		})
	}

	return groups, count, nil
}

func (e *exceptionStackTraceRepository) FindByHash(ctx context.Context, projectId uuid.UUID, exceptionHash string, page, pageSize int) (*models.ExceptionGroup, []models.ExceptionStackTrace, int64, error) {
	offset := (page - 1) * pageSize
	params := lit.P{"project_id": projectId, "exception_hash": exceptionHash}

	groupRow, err := lit.SelectSingleNamed[exceptionGroupRow](db.TelemetryDB,
		`SELECT exception_hash, ANY_VALUE(stack_trace) as stack_trace, MAX(recorded_at) as last_seen, MIN(recorded_at) as first_seen, COUNT(*) as count, NULL as max_archived_at
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

	group := &models.ExceptionGroup{
		ExceptionHash: groupRow.ExceptionHash,
		StackTrace:    groupRow.StackTrace,
		LastSeen:      groupRow.LastSeen.Time,
		FirstSeen:     groupRow.FirstSeen.Time,
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
	results, err := lit.SelectNamed[exceptionBucketRow](db.TelemetryDB,
		`SELECT `+timeBucketExpr("recorded_at", 3600)+` as bucket, CAST(COUNT(*) AS DOUBLE) as agg_value
		FROM exception_stack_traces WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY bucket ORDER BY bucket ASC`,
		lit.P{"project_id": projectId, "from": start.UTC(), "to": end.UTC()})
	if err != nil {
		return nil, err
	}
	return exceptionBucketsToPoints(results), nil
}

func (e *exceptionStackTraceRepository) CountByInterval(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	results, err := lit.SelectNamed[exceptionBucketRow](db.TelemetryDB,
		`SELECT `+timeBucketExpr("recorded_at", intervalMinutes*60)+` as bucket, CAST(COUNT(*) AS DOUBLE) as agg_value
		FROM exception_stack_traces WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY bucket ORDER BY bucket ASC`,
		lit.P{"project_id": projectId, "from": start.UTC(), "to": end.UTC()})
	if err != nil {
		return nil, err
	}
	return exceptionBucketsToPoints(results), nil
}

func (e *exceptionStackTraceRepository) GetHourlyTrendForHashes(ctx context.Context, projectId uuid.UUID, hashes []string, start, end time.Time) (map[string][]models.ExceptionTrendPoint, error) {
	if len(hashes) == 0 {
		return make(map[string][]models.ExceptionTrendPoint), nil
	}

	params := lit.P{"project_id": projectId, "from": start.UTC(), "to": end.UTC()}
	placeholders := make([]string, len(hashes))
	for i, h := range hashes {
		key := fmt.Sprintf("hash_%d", i)
		placeholders[i] = ":" + key
		params[key] = h
	}

	query := `SELECT exception_hash, ` + timeBucketExpr("recorded_at", 3600) + ` as hour, COUNT(*) as count
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
		result[row.Hash] = append(result[row.Hash], models.ExceptionTrendPoint{
			Timestamp: row.Hour.Time,
			Count:     row.Count,
		})
	}

	return result, nil
}

func (e *exceptionStackTraceRepository) ArchiveByHashes(ctx context.Context, projectId uuid.UUID, hashes []string) error {
	if len(hashes) == 0 {
		return nil
	}

	now := time.Now().UTC()
	for _, hash := range hashes {
		query, args, err := lit.ParseNamedQuery(db.Driver,
			"INSERT INTO archived_exceptions (project_id, exception_hash, archived_at) VALUES (:project_id, :exception_hash, :archived_at) ON CONFLICT (project_id, exception_hash) DO UPDATE SET archived_at = excluded.archived_at",
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
		params["from"] = from.UTC()
		params["to"] = to.UTC()
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
		params["from"] = from.UTC()
		params["to"] = to.UTC()
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
		params["from"] = from.UTC()
		params["to"] = to.UTC()
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
