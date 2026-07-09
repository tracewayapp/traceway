package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFiredNotificationRepository_Insert(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	n := makeFiredNotification(projectId, "High Error Rate", "sent", now)

	err := FiredNotificationRepository.Insert(ctx, n)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
}

func TestFiredNotificationRepository_InsertMultiple(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	for i := 0; i < 5; i++ {
		n := makeFiredNotification(projectId, "Alert Rule", "sent", now.Add(time.Duration(i)*time.Minute))
		if err := FiredNotificationRepository.Insert(ctx, n); err != nil {
			t.Fatalf("Insert %d failed: %v", i, err)
		}
	}
}
