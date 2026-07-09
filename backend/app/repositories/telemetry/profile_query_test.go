//go:build !telemetry_ch

package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/profiling"
)

func mustInsertStacks(t *testing.T, ctx context.Context, stacks ...models.ProfileStack) {
	t.Helper()
	if err := ProfileRepository.InsertStacksAsync(ctx, stacks); err != nil {
		t.Fatalf("InsertStacksAsync: %v", err)
	}
}

func mustInsertSamples(t *testing.T, ctx context.Context, samples ...models.ProfileSample) {
	t.Helper()
	if err := ProfileRepository.InsertSamplesAsync(ctx, samples); err != nil {
		t.Fatalf("InsertSamplesAsync: %v", err)
	}
}

func mustInsertProfiles(t *testing.T, ctx context.Context, profiles ...models.Profile) {
	t.Helper()
	if err := ProfileRepository.InsertProfilesAsync(ctx, profiles); err != nil {
		t.Fatalf("InsertProfilesAsync: %v", err)
	}
}

func flameValueByStack(rows []models.ProfileStackValue) map[string]int64 {
	out := map[string]int64{}
	for _, r := range rows {
		key := ""
		for i, f := range r.Stack {
			if i > 0 {
				key += ";"
			}
			key += f
		}
		out[key] = r.Value
	}
	return out
}

func TestProfileRepository_FindGroupedByService(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	from := base.Add(-time.Hour)
	to := base.Add(time.Hour)

	mustInsertProfiles(t, ctx,
		models.Profile{Id: uuid.New(), ProjectId: projectId, RecordedAt: base, ServiceName: "checkout", ProfileType: "cpu", SampleCount: 2, TotalValue: 300},
		models.Profile{Id: uuid.New(), ProjectId: projectId, RecordedAt: base.Add(time.Minute), ServiceName: "checkout", ProfileType: "cpu", SampleCount: 1, TotalValue: 200},
		models.Profile{Id: uuid.New(), ProjectId: projectId, RecordedAt: base, ServiceName: "checkout", ProfileType: "inuse_space", SampleCount: 3, TotalValue: 9000},
	)

	groups, total, err := ProfileRepository.FindGroupedByService(ctx, projectId, from, to, 1, 50, "total_value", "desc", "")
	if err != nil {
		t.Fatalf("FindGroupedByService: %v", err)
	}
	if total != 2 {
		t.Errorf("distinct (service,type) groups = %d, want 2", total)
	}

	byType := map[string]models.ProfileGroup{}
	for _, g := range groups {
		if g.ServiceName != "checkout" {
			t.Errorf("unexpected service %q", g.ServiceName)
		}
		byType[g.Type] = g
	}
	cpu := byType["cpu"]
	if cpu.ProfileCount != 2 || cpu.TotalValue != 500 || cpu.SampleCount != 3 {
		t.Errorf("cpu group = %+v, want ProfileCount=2 TotalValue=500 SampleCount=3", cpu)
	}
	inuse := byType["inuse_space"]
	if inuse.ProfileCount != 1 || inuse.TotalValue != 9000 {
		t.Errorf("inuse group = %+v, want ProfileCount=1 TotalValue=9000", inuse)
	}
}

