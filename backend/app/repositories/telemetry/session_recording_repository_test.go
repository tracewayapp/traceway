package telemetry

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestSessionRecordingRepository_InsertAndFind(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	exceptionId := uuid.New()
	now := truncateMs(time.Now().UTC())

	rec := makeSessionRecording(projectId, exceptionId, "/recordings/session-1.json", now)

	err := SessionRecordingRepository.InsertAsync(ctx, []models.SessionRecording{rec})
	if err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	filePath, err := SessionRecordingRepository.FindByExceptionId(ctx, projectId, exceptionId)
	if err != nil {
		t.Fatalf("FindByExceptionId failed: %v", err)
	}

	if filePath != "/recordings/session-1.json" {
		t.Errorf("expected file path '/recordings/session-1.json', got %q", filePath)
	}
}

func TestSessionRecordingRepository_FindByExceptionId_NotFound(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	_, err := SessionRecordingRepository.FindByExceptionId(ctx, uuid.New(), uuid.New())
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows for unknown exception, got %v", err)
	}
}

func TestSessionRecordingRepository_ReturnsLatest(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	exceptionId := uuid.New()
	now := truncateMs(time.Now().UTC())

	rec1 := makeSessionRecording(projectId, exceptionId, "/recordings/old.json", now.Add(-time.Hour))
	rec2 := makeSessionRecording(projectId, exceptionId, "/recordings/new.json", now)

	err := SessionRecordingRepository.InsertAsync(ctx, []models.SessionRecording{rec1, rec2})
	if err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	filePath, err := SessionRecordingRepository.FindByExceptionId(ctx, projectId, exceptionId)
	if err != nil {
		t.Fatalf("FindByExceptionId failed: %v", err)
	}

	if filePath != "/recordings/new.json" {
		t.Errorf("expected latest recording '/recordings/new.json', got %q", filePath)
	}
}

func TestSessionRecordingRepository_InsertEmpty(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	err := SessionRecordingRepository.InsertAsync(ctx, []models.SessionRecording{})
	if err != nil {
		t.Fatalf("InsertAsync with empty slice should not error: %v", err)
	}
}
