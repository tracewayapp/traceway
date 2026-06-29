//go:build duckdb && !pgch

package repositories

import (
	"context"
	"database/sql"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type sessionRecording struct {
	Id           uuid.UUID  `lit:"id"`
	ProjectId    uuid.UUID  `lit:"project_id"`
	ExceptionId  uuid.UUID  `lit:"exception_id"`
	SessionId    *uuid.UUID `lit:"session_id"`
	SegmentIndex int32      `lit:"segment_index"`
	FilePath     string     `lit:"file_path"`
	RecordedAt   SQLiteTime `lit:"recorded_at"`
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

	conn, err := db.DuckDBConnector.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	appender, err := duckdb.NewAppenderFromConn(conn, "", "session_recordings")
	if err != nil {
		return err
	}

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
			appender.Close()
			return err
		}
	}

	return appender.Close()
}

func (r *sessionRecordingRepository) FindByExceptionId(ctx context.Context, projectId uuid.UUID, exceptionId uuid.UUID) (string, error) {
	result, err := lit.SelectSingleNamed[filePathResult](db.TelemetryDB,
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

var SessionRecordingRepository = sessionRecordingRepository{}
