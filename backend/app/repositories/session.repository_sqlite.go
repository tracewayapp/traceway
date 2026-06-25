//go:build !pgch

package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type sessionRow struct {
	Id                 uuid.UUID     `lit:"id"`
	ProjectId          uuid.UUID     `lit:"project_id"`
	StartedAt          SQLiteTime    `lit:"started_at"`
	EndedAt            *SQLiteTime   `lit:"ended_at"`
	Duration           int64         `lit:"duration"`
	ClientIP           string        `lit:"client_ip"`
	Attributes         SQLiteJSONMap `lit:"attributes"`
	AppVersion         string        `lit:"app_version"`
	ServerName         string        `lit:"server_name"`
	DistributedTraceId *uuid.UUID    `lit:"distributed_trace_id"`
}

type sessionRowNaming struct{ lit.DefaultDbNamingStrategy }

func (sessionRowNaming) GetTableNameFromStructName(string) string {
	return "sessions"
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModelWithNaming[sessionRow](driver, sessionRowNaming{})
	})
}

func sessionToRow(s models.Session) sessionRow {
	row := sessionRow{
		Id:                 s.Id,
		ProjectId:          s.ProjectId,
		StartedAt:          NewSQLiteTime(s.StartedAt),
		Duration:           s.Duration,
		ClientIP:           s.ClientIP,
		Attributes:         NewSQLiteJSONMap(s.Attributes),
		AppVersion:         s.AppVersion,
		ServerName:         s.ServerName,
		DistributedTraceId: s.DistributedTraceId,
	}
	if s.EndedAt != nil {
		t := NewSQLiteTime(*s.EndedAt)
		row.EndedAt = &t
	}
	return row
}

func (row *sessionRow) toModel() models.Session {
	s := models.Session{
		Id:                 row.Id,
		ProjectId:          row.ProjectId,
		StartedAt:          row.StartedAt.Time,
		Duration:           row.Duration,
		ClientIP:           row.ClientIP,
		AppVersion:         row.AppVersion,
		ServerName:         row.ServerName,
		DistributedTraceId: row.DistributedTraceId,
	}
	if row.EndedAt != nil {
		t := row.EndedAt.Time
		s.EndedAt = &t
	}
	if row.Attributes != nil {
		s.Attributes = map[string]string(row.Attributes)
	}
	return s
}

type sessionRepository struct{}

func (r *sessionRepository) Upsert(ctx context.Context, sessions []models.Session) error {
	if len(sessions) == 0 {
		return nil
	}
	const stmt = `INSERT INTO sessions
		(id, project_id, started_at, ended_at, duration, client_ip, attributes, app_version, server_name, distributed_trace_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			ended_at = COALESCE(excluded.ended_at, sessions.ended_at),
			duration = CASE WHEN excluded.duration > 0 THEN excluded.duration ELSE sessions.duration END,
			attributes = excluded.attributes,
			app_version = excluded.app_version,
			server_name = excluded.server_name,
			distributed_trace_id = excluded.distributed_trace_id`

	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, s := range sessions {
		row := sessionToRow(s)
		attrs, err := row.Attributes.Value()
		if err != nil {
			return err
		}
		var endedAt interface{}
		if row.EndedAt != nil {
			v, err := row.EndedAt.Value()
			if err != nil {
				return err
			}
			endedAt = v
		}
		startedAt, err := row.StartedAt.Value()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, stmt,
			row.Id, row.ProjectId, startedAt, endedAt, row.Duration,
			row.ClientIP, attrs, row.AppVersion, row.ServerName, row.DistributedTraceId,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *sessionRepository) CountBetween(ctx context.Context, projectId uuid.UUID, start, end time.Time) (int64, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		"SELECT count(*) as count FROM sessions WHERE project_id = :project_id AND started_at >= :start AND started_at <= :end",
		lit.P{"project_id": projectId, "start": NewSQLiteTime(start), "end": NewSQLiteTime(end)})
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return int64(result.Count), nil
}

