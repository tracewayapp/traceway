//go:build telemetry_ch

package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
)

type sessionRecordingRepository struct{}

func (r *sessionRecordingRepository) InsertAsync(ctx context.Context, recordings []models.SessionRecording) error {
	if len(recordings) == 0 {
		return nil
	}
	batch, err := chdb.Conn.PrepareBatch(chdb.BatchCtx(), "INSERT INTO session_recordings (id, project_id, exception_id, session_id, segment_index, file_path, recorded_at)")
	if err != nil {
		return err
	}
	for _, rec := range recordings {
		if err := batch.Append(rec.Id, rec.ProjectId, rec.ExceptionId, rec.SessionId, rec.SegmentIndex, rec.FilePath, rec.RecordedAt); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (r *sessionRecordingRepository) FindByExceptionId(ctx context.Context, projectId uuid.UUID, exceptionId uuid.UUID) (string, error) {
	var filePath string
	err := chdb.Conn.QueryRow(ctx,
		"SELECT file_path FROM session_recordings WHERE project_id = ? AND exception_id = ? ORDER BY recorded_at DESC LIMIT 1",
		projectId, exceptionId).Scan(&filePath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", sql.ErrNoRows
		}
		return "", err
	}
	return filePath, nil
}

// FindBySessionId returns all recording segments for a session ordered by segment_index.
func (r *sessionRecordingRepository) FindBySessionId(ctx context.Context, projectId, sessionId uuid.UUID) ([]models.SessionRecording, error) {
	rows, err := chdb.Conn.Query(ctx,
		"SELECT id, project_id, exception_id, session_id, segment_index, file_path, recorded_at FROM session_recordings WHERE project_id = ? AND session_id = ? ORDER BY segment_index ASC, recorded_at ASC",
		projectId, sessionId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recordings []models.SessionRecording
	for rows.Next() {
		var rec models.SessionRecording
		if err := rows.Scan(&rec.Id, &rec.ProjectId, &rec.ExceptionId, &rec.SessionId, &rec.SegmentIndex, &rec.FilePath, &rec.RecordedAt); err != nil {
			return nil, err
		}
		recordings = append(recordings, rec)
	}
	return recordings, nil
}

var SessionRecordingRepository = &sessionRecordingRepository{}