func TestProfileRepository_GetFlameGraph_CounterMergesInstancesAndFiltersLabels(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	from := base.Add(-time.Hour)
	to := base.Add(time.Hour)

	hashAB := profiling.HashFrames([]string{"a", "b"})
	hashAC := profiling.HashFrames([]string{"a", "c"})
	mustInsertStacks(t, ctx,
		models.ProfileStack{ProjectId: projectId, ServiceName: "checkout", StackHash: hashAB, Stack: []string{"a", "b"}, LastSeen: base},
		models.ProfileStack{ProjectId: projectId, ServiceName: "checkout", StackHash: hashAC, Stack: []string{"a", "c"}, LastSeen: base},
	)

	cpu := func(server string, hash uint64, value int64, env string) models.ProfileSample {
		return models.ProfileSample{
			ProjectId: projectId, ProfileId: uuid.New(), ServiceName: "checkout",
			Type: "cpu", Start: base, End: base.Add(30 * time.Second),
			StackHash: hash, Value: value, ServerName: server, Labels: map[string]string{"env": env},
		}
	}
	mustInsertSamples(t, ctx,
		cpu("pod-a", hashAB, 200, "prod"),
		cpu("pod-b", hashAB, 300, "prod"),
		cpu("pod-a", hashAC, 100, "dev"),
	)

	all, err := ProfileRepository.GetFlameGraph(ctx, projectId, "checkout", "cpu", from, to, nil, false, "", "")
	if err != nil {
		t.Fatalf("GetFlameGraph: %v", err)
	}
	got := flameValueByStack(all)
	if got["a;b"] != 500 {
		t.Errorf("a;b merged value = %d, want 500 (pod-a 200 + pod-b 300)", got["a;b"])
	}
	if got["a;c"] != 100 {
		t.Errorf("a;c value = %d, want 100", got["a;c"])
	}

	prod, err := ProfileRepository.GetFlameGraph(ctx, projectId, "checkout", "cpu", from, to, map[string]string{"env": "prod"}, false, "", "")
	if err != nil {
		t.Fatalf("GetFlameGraph (filtered): %v", err)
	}
	gotProd := flameValueByStack(prod)
	if gotProd["a;b"] != 500 {
		t.Errorf("filtered a;b = %d, want 500", gotProd["a;b"])
	}
	if _, ok := gotProd["a;c"]; ok {
		t.Errorf("env=prod filter must exclude the dev-only a;c stack, got %v", gotProd)
	}
}

func TestProfileRepository_GetFlameGraph_FiltersByDottedLabelKey(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	from := base.Add(-time.Hour)
	to := base.Add(time.Hour)

	hashAB := profiling.HashFrames([]string{"a", "b"})
	hashAC := profiling.HashFrames([]string{"a", "c"})
	mustInsertStacks(t, ctx,
		models.ProfileStack{ProjectId: projectId, ServiceName: "checkout", StackHash: hashAB, Stack: []string{"a", "b"}, LastSeen: base},
		models.ProfileStack{ProjectId: projectId, ServiceName: "checkout", StackHash: hashAC, Stack: []string{"a", "c"}, LastSeen: base},
	)
	cpu := func(hash uint64, value int64, route string) models.ProfileSample {
		return models.ProfileSample{
			ProjectId: projectId, ProfileId: uuid.New(), ServiceName: "checkout",
			Type: "cpu", Start: base, End: base.Add(30 * time.Second),
			StackHash: hash, Value: value, Labels: map[string]string{"http.route": route},
		}
	}
	mustInsertSamples(t, ctx,
		cpu(hashAB, 200, "GET /checkout"),
		cpu(hashAC, 100, "GET /cart"),
	)

	rows, err := ProfileRepository.GetFlameGraph(ctx, projectId, "checkout", "cpu", from, to, map[string]string{"http.route": "GET /checkout"}, false, "", "")
	if err != nil {
		t.Fatalf("GetFlameGraph (dotted filter): %v", err)
	}
	got := flameValueByStack(rows)
	if got["a;b"] != 200 {
		t.Errorf("a;b = %d, want 200 (dotted-key filter must match in SQLite mode)", got["a;b"])
	}
	if _, ok := got["a;c"]; ok {
		t.Errorf("http.route=GET /checkout must exclude the a;c stack, got %v", got)
	}
}

