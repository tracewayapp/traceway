package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestProfileInsertAfterIdentityCleanup(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	profile := models.Profile{
		Id: uuid.New(), ProjectId: uuid.New(), RecordedAt: now,
		ServiceName: "trace-identity", ProfileType: "cpu", Unit: "nanoseconds",
		SampleCount: 1, TotalValue: 123, TraceId: "0102030405060708090a0b0c0d0e0f10", SpanId: "0102030405060708",
	}
	if err := ProfileRepository.InsertProfilesAsync(ctx, []models.Profile{profile}); err != nil {
		t.Fatal(err)
	}
	groups, total, err := ProfileRepository.FindGroupedByService(ctx, profile.ProjectId, now.Add(-time.Second), now.Add(time.Second), 1, 10, "profile_count", "desc", "")
	if err != nil || total != 1 || len(groups) != 1 || groups[0].ServiceName != profile.ServiceName || groups[0].ProfileCount != 1 {
		t.Fatalf("profile was not stored after obsolete column removal: %+v total=%d: %v", groups, total, err)
	}
}
