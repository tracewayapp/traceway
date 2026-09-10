//go:build telemetry_ch

package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type sessionRepository struct{}

func (r *sessionRepository) Upsert(ctx context.Context, sessions []models.Session) error {
	if len(sessions) == 0 {
		return nil
	}
	return chdb.SendBatch("INSERT INTO sessions (id, project_id, started_at, ended_at, duration, client_ip, attributes, app_version, server_name, distributed_trace_id, version)", func(batch driver.Batch) error {
		for _, s := range sessions {
			attrs := s.Attributes
			if attrs == nil {
				attrs = map[string]string{}
			}
			if err := batch.Append(s.Id, s.ProjectId, s.StartedAt, s.EndedAt, s.Duration, s.ClientIP, attrs, s.AppVersion, s.ServerName, s.DistributedTraceId, time.Now()); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *sessionRepository) CountBetween(ctx context.Context, projectId uuid.UUID, start, end time.Time) (int64, error) {
	var count uint64
	err := chdb.Conn.QueryRow(ctx,
		"SELECT count() FROM sessions FINAL WHERE project_id = ? AND started_at >= ? AND started_at <= ?",
		projectId, start, end).Scan(&count)
	return int64(count), err
}

func (r *sessionRepository) FindAll(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy string, sortDirection string, search string, attributeFilters []shared.SessionAttributeFilter) ([]models.Session, int64, error) {
	whereExtra, extraArgs := buildSessionFilterClause(search, attributeFilters)

	countArgs := []interface{}{projectId, fromDate, toDate}
	countArgs = append(countArgs, extraArgs...)
	var count uint64
	countQuery := "SELECT count() FROM sessions FINAL WHERE project_id = ? AND started_at >= ? AND started_at <= ?" + whereExtra
	if err := chdb.Conn.QueryRow(ctx, countQuery, countArgs...).Scan(&count); err != nil {
		return nil, 0, err
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

	query := "SELECT id, project_id, started_at, ended_at, duration, client_ip, attributes, app_version, server_name, distributed_trace_id FROM sessions FINAL WHERE project_id = ? AND started_at >= ? AND started_at <= ?" + whereExtra + " ORDER BY " + orderBy + " " + sortDir + " LIMIT ? OFFSET ?"
	queryArgs := []interface{}{projectId, fromDate, toDate}
	queryArgs = append(queryArgs, extraArgs...)
	queryArgs = append(queryArgs, pageSize, offset)
	rows, err := chdb.Conn.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var sessions []models.Session
	for rows.Next() {
		var s models.Session
		if err := rows.Scan(&s.Id, &s.ProjectId, &s.StartedAt, &s.EndedAt, &s.Duration, &s.ClientIP, &s.Attributes, &s.AppVersion, &s.ServerName, &s.DistributedTraceId); err != nil {
			return nil, 0, err
		}
		sessions = append(sessions, s)
	}
	return sessions, int64(count), nil
}

func (r *sessionRepository) FindById(ctx context.Context, projectId, sessionId uuid.UUID, startedAt *time.Time) (*models.Session, error) {
	var s models.Session

	query := `SELECT id, project_id, started_at, ended_at, duration, client_ip, attributes, app_version, server_name, distributed_trace_id
		FROM sessions FINAL
		WHERE project_id = ? AND id = ?`
	args := []any{projectId, sessionId}
	if startedAt != nil {
		from, to := shared.TraceWindowBounds(*startedAt)
		query += ` AND started_at >= ? AND started_at <= ?`
		args = append(args, from, to)
	}
	query += ` LIMIT 1`

	err := chdb.Conn.QueryRow(ctx, query, args...).Scan(
		&s.Id, &s.ProjectId, &s.StartedAt, &s.EndedAt, &s.Duration, &s.ClientIP, &s.Attributes, &s.AppVersion, &s.ServerName, &s.DistributedTraceId,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func buildSessionFilterClause(search string, filters []shared.SessionAttributeFilter) (string, []interface{}) {
	var sb strings.Builder
	args := []interface{}{}
	if s := strings.TrimSpace(search); s != "" {
		sb.WriteString(" AND (positionCaseInsensitiveUTF8(toString(id), ?) > 0 OR positionCaseInsensitiveUTF8(client_ip, ?) > 0 OR arrayExists(v -> positionCaseInsensitiveUTF8(v, ?) > 0, mapValues(attributes)))")
		args = append(args, s, s, s)
	}
	for _, f := range filters {
		if f.Key == "" {
			continue
		}
		sb.WriteString(" AND ")
		if f.Exclude {
			sb.WriteString("NOT ")
		}
		sb.WriteString("(mapContains(attributes, ?) AND ")
		if f.Contains {
			sb.WriteString("positionCaseInsensitiveUTF8(attributes[?], ?) > 0)")
		} else {
			sb.WriteString("attributes[?] = ?)")
		}
		args = append(args, f.Key, f.Key, f.Value)
	}
	return sb.String(), args
}

var SessionRepository = &sessionRepository{}
