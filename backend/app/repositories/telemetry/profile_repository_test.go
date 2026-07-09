//go:build !telemetry_ch

package telemetry

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestProfileRepository_InsertAndReadBack(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	profileId := uuid.New()
	now := time.Now().UTC().Truncate(time.Millisecond)

	hash := uint64(0xFFFFFFFFFFFFFFF1)
	frames := []string{"main.main", "main.work"}

	stack := models.ProfileStack{
		ProjectId: projectId, ServiceName: "checkout",
		StackHash: hash, Stack: frames, LastSeen: now,
	}
	if err := ProfileRepository.InsertStacksAsync(ctx, []models.ProfileStack{stack}); err != nil {
		t.Fatalf("InsertStacksAsync: %v", err)
	}
	stack.LastSeen = now.Add(time.Minute)
	if err := ProfileRepository.InsertStacksAsync(ctx, []models.ProfileStack{stack}); err != nil {
		t.Fatalf("InsertStacksAsync (dedup): %v", err)
	}

	sample := models.ProfileSample{
		ProjectId: projectId, ProfileId: profileId, ServiceName: "checkout",
		Type: "cpu", Start: now, End: now.Add(30 * time.Second),
		StackHash: hash, Value: 300, ServerName: "pod-a", AppVersion: "1.2.3",
	}
	if err := ProfileRepository.InsertSamplesAsync(ctx, []models.ProfileSample{sample}); err != nil {
		t.Fatalf("InsertSamplesAsync: %v", err)
	}

	prof := models.Profile{
		Id: profileId, ProjectId: projectId, RecordedAt: now, Duration: 30 * time.Second,
		ServiceName: "checkout", ProfileType: "cpu",
		SampleCount: 1, TotalValue: 300, ServerName: "pod-a", AppVersion: "1.2.3",
	}
	if err := ProfileRepository.InsertProfilesAsync(ctx, []models.Profile{prof}); err != nil {
		t.Fatalf("InsertProfilesAsync: %v", err)
	}

	var stackCount int
	if err := db.TelemetryDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM profiling_stacks").Scan(&stackCount); err != nil {
		t.Fatalf("count stacks: %v", err)
	}
	if stackCount != 1 {
		t.Errorf("expected 1 deduped stack, got %d", stackCount)
	}

	var gotHash int64
	var gotStackJSON string
	if err := db.TelemetryDB.QueryRowContext(ctx,
		"SELECT stack_hash, stack FROM profiling_stacks LIMIT 1").Scan(&gotHash, &gotStackJSON); err != nil {
		t.Fatalf("read stack: %v", err)
	}
	if uint64(gotHash) != hash {
		t.Errorf("stack_hash round-trip = %d, want %d", uint64(gotHash), hash)
	}
	var gotFrames []string
	if err := json.Unmarshal([]byte(gotStackJSON), &gotFrames); err != nil {
		t.Fatalf("decode stack json: %v", err)
	}
	if len(gotFrames) != 2 || gotFrames[0] != "main.main" || gotFrames[1] != "main.work" {
		t.Errorf("frames = %v, want [main.main main.work]", gotFrames)
	}

	var gotType string
	var gotValue, gotSampleHash int64
	if err := db.TelemetryDB.QueryRowContext(ctx,
		"SELECT type, value, stack_hash FROM profiling_samples LIMIT 1").Scan(&gotType, &gotValue, &gotSampleHash); err != nil {
		t.Fatalf("read sample: %v", err)
	}
	if gotType != "cpu" || gotValue != 300 || uint64(gotSampleHash) != hash {
		t.Errorf("sample = (%s,%d,%d), want (%s,300,%d)", gotType, gotValue, uint64(gotSampleHash), "cpu", hash)
	}

	var gotTotal int64
	var gotProfileType string
	if err := db.TelemetryDB.QueryRowContext(ctx,
		"SELECT profile_type, total_value FROM profiles WHERE id = ?", profileId.String()).Scan(&gotProfileType, &gotTotal); err != nil {
		t.Fatalf("read profile: %v", err)
	}
	if gotProfileType != "cpu" || gotTotal != 300 {
		t.Errorf("profile = (%s,%d), want (%s,300)", gotProfileType, gotTotal, "cpu")
	}
}
