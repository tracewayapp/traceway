package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestAiTraceRepository_GetTraceNameStats(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	traces := []models.AiTrace{
		makeAiTrace(projectId, "summarize", 100*time.Millisecond, 100, 0.5, now),
		makeAiTrace(projectId, "summarize", 200*time.Millisecond, 200, 1.0, now.Add(time.Minute)),
		makeAiTrace(projectId, "summarize", 300*time.Millisecond, 300, 1.5, now.Add(2*time.Minute)),
	}

	if err := AiTraceRepository.InsertAsync(ctx, traces); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)
	stats, err := AiTraceRepository.GetTraceNameStats(ctx, projectId, "summarize", start, end)
	if err != nil {
		t.Fatalf("GetTraceNameStats failed: %v", err)
	}

	if stats.Count != 3 {
		t.Errorf("expected count 3, got %d", stats.Count)
	}
	if stats.TotalTokens != 600 {
		t.Errorf("expected total tokens 600, got %d", stats.TotalTokens)
	}
	assertApproxEqual(t, "TotalCost", stats.TotalCost, 3.0, 0.001)
	assertApproxEqual(t, "AvgDuration", stats.AvgDuration, 200.0, 1.0)
	assertApproxEqual(t, "MedianDuration", stats.MedianDuration, 200.0, 1.0)
	assertApproxEqual(t, "AvgInputTokens", stats.AvgInputTokens, 100.0, 0.001)
}

func TestAiTraceRepository_GetTraceNameStats_EmptyWindow(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	stats, err := AiTraceRepository.GetTraceNameStats(ctx, uuid.New(), "missing", now.Add(-time.Hour), now)
	if err != nil {
		t.Fatalf("GetTraceNameStats failed: %v", err)
	}
	if stats.Count != 0 {
		t.Errorf("expected count 0, got %d", stats.Count)
	}
	if stats.TotalTokens != 0 {
		t.Errorf("expected total tokens 0, got %d", stats.TotalTokens)
	}
}
