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

// buildSessionFilterClause assembles the search + attribute-filter portion of
// the WHERE clause for sessions queries. Empty inputs return ("", nil).
// Search restricts to a single session id (only valid UUIDs match — any other
// search input falls through and matches nothing useful, which is the right
// behaviour given attribute filters now cover the structured-search use case).
// Each attribute filter adds an exact-match clause via the `attributes[?]`
// Map subscript, which is O(1) per row and indexed by the bloom filters on
// `mapKeys(attributes)` / `mapValues(attributes)`.
func buildSessionFilterClause(search string, filters []shared.SessionAttributeFilter) (string, []interface{}) {
	var sb strings.Builder
	args := []interface{}{}
	if s := strings.TrimSpace(search); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			sb.WriteString(" AND id = ?")
			args = append(args, id)
		}
	}
	for _, f := range filters {
		if f.Key == "" {
			continue
		}
		// `attributes[?]` does an O(1) lookup on the Map column. Combined
		// with the bloom_filter on mapKeys/mapValues this is index-driven.
		sb.WriteString(" AND attributes[?] = ?")
		args = append(args, f.Key, f.Value)
	}
	return sb.String(), args
}

var SessionRepository = &sessionRepository{}
