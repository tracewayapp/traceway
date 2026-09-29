//go:build telemetry_ch

package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	return chdb.SendBatch("INSERT INTO sessions (id, project_id, started_at, ended_at, duration, client_ip, attributes, app_version, server_name, trace_id, version)", func(batch driver.Batch) error {
		for _, s := range sessions {
			attrs := s.Attributes
			if attrs == nil {
				attrs = map[string]string{}
			}
			if err := batch.Append(s.Id, s.ProjectId, s.StartedAt, s.EndedAt, s.Duration, s.ClientIP, attrs, s.AppVersion, s.ServerName, s.TraceId, time.Now()); err != nil {
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

	orderExpr := "s.started_at"
	if orderBy == "duration" {
		orderExpr = sessionDurationSortKey
	}
	sortDir := "DESC"
	if sortDirection == "asc" {
		sortDir = "ASC"
	}

	offset := (page - 1) * pageSize

	recFrom, recTo := shared.SessionRecordingWindow(fromDate, toDate)
	query := "SELECT s.id, s.project_id, s.started_at, s.ended_at, s.duration, s.client_ip, s.attributes, s.app_version, s.server_name, s.trace_id, a.last_activity, a.last_received" +
		" FROM (SELECT id, project_id, started_at, ended_at, duration, client_ip, attributes, app_version, server_name, trace_id FROM sessions FINAL WHERE project_id = ? AND started_at >= ? AND started_at <= ?" + whereExtra + ") AS s" +
		" LEFT JOIN (" + sessionActivityQuery + ") AS a ON a.sid = s.id" +
		" ORDER BY " + orderExpr + " " + sortDir + " LIMIT ? OFFSET ?" +
		" SETTINGS join_use_nulls = 1"
	queryArgs := []interface{}{projectId, fromDate, toDate}
	queryArgs = append(queryArgs, extraArgs...)
	queryArgs = append(queryArgs, projectId, recFrom, recTo)
	queryArgs = append(queryArgs, pageSize, offset)
	rows, err := chdb.Conn.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	now := time.Now()
	var sessions []models.Session
	for rows.Next() {
		var s models.Session
		var activity shared.SessionActivity
		if err := rows.Scan(&s.Id, &s.ProjectId, &s.StartedAt, &s.EndedAt, &s.Duration, &s.ClientIP, &s.Attributes, &s.AppVersion, &s.ServerName, &s.TraceId, &activity.LastActivity, &activity.LastReceived); err != nil {
			return nil, 0, err
		}
		shared.ResolveSessionEnd(&s, activity, now)
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return sessions, int64(count), nil
}

const sessionActivityQuery = `SELECT assumeNotNull(session_id) AS sid,
		max(coalesce(ended_at, toDateTime64(recorded_at, 3))) AS last_activity,
		max(recorded_at) AS last_received
	FROM session_recordings
	WHERE project_id = ? AND session_id IS NOT NULL AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY sid`

var sessionDurationSortKey = fmt.Sprintf(`multiIf(a.last_activity IS NULL, least(intDiv(s.duration, 1000000), %[2]d),
	assumeNotNull(a.last_activity) > toDateTime64(s.started_at, 3) + INTERVAL %[3]d MINUTE
		AND s.ended_at > s.started_at AND s.ended_at <= s.started_at + INTERVAL %[3]d MINUTE,
	toInt64(dateDiff('millisecond', toDateTime64(s.started_at, 3), toDateTime64(assumeNotNull(s.ended_at), 3))),
	least(toInt64(dateDiff('millisecond', toDateTime64(s.started_at, 3), if(
		s.ended_at > greatest(assumeNotNull(a.last_activity), toDateTime64(s.started_at, 3))
			AND s.ended_at < greatest(assumeNotNull(a.last_activity), toDateTime64(s.started_at, 3)) + INTERVAL %[1]d MINUTE,
		toDateTime64(assumeNotNull(s.ended_at), 3),
		greatest(assumeNotNull(a.last_activity), toDateTime64(s.started_at, 3))))), %[2]d))`,
	int(shared.SessionIdleTimeout/time.Minute), shared.SessionMaxSpan.Milliseconds(), int(shared.SessionMaxSpan/time.Minute))

func (r *sessionRepository) FindById(ctx context.Context, projectId, sessionId uuid.UUID, startedAt *time.Time) (*models.Session, error) {
	var s models.Session

	query := `SELECT id, project_id, started_at, ended_at, duration, client_ip, attributes, app_version, server_name, trace_id
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
		&s.Id, &s.ProjectId, &s.StartedAt, &s.EndedAt, &s.Duration, &s.ClientIP, &s.Attributes, &s.AppVersion, &s.ServerName, &s.TraceId,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	from, to := shared.TraceWindowBounds(s.StartedAt)
	var segments uint64
	var lastActivity, lastReceived time.Time
	err = chdb.Conn.QueryRow(ctx,
		`SELECT count(), max(coalesce(ended_at, toDateTime64(recorded_at, 3))), max(recorded_at)
			FROM session_recordings
			WHERE project_id = ? AND session_id = ? AND recorded_at >= ? AND recorded_at <= ?`,
		projectId, sessionId, from, to).Scan(&segments, &lastActivity, &lastReceived)
	if err != nil {
		return nil, err
	}
	var activity shared.SessionActivity
	if segments > 0 {
		activity = shared.SessionActivity{LastActivity: &lastActivity, LastReceived: &lastReceived}
	}
	shared.ResolveSessionEnd(&s, activity, time.Now())
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
