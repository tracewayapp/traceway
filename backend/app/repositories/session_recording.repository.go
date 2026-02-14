package repositories

import (
	"backend/app/chdb"
	"backend/app/models"
	"context"
	"database/sql"
	"errors"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
)

type sessionRecordingRepository struct{}

func (r *sessionRecordingRepository) InsertAsync(ctx context.Context, recordings []models.SessionRecording) error {
	if len(recordings) == 0 {
		return nil
	}
	batch, err := (*chdb.Conn).PrepareBatch(clickhouse.Context(context.Background(), clickhouse.WithAsync(false)), "INSERT INTO session_recordings (id, project_id, exception_id, file_path, recorded_at)")
	if err != nil {
		return err
	}
	for _, rec := range recordings {
		if err := batch.Append(rec.Id, rec.ProjectId, rec.ExceptionId, rec.FilePath, rec.RecordedAt); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (r *sessionRecordingRepository) FindByExceptionId(ctx context.Context, projectId uuid.UUID, exceptionId uuid.UUID) (string, error) {
	var filePath string
	err := (*chdb.Conn).QueryRow(ctx,
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

var SessionRecordingRepository = sessionRecordingRepository{}