func (r *sessionRepository) FindAll(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy string, sortDirection string, search string, attributeFilters []SessionAttributeFilter) ([]models.Session, int64, error) {
	whereExtra, extraParams := buildSessionFilterClauseSQLite(search, attributeFilters)

	countQuery := "SELECT count(*) as count FROM sessions WHERE project_id = :project_id AND started_at >= :start AND started_at <= :end" + whereExtra
	countParams := lit.P{
		"project_id": projectId,
		"start":      NewSQLiteTime(fromDate),
		"end":        NewSQLiteTime(toDate),
	}
	for k, v := range extraParams {
		countParams[k] = v
	}
	countResult, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB, countQuery, countParams)
	if err != nil {
		return nil, 0, err
	}
	var count int64
	if countResult != nil {
		count = int64(countResult.Count)
	}

	allowedOrderBy := map[string]bool{
		"started_at": true,
		"duration":   true,
	}
	if !allowedOrderBy[orderBy] {
		orderBy = "started_at"
	}
	sortDir := "DESC"
	if sortDirection == "asc" {
		sortDir = "ASC"
	}

	offset := (page - 1) * pageSize
	query := "SELECT id, project_id, started_at, ended_at, duration, client_ip, attributes, app_version, server_name, distributed_trace_id FROM sessions WHERE project_id = :project_id AND started_at >= :start AND started_at <= :end" + whereExtra + " ORDER BY " + orderBy + " " + sortDir + " LIMIT :limit OFFSET :offset"

	queryParams := lit.P{
		"project_id": projectId,
		"start":      NewSQLiteTime(fromDate),
		"end":        NewSQLiteTime(toDate),
		"limit":      pageSize,
		"offset":     offset,
	}
	for k, v := range extraParams {
		queryParams[k] = v
	}
	rows, err := lit.SelectNamed[sessionRow](db.TelemetryDB, query, queryParams)
	if err != nil {
		return nil, 0, err
	}

	sessions := make([]models.Session, 0, len(rows))
	for _, row := range rows {
		sessions = append(sessions, row.toModel())
	}
	return sessions, count, nil
}

// buildSessionFilterClauseSQLite produces a WHERE fragment + named params
// mirroring the ClickHouse helper: search must be a valid UUID for an exact
// id match (anything else is ignored), and each attribute filter becomes an
// exact match against `json_extract(attributes, '$.<key>')`.
func buildSessionFilterClauseSQLite(search string, filters []SessionAttributeFilter) (string, lit.P) {
	var sb strings.Builder
	params := lit.P{}
	if s := strings.TrimSpace(search); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			sb.WriteString(" AND id = :search_id")
			params["search_id"] = id
		}
	}
	for i, f := range filters {
		if f.Key == "" {
			continue
		}
		keyParam := fmt.Sprintf("attr_k_%d", i)
		valParam := fmt.Sprintf("attr_v_%d", i)
		sb.WriteString(" AND json_extract(attributes, '$.\"' || :")
		sb.WriteString(keyParam)
		sb.WriteString(" || '\"') = :")
		sb.WriteString(valParam)
		params[keyParam] = f.Key
		params[valParam] = f.Value
	}
	return sb.String(), params
}

func (r *sessionRepository) FindById(ctx context.Context, projectId, sessionId uuid.UUID, startedAt *time.Time) (*models.Session, error) {
	query := "SELECT id, project_id, started_at, ended_at, duration, client_ip, attributes, app_version, server_name, distributed_trace_id FROM sessions WHERE project_id = :project_id AND id = :id"
	params := lit.P{"project_id": projectId, "id": sessionId}
	if startedAt != nil {
		from, to := traceWindowBounds(*startedAt)
		query += " AND started_at >= :from AND started_at <= :to"
		params["from"] = NewSQLiteTime(from)
		params["to"] = NewSQLiteTime(to)
	}
	query += " LIMIT 1"

	row, err := lit.SelectSingleNamed[sessionRow](db.TelemetryDB, query, params)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	s := row.toModel()
	return &s, nil
}

var SessionRepository = sessionRepository{}