func TestProfileRepository_GetFlameGraph_GaugeUsesLatestPerServer(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	from := base.Add(-time.Hour)
	to := base.Add(time.Hour)

	hashX := profiling.HashFrames([]string{"a", "b"})
	mustInsertStacks(t, ctx,
		models.ProfileStack{ProjectId: projectId, ServiceName: "checkout", StackHash: hashX, Stack: []string{"a", "b"}, LastSeen: base},
	)

	inuse := func(server string, start time.Time, value int64) models.ProfileSample {
		return models.ProfileSample{
			ProjectId: projectId, ProfileId: uuid.New(), ServiceName: "checkout",
			Type: "inuse_space", Start: start, End: start,
			StackHash: hashX, Value: value, ServerName: server,
		}
	}
	t1 := base.Add(-30 * time.Minute)
	t2 := base
	mustInsertSamples(t, ctx,
		inuse("pod-a", t1, 100),
		inuse("pod-a", t2, 300),
		inuse("pod-b", t1, 50),
	)

	rows, err := ProfileRepository.GetFlameGraph(ctx, projectId, "checkout", "inuse_space", from, to, nil, true, "", "")
	if err != nil {
		t.Fatalf("GetFlameGraph (gauge): %v", err)
	}
	got := flameValueByStack(rows)
	if got["a;b"] != 350 {
		t.Errorf("gauge a;b = %d, want 350 (pod-a latest 300 + pod-b 50, NOT 450)", got["a;b"])
	}
}

func TestProfileRepository_GetFlameGraph_GaugeDedupsTiedStartTimes(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	from := base.Add(-time.Hour)
	to := base.Add(time.Hour)

	hashX := profiling.HashFrames([]string{"a", "b"})
	mustInsertStacks(t, ctx,
		models.ProfileStack{ProjectId: projectId, ServiceName: "checkout", StackHash: hashX, Stack: []string{"a", "b"}, LastSeen: base},
	)

	inuse := func(server string, start time.Time, value int64) models.ProfileSample {
		return models.ProfileSample{
			ProjectId: projectId, ProfileId: uuid.New(), ServiceName: "checkout",
			Type: "inuse_space", Start: start, End: start,
			StackHash: hashX, Value: value, ServerName: server,
		}
	}
	mustInsertSamples(t, ctx,
		inuse("pod-a", base, 300),
		inuse("pod-a", base, 300),
		inuse("pod-b", base, 50),
	)

	rows, err := ProfileRepository.GetFlameGraph(ctx, projectId, "checkout", "inuse_space", from, to, nil, true, "", "")
	if err != nil {
		t.Fatalf("GetFlameGraph (gauge ties): %v", err)
	}
	got := flameValueByStack(rows)
	if got["a;b"] != 350 {
		t.Errorf("gauge a;b = %d, want 350 (one pod-a snapshot 300 + pod-b 50, tied start_times must not both count)", got["a;b"])
	}
}

func TestProfileRepository_GetSeries_CounterSumsPerBucket(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	h12 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	h13 := time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC)
	from := h12.Add(-time.Minute)
	to := h13.Add(time.Hour)

	hashX := profiling.HashFrames([]string{"a"})
	sample := func(start time.Time, value int64) models.ProfileSample {
		return models.ProfileSample{
			ProjectId: projectId, ProfileId: uuid.New(), ServiceName: "checkout",
			Type: "cpu", Start: start, End: start, StackHash: hashX, Value: value,
		}
	}
	mustInsertSamples(t, ctx,
		sample(h12.Add(5*time.Minute), 100),
		sample(h12.Add(20*time.Minute), 200),
		sample(h13.Add(10*time.Minute), 700),
	)

	points, err := ProfileRepository.GetSeries(ctx, projectId, "checkout", "cpu", from, to, 60, false, nil, "", "", false)
	if err != nil {
		t.Fatalf("GetSeries: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 hourly buckets, got %d (%+v)", len(points), points)
	}
	if points[0].Value != 300 {
		t.Errorf("bucket 12:00 = %v, want 300 (100+200)", points[0].Value)
	}
	if points[1].Value != 700 {
		t.Errorf("bucket 13:00 = %v, want 700", points[1].Value)
	}
}
