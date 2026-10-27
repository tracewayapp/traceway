//go:build telemetry_ch

package clickhouse

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

func (r *sessionRepository) SLICounts(ctx context.Context, projectId uuid.UUID, spec shared.SessionSLISpec, from, to time.Time, bucketMinutes int) ([]shared.SLOBucketPoint, error) {
	failed := "id IN (SELECT assumeNotNull(session_id) FROM exceptions_v2 WHERE project_id = ? AND session_id IS NOT NULL AND recorded_at >= ? AND recorded_at <= ?"
	if !spec.IncludeMessages {
		failed += " AND is_message = 0"
	}
	failed += ")"
	args := []any{projectId, from.UTC(), to.Add(shared.SessionExceptionSlack).UTC(), projectId, from.UTC(), to.UTC()}
	where := ""
	if spec.AppVersion != "" {
		where = " AND app_version = ?"
		args = append(args, spec.AppVersion)
	}
	query := "SELECT " + epochBucket("started_at", bucketMinutes*60) + " AS bucket, toInt64(count()), " + countIf("NOT ("+failed+")") +
		" FROM sessions FINAL WHERE project_id = ? AND started_at >= ? AND started_at <= ?" + where + " GROUP BY bucket ORDER BY bucket ASC"
	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return shared.ScanSLOBucketPoints(rows, from, to)
}
