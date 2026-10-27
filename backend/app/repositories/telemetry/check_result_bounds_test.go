package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

func TestCheckResultRepository_CountsResultsInTheFirstSecondOfARange(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project := uuid.New()
	from := time.Now().UTC().Truncate(time.Minute).Add(-10 * time.Minute)
	to := from.Add(time.Minute)
	for i, offset := range []time.Duration{500 * time.Millisecond, 30 * time.Second} {
		at := from.Add(offset)
		if err := CheckResultRepository.Insert(ctx, shared.CheckResult{ProjectId: project, CheckId: 3, RunId: i + 1, CheckType: "http", RecordedAt: at, ExecutedAt: at, Status: "down"}); err != nil {
			t.Fatal(err)
		}
	}

	points, err := CheckResultRepository.SeriesByCheck(ctx, project, 3, from, to, 1)
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	for _, point := range points {
		total += int64(point.Total)
	}
	if total != 2 {
		t.Fatalf("series counted %d results, want 2: %+v", total, points)
	}

	aggregates, err := CheckResultRepository.AggregatesByProject(ctx, project, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if aggregates[3] == nil || aggregates[3].Total != 2 {
		t.Fatalf("aggregates: %+v", aggregates[3])
	}

	entries, count, err := CheckResultRepository.FindByCheck(ctx, project, 3, &from, &to, "", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || len(entries) != 2 {
		t.Fatalf("history listed %d of %d results, want 2", len(entries), count)
	}
}
