//go:build !telemetry_ch

package telemetry

import (
	"context"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
)

func reasonQuery(ctx context.Context, query string, args ...any) (reasonRows, error) {
	return db.TelemetryDB.QueryContext(ctx, query, args...)
}
func reasonInsert(ctx context.Context, args ...any) error {
	_, err := db.TelemetryDB.ExecContext(ctx, "INSERT INTO session_recording_reasons (project_id, session_id, trigger_id, revision, name, sequence, fired_at) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (project_id, trigger_id, session_id, revision) DO UPDATE SET fired_at = CASE WHEN excluded.fired_at < session_recording_reasons.fired_at THEN excluded.fired_at ELSE session_recording_reasons.fired_at END", args...)
	return err
}
