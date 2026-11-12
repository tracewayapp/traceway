//go:build telemetry_ch

package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"slices"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/chdb"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

type sessionRepository struct{}

// sessionEndVersionLead ranks every row that records an end above every row that does not in the table's
// ReplacingMergeTree(version). An update without an end that loses a race with session.end (it looked for an end
// before that insert landed) must not become the surviving row with ended_at NULL. A newer end still wins over an
// older one, and a century keeps the two ranges apart for the table's lifetime.
const sessionEndVersionLead = 100 * 365 * 24 * time.Hour

// Upsert keeps an end once recorded, as the COALESCE of the SQLite and DuckDB upserts does: rows arriving without
// one take the end the table already holds, so a later snapshot of the attributes still wins.
func (r *sessionRepository) Upsert(ctx context.Context, sessions []models.Session) error {
	if len(sessions) == 0 {
		return nil
	}
	sessions, err := withRecordedEnds(ctx, sessions)
	if err != nil {
		return err
	}
	return chdb.SendBatch(ctx, "INSERT INTO sessions (id, project_id, started_at, ended_at, duration, client_ip, attributes, app_version, server_name, trace_id, version)", func(batch driver.Batch) error {
		for _, s := range sessions {
			attrs := s.Attributes
			if attrs == nil {
				attrs = map[string]string{}
			}
			version := time.Now()
			if s.EndedAt != nil {
				version = version.Add(sessionEndVersionLead)
			}
			if err := batch.Append(s.Id, s.ProjectId, s.StartedAt, s.EndedAt, s.Duration, s.ClientIP, attrs, s.AppVersion, s.ServerName, s.TraceId, version); err != nil {
				return err
			}
		}
		return nil
	})
}

type sessionRowKey struct {
	id      uuid.UUID
	started int64
}

type recordedEnd struct {
	endedAt  time.Time
	duration int64
}

