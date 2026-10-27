//go:build telemetry_duckdb

package duckdb

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

func (r *sessionRepository) SLICounts(ctx context.Context, projectId uuid.UUID, spec shared.SessionSLISpec, from, to time.Time, bucketMinutes int) ([]shared.SLOBucketPoint, error) {
	pid := projectId.String()
	failed := "id IN (SELECT session_id FROM exceptions_v2 WHERE project_id = ? AND session_id IS NOT NULL AND session_id != '' AND recorded_at >= ? AND recorded_at <= ?"
	if !spec.IncludeMessages {
		failed += " AND is_message = 0"
	}
	failed += ")"
	args := []any{pid, from.UTC(), to.Add(shared.SessionExceptionSlack).UTC(), pid, from.UTC(), to.UTC()}
	where := ""
	if spec.AppVersion != "" {
		where = " AND app_version = ?"
		args = append(args, spec.AppVersion)
	}
	query := "SELECT " + epochBucket("started_at", bucketMinutes*60) + " AS bucket, CAST(COUNT(*) AS BIGINT), " + sumIf("NOT ("+failed+")") +
		" FROM sessions WHERE project_id = ? AND started_at >= ? AND started_at <= ?" + where + " GROUP BY bucket ORDER BY bucket ASC"
	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return shared.ScanSLOBucketPoints(rows, from, to)
}
