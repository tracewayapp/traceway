//go:build telemetry_duckdb

package duckdb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type sessionRecording struct {
	Id           uuid.UUID              `lit:"id"`
	ProjectId    uuid.UUID              `lit:"project_id"`
	ExceptionId  uuid.UUID              `lit:"exception_id"`
	SessionId    *uuid.UUID             `lit:"session_id"`
	SegmentIndex int32                  `lit:"segment_index"`
	FilePath     string                 `lit:"file_path"`
	RecordedAt   sqlitetypes.SQLiteTime `lit:"recorded_at"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[sessionRecording](driver)
	})
}

type sessionRecordingRepository struct{}

func (r *sessionRecordingRepository) InsertAsync(ctx context.Context, recordings []models.SessionRecording) error {
	if len(recordings) == 0 {
		return nil
	}

	// Unlike the other telemetry tables, dropped rows must surface as an error:
	// the recordings uploader counts an InsertAsync failure in its failed metric,
	// and a silently missing row means an orphaned blob in storage.
	dropped := 0
	err := withAppender(ctx, "session_recordings", func(appender *duckdb.Appender) {
		for _, rec := range recordings {
			var sessionId *string
			if rec.SessionId != nil {
				s := rec.SessionId.String()
				sessionId = &s
			}
			// DDL column order: id, project_id, exception_id, file_path, recorded_at, session_id, segment_index
			if err := appender.AppendRow(
				rec.Id.String(),
				rec.ProjectId.String(),
				rec.ExceptionId.String(),
				rec.FilePath,
				rec.RecordedAt.UTC(),
				nullableString(sessionId),
				int64(rec.SegmentIndex),
			); err != nil {
				captureDroppedRow("session_recordings", err)
				dropped++
			}
		}
	})
	if err != nil {
		return err
	}
	if dropped > 0 {
		return fmt.Errorf("session_recordings insert: dropped %d of %d rows", dropped, len(recordings))
	}
	return nil
}

func (r *sessionRecordingRepository) FindByExceptionId(ctx context.Context, projectId uuid.UUID, exceptionId uuid.UUID) (string, error) {
	result, err := lit.SelectSingleNamed[sqlitetypes.FilePathResult](db.TelemetryDB,
		"SELECT file_path FROM session_recordings WHERE project_id = :project_id AND exception_id = :exception_id ORDER BY recorded_at DESC LIMIT 1",
		lit.P{"project_id": projectId, "exception_id": exceptionId})
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", sql.ErrNoRows
	}
	return result.FilePath, nil
}

// FindBySessionId returns all recording segments for a session ordered by segment_index.
func (r *sessionRecordingRepository) FindBySessionId(ctx context.Context, projectId, sessionId uuid.UUID) ([]models.SessionRecording, error) {
	rows, err := lit.SelectNamed[sessionRecording](db.TelemetryDB,
		"SELECT id, project_id, exception_id, session_id, segment_index, file_path, recorded_at FROM session_recordings WHERE project_id = :project_id AND session_id = :session_id ORDER BY segment_index ASC, recorded_at ASC",
		lit.P{"project_id": projectId, "session_id": sessionId})
	if err != nil {
		return nil, err
	}

	out := make([]models.SessionRecording, 0, len(rows))
	for _, row := range rows {
		out = append(out, models.SessionRecording{
			Id:           row.Id,
			ProjectId:    row.ProjectId,
			ExceptionId:  row.ExceptionId,
			SessionId:    row.SessionId,
			SegmentIndex: row.SegmentIndex,
			FilePath:     row.FilePath,
			RecordedAt:   row.RecordedAt.Time,
		})
	}
	return out, nil
}

var SessionRecordingRepository = &sessionRecordingRepository{}