// withRecordedEnds returns the sessions with the stored end filled in where one arrives without it. A row is the
// (project_id, started_at, id) of the sorting key; started_at is stored in whole seconds.
func withRecordedEnds(ctx context.Context, sessions []models.Session) ([]models.Session, error) {
	open := map[uuid.UUID][]int{}
	for i, s := range sessions {
		if s.EndedAt == nil {
			open[s.ProjectId] = append(open[s.ProjectId], i)
		}
	}
	if len(open) == 0 {
		return sessions, nil
	}
	filled := slices.Clone(sessions)
	for projectId, indexes := range open {
		ids := make([]uuid.UUID, 0, len(indexes))
		starts := make([]time.Time, 0, len(indexes))
		for _, i := range indexes {
			ids = append(ids, filled[i].Id)
			starts = append(starts, filled[i].StartedAt)
		}
		rows, err := chdb.Conn.Query(ctx, `SELECT id, started_at, argMax(assumeNotNull(ended_at), version), argMax(duration, version)
			FROM sessions
			WHERE project_id = ? AND started_at >= ? AND started_at <= ? AND id IN (?) AND ended_at IS NOT NULL
			GROUP BY id, started_at`,
			projectId, slices.MinFunc(starts, time.Time.Compare).Truncate(time.Second), slices.MaxFunc(starts, time.Time.Compare), ids)
		if err != nil {
			return nil, err
		}
		ends := map[sessionRowKey]recordedEnd{}
		for rows.Next() {
			var id uuid.UUID
			var started time.Time
			var end recordedEnd
			if err := rows.Scan(&id, &started, &end.endedAt, &end.duration); err != nil {
				rows.Close()
				return nil, err
			}
			ends[sessionRowKey{id, started.Unix()}] = end
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		for _, i := range indexes {
			if end, ok := ends[sessionRowKey{filled[i].Id, filled[i].StartedAt.Unix()}]; ok {
				endedAt := end.endedAt
				filled[i].EndedAt, filled[i].Duration = &endedAt, end.duration
			}
		}
	}
	return filled, nil
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
	query := "SELECT s.id, s.project_id, s.started_at, s.ended_at, s.duration, s.client_ip, s.attributes, s.app_version, s.server_name, s.trace_id, a.last_activity, a.span_activity, a.last_received" +
		" FROM (SELECT id, project_id, started_at, ended_at, duration, client_ip, attributes, app_version, server_name, trace_id FROM sessions FINAL WHERE project_id = ? AND started_at >= ? AND started_at <= ?" + whereExtra + ") AS s" +
		" LEFT JOIN (" + sessionActivityQuery + ") AS a ON a.sid = s.id" +
		" ORDER BY " + orderExpr + " " + sortDir + " LIMIT ? OFFSET ?" +
		" SETTINGS join_use_nulls = 1"
	queryArgs := []interface{}{projectId, fromDate, toDate}
	queryArgs = append(queryArgs, extraArgs...)
	queryArgs = append(queryArgs, projectId, fromDate, toDate, projectId, recFrom, recTo)
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
		if err := rows.Scan(&s.Id, &s.ProjectId, &s.StartedAt, &s.EndedAt, &s.Duration, &s.ClientIP, &s.Attributes, &s.AppVersion, &s.ServerName, &s.TraceId, &activity.LastActivity, &activity.LastActivityInSpan, &activity.LastReceived); err != nil {
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

var sessionActivityQuery = fmt.Sprintf(`SELECT assumeNotNull(r.session_id) AS sid,
		max(coalesce(r.ended_at, toDateTime64(r.recorded_at, 3))) AS last_activity,
		maxIfOrNull(coalesce(r.ended_at, toDateTime64(r.recorded_at, 3)),
			coalesce(r.ended_at, toDateTime64(r.recorded_at, 3)) <= toDateTime64(ss.started_at, 3) + INTERVAL %d MINUTE) AS span_activity,
		max(r.recorded_at) AS last_received
	FROM session_recordings AS r
	INNER JOIN (SELECT id, started_at FROM sessions FINAL WHERE project_id = ? AND started_at >= ? AND started_at <= ?) AS ss ON ss.id = assumeNotNull(r.session_id)
	WHERE r.project_id = ? AND r.session_id IS NOT NULL AND r.recorded_at >= ? AND r.recorded_at <= ?
	GROUP BY sid`, int(shared.SessionMaxSpan/time.Minute))

var sessionLastActivity = fmt.Sprintf(`if(assumeNotNull(a.last_activity) > toDateTime64(s.started_at, 3) + INTERVAL %d MINUTE,
	assumeNotNull(coalesce(a.span_activity, toDateTime64(s.started_at, 3))), assumeNotNull(a.last_activity))`, int(shared.SessionMaxSpan/time.Minute))

var sessionDurationSortKey = strings.ReplaceAll(fmt.Sprintf(`if(a.last_activity IS NULL, least(intDiv(s.duration, 1000000), %[2]d),
	least(toInt64(dateDiff('millisecond', toDateTime64(s.started_at, 3), if(
		s.ended_at > greatest({last}, toDateTime64(s.started_at, 3))
			AND s.ended_at < greatest({last}, toDateTime64(s.started_at, 3)) + INTERVAL %[1]d MINUTE,
		toDateTime64(assumeNotNull(s.ended_at), 3),
		greatest({last}, toDateTime64(s.started_at, 3))))), %[2]d))`,
	int(shared.SessionIdleTimeout/time.Minute), shared.SessionMaxSpan.Milliseconds()), "{last}", sessionLastActivity)

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
	var spanActivity *time.Time
	err = chdb.Conn.QueryRow(ctx,
		`SELECT count(), max(coalesce(ended_at, toDateTime64(recorded_at, 3))),
				maxIfOrNull(coalesce(ended_at, toDateTime64(recorded_at, 3)), coalesce(ended_at, toDateTime64(recorded_at, 3)) <= ?),
				max(recorded_at)
			FROM session_recordings
			WHERE project_id = ? AND session_id = ? AND recorded_at >= ? AND recorded_at <= ?`,
		s.StartedAt.Add(shared.SessionMaxSpan), projectId, sessionId, from, to).Scan(&segments, &lastActivity, &spanActivity, &lastReceived)
	if err != nil {
		return nil, err
	}
	var activity shared.SessionActivity
	if segments > 0 {
		activity = shared.SessionActivity{LastActivity: &lastActivity, LastActivityInSpan: spanActivity, LastReceived: &lastReceived}
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
