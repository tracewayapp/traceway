//go:build telemetry_ch

package telemetry

import (
	"context"
	"github.com/tracewayapp/traceway/backend/v2/app/chdb"
)

func reasonQuery(ctx context.Context, query string, args ...any) (reasonRows, error) {
	return chdb.Conn.Query(ctx, query, args...)
}
func reasonInsert(ctx context.Context, args ...any) error {
	return chdb.Conn.Exec(ctx, "INSERT INTO session_recording_reasons (project_id, session_id, trigger_id, revision, name, sequence, fired_at) VALUES (?, ?, ?, ?, ?, ?, ?)", args...)
}
