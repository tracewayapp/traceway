//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package slo

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

func setupSLODB(t *testing.T) uuid.UUID {
	t.Helper()
	dbtest.SetupSQLite(t)
	windowCaches = map[int]*windowCache{}
	project, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		return transactional.ProjectRepository.Create(tx, "slo-test", "gin")
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return project.Id
}

func createTestSLO(t *testing.T, projectId uuid.UUID, kind, config string, target float64, windowDays, bucketMinutes int) *models.SLO {
	t.Helper()
	now := time.Now().UTC()
	entity := &models.SLO{
		ProjectId: projectId, Name: "test", SliKind: kind, SliConfig: models.JSONText(config),
		TargetPercent: target, WindowDays: windowDays, BucketMinutes: bucketMinutes, Enabled: true,
		Status: models.SLOStatusNoData, CreatedAt: now, UpdatedAt: now,
	}
	id, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.SLORepository.Create(tx, entity)
	})
	if err != nil {
		t.Fatalf("create slo: %v", err)
	}
	entity.Id = id
	return entity
}

func reloadSLO(t *testing.T, id int) *models.SLO {
	t.Helper()
	entity, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.SLO, error) {
		return transactional.SLORepository.FindById(tx, id)
	})
	if err != nil || entity == nil {
		t.Fatalf("reload slo %d: %v", id, err)
	}
	return entity
}

func endpointAt(projectId uuid.UUID, status int16, at time.Time) models.Endpoint {
	return models.Endpoint{Id: uuid.New(), ProjectId: projectId, Endpoint: "GET /users", Duration: 50 * time.Millisecond, RecordedAt: at, StatusCode: status, ClientIP: "127.0.0.1"}
}

func TestWorkerComputesEndpointAvailability(t *testing.T) {
	projectId := setupSLODB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Minute).Add(30 * time.Second)
	var endpoints []models.Endpoint
	for minute := 1; minute <= 20; minute++ {
		at := now.Add(-time.Duration(minute) * time.Minute)
		for i := 0; i < 9; i++ {
			endpoints = append(endpoints, endpointAt(projectId, 200, at.Add(time.Duration(i)*time.Second)))
		}
		endpoints = append(endpoints, endpointAt(projectId, 500, at.Add(9*time.Second)))
	}
	if err := telemetry.EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatal(err)
	}
	entity := createTestSLO(t, projectId, models.SLIKindEndpoint, `{"endpoint":"","good":"availability"}`, 99, 7, 1)

	RunOnce(ctx, now)

	got := reloadSLO(t, entity.Id)
	if got.Status != models.SLOStatusBreached {
		t.Fatalf("90%% availability against a 99%% target is breached, got %q", got.Status)
	}
	if got.CurrentSli == nil || *got.CurrentSli < 89.9 || *got.CurrentSli > 90.1 {
		t.Fatalf("current sli: %v", got.CurrentSli)
	}
	if got.BudgetRemaining == nil || *got.BudgetRemaining > -800 {
		t.Fatalf("budget is overspent tenfold: %v", got.BudgetRemaining)
	}
	lastClosed := now.Truncate(time.Minute)
	if got.LastBucketAt == nil || !got.LastBucketAt.Equal(lastClosed) {
		t.Fatalf("last bucket = %v, want %v", got.LastBucketAt, lastClosed)
	}
	windowStart := lastClosed.Add(-7 * 24 * time.Hour)
	if got.HistoryFrom == nil || got.HistoryFrom.After(windowStart) {
		t.Fatalf("backfill of an empty week should finish in one tick: history from %v", got.HistoryFrom)
	}
	good, total, err := telemetry.SLOBucketRepository.SumRange(ctx, projectId, entity.Id, windowStart, lastClosed)
	if err != nil || good != 180 || total != 200 {
		t.Fatalf("window sums = %d/%d (%v), want 180/200", good, total, err)
	}

	late := endpointAt(projectId, 200, now.Add(-3*time.Minute).Add(20*time.Second))
	if err := telemetry.EndpointRepository.InsertAsync(ctx, []models.Endpoint{late}); err != nil {
		t.Fatal(err)
	}
	RunOnce(ctx, now.Add(time.Minute))
	RunOnce(ctx, now.Add(2*time.Minute))
	good, total, err = telemetry.SLOBucketRepository.SumRange(ctx, projectId, entity.Id, windowStart, lastClosed.Add(2*time.Minute))
	if err != nil || good != 181 || total != 201 {
		t.Fatalf("window sums after late data = %d/%d (%v), want 181/201", good, total, err)
	}
}

func TestWorkerMetricTimeslices(t *testing.T) {
	projectId := setupSLODB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(5 * time.Minute).Add(time.Minute)
	lastClosed := now.Truncate(5 * time.Minute)
	points := []models.MetricPoint{
		{ProjectId: projectId, Name: "cpu.used_pcnt", Value: 40, Tags: map[string]string{}, RecordedAt: lastClosed.Add(-5 * time.Minute).Add(10 * time.Second)},
		{ProjectId: projectId, Name: "cpu.used_pcnt", Value: 95, Tags: map[string]string{}, RecordedAt: lastClosed.Add(-10 * time.Minute).Add(10 * time.Second)},
		{ProjectId: projectId, Name: "cpu.used_pcnt", Value: 10, Tags: map[string]string{}, RecordedAt: lastClosed.Add(-15 * time.Minute).Add(10 * time.Second)},
	}
	if err := telemetry.MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatal(err)
	}
	strict := createTestSLO(t, projectId, models.SLIKindMetric, `{"metricName":"cpu.used_pcnt","tags":{},"aggregation":"avg","operator":"lt","threshold":90,"missingIsBad":true}`, 99, 7, 5)
	lenient := createTestSLO(t, projectId, models.SLIKindMetric, `{"metricName":"cpu.used_pcnt","tags":{},"aggregation":"avg","operator":"lt","threshold":90,"missingIsBad":false}`, 50, 7, 5)
	since := lastClosed.Add(-30 * time.Minute)
	strict.SliSince = &since
	setSLISince(t, strict.Id, since)

	RunOnce(ctx, now)

	windowStart := lastClosed.Add(-7 * 24 * time.Hour)
	good, total, err := telemetry.SLOBucketRepository.SumRange(ctx, projectId, strict.Id, windowStart, lastClosed)
	if err != nil || total != 6 || good != 2 {
		t.Fatalf("empty slices are bad only since the definition took effect: %d/%d (%v), want 2/6", good, total, err)
	}
	good, total, err = telemetry.SLOBucketRepository.SumRange(ctx, projectId, lenient.Id, windowStart, lastClosed)
	if err != nil || total != 3 || good != 2 {
		t.Fatalf("empty slices do not count: %d/%d (%v), want 2/3", good, total, err)
	}
	if got := reloadSLO(t, lenient.Id); got.Status != models.SLOStatusOk {
		t.Fatalf("2 of 3 slices good against a 50%% target is ok, got %q", got.Status)
	}
}

func TestWorkerMonitorUptime(t *testing.T) {
	projectId := setupSLODB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Minute).Add(30 * time.Second)
	at := now.Add(-5 * time.Minute)
	for i, status := range []string{"up", "up", "down", "missed", "up"} {
		result := shared.CheckResult{ProjectId: projectId, CheckId: 3, RunId: i + 1, CheckType: "http", RecordedAt: at.Add(time.Duration(i) * 10 * time.Second), ExecutedAt: at, Status: status, LatencyMs: 12}
		if err := telemetry.CheckResultRepository.Insert(ctx, result); err != nil {
			t.Fatal(err)
		}
	}
	foreign := shared.CheckResult{ProjectId: projectId, CheckId: 4, RunId: 9, CheckType: "http", RecordedAt: at, ExecutedAt: at, Status: "down"}
	if err := telemetry.CheckResultRepository.Insert(ctx, foreign); err != nil {
		t.Fatal(err)
	}
	entity := createTestSLO(t, projectId, models.SLIKindMonitor, `{"checkIds":[3]}`, 90, 7, 1)

	RunOnce(ctx, now)

	got := reloadSLO(t, entity.Id)
	if got.CurrentSli == nil || *got.CurrentSli != 75 {
		t.Fatalf("3 up of 4 measured probes is 75%%: %v", got.CurrentSli)
	}
	if got.Status != models.SLOStatusBreached {
		t.Fatalf("75%% against 90%% is breached: %q", got.Status)
	}
}

func setSLISince(t *testing.T, id int, since time.Time) {
	t.Helper()
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		_, err := tx.Exec("UPDATE slos SET sli_since = ? WHERE id = ?", since.UTC(), id)
		return struct{}{}, err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWorkerDiscardsBucketsOfAnEditDuringTheTick(t *testing.T) {
	projectId := setupSLODB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Minute).Add(30 * time.Second)
	var endpoints []models.Endpoint
	for minute := 1; minute <= 120; minute++ {
		at := now.Add(-time.Duration(minute) * time.Minute)
		bad := endpointAt(projectId, 500, at)
		bad.Endpoint = "GET /old"
		good := endpointAt(projectId, 200, at)
		good.Endpoint = "GET /new"
		endpoints = append(endpoints, bad, good)
	}
	if err := telemetry.EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatal(err)
	}
	entity := createTestSLO(t, projectId, models.SLIKindEndpoint, `{"endpoint":"GET /old","good":"availability"}`, 99, 7, 1)
	loadedByTheTick := reloadSLO(t, entity.Id)

	edited := reloadSLO(t, entity.Id)
	edited.SliConfig = models.JSONText(`{"endpoint":"GET /new","good":"availability"}`)
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		if err := transactional.SLORepository.Update(tx, edited); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, transactional.SLORepository.ResetHistory(tx, edited.Id, now)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := telemetry.SLOBucketRepository.DeleteBySLO(ctx, projectId, entity.Id); err != nil {
		t.Fatal(err)
	}

	computeAll(ctx, []*models.SLO{loadedByTheTick}, now)
	if got := reloadSLO(t, entity.Id); got.LastBucketAt != nil {
		t.Fatalf("the stale tick must not write its snapshot over the reset: %v", got.LastBucketAt)
	}
	RunOnce(ctx, now.Add(time.Minute))

	got := reloadSLO(t, entity.Id)
	if got.CurrentSli == nil || *got.CurrentSli != 100 {
		t.Fatalf("the new definition only sees 200s: %v", got.CurrentSli)
	}
}

func TestWorkerKeepsBucketsWhenOnlyTheTargetChangedDuringTheTick(t *testing.T) {
	projectId := setupSLODB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Minute).Add(30 * time.Second)
	if err := telemetry.EndpointRepository.InsertAsync(ctx, []models.Endpoint{endpointAt(projectId, 200, now.Add(-2*time.Minute))}); err != nil {
		t.Fatal(err)
	}
	entity := createTestSLO(t, projectId, models.SLIKindEndpoint, `{"endpoint":"","good":"availability"}`, 99, 7, 1)
	loadedByTheTick := reloadSLO(t, entity.Id)
	edited := reloadSLO(t, entity.Id)
	edited.TargetPercent = 95
	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, transactional.SLORepository.Update(tx, edited)
	}); err != nil {
		t.Fatal(err)
	}

	computeAll(ctx, []*models.SLO{loadedByTheTick}, now)
	_, total, err := telemetry.SLOBucketRepository.SumRange(ctx, projectId, entity.Id, now.Add(-time.Hour), now)
	if err != nil || total != 1 {
		t.Fatalf("buckets of an unchanged definition stay: %d (%v)", total, err)
	}
}

func TestWorkerFillsTheGapOfAnOutage(t *testing.T) {
	projectId := setupSLODB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Minute).Add(30 * time.Second)
	var endpoints []models.Endpoint
	for hour := 1; hour <= 96; hour++ {
		endpoints = append(endpoints, endpointAt(projectId, 500, now.Add(-time.Duration(hour)*time.Hour)))
	}
	if err := telemetry.EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatal(err)
	}
	entity := createTestSLO(t, projectId, models.SLIKindEndpoint, `{"endpoint":"","good":"availability"}`, 99, 7, 1)
	computeAll(ctx, []*models.SLO{entity}, now.Add(-72*time.Hour))

	RunOnce(ctx, now)

	got := reloadSLO(t, entity.Id)
	if got.GapFrom != nil || got.GapTo != nil {
		t.Fatalf("the gap is filled within one budget: %v %v", got.GapFrom, got.GapTo)
	}
	window, err := WindowSums(ctx, got, 7*24*time.Hour, now)
	if err != nil || window.Total != 96 {
		t.Fatalf("every request of the window counts after an outage: %d (%v)", window.Total, err)
	}
}

func TestWorkerCountsASessionCrashThatArrivesLate(t *testing.T) {
	projectId := setupSLODB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Minute).Add(30 * time.Second)
	start := now.Add(-2 * time.Minute)
	sessions := make([]models.Session, 0, 10)
	for i := 0; i < 10; i++ {
		sessions = append(sessions, models.Session{Id: uuid.New(), ProjectId: projectId, StartedAt: start.Add(time.Duration(i) * time.Second), Attributes: map[string]string{}})
	}
	if err := telemetry.SessionRepository.Upsert(ctx, sessions); err != nil {
		t.Fatal(err)
	}
	entity := createTestSLO(t, projectId, models.SLIKindSession, `{}`, 99, 7, 1)
	for minute := 0; minute < 30; minute++ {
		RunOnce(ctx, now.Add(time.Duration(minute)*time.Minute))
	}
	crash := models.ExceptionStackTrace{Id: uuid.New(), ProjectId: projectId, ExceptionHash: "crash", StackTrace: "boom", RecordedAt: now.Add(30 * time.Minute), SessionId: &sessions[0].Id}
	if err := telemetry.ExceptionStackTraceRepository.InsertAsync(ctx, []models.ExceptionStackTrace{crash}); err != nil {
		t.Fatal(err)
	}
	for minute := 30; minute < 30+int(lateArrivalInterval/time.Minute); minute++ {
		RunOnce(ctx, now.Add(time.Duration(minute)*time.Minute))
	}

	got := reloadSLO(t, entity.Id)
	if got.CurrentSli == nil || *got.CurrentSli != 90 {
		t.Fatalf("a crash 30 minutes into a session must still count: %v", derefSLI(got.CurrentSli))
	}
}

func TestWorkerCountsAMobileCrashUploadedTheNextDay(t *testing.T) {
	projectId := setupSLODB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Minute).Add(-48 * time.Hour).Add(30 * time.Second)
	start := now.Add(-2 * time.Minute)
	sessions := make([]models.Session, 0, 10)
	for i := 0; i < 10; i++ {
		sessions = append(sessions, models.Session{Id: uuid.New(), ProjectId: projectId, StartedAt: start.Add(time.Duration(i) * time.Second), Attributes: map[string]string{}})
	}
	if err := telemetry.SessionRepository.Upsert(ctx, sessions); err != nil {
		t.Fatal(err)
	}
	entity := createTestSLO(t, projectId, models.SLIKindSession, `{}`, 99, 7, 1)
	RunOnce(ctx, now)
	RunOnce(ctx, now.Add(time.Hour))

	crash := models.ExceptionStackTrace{Id: uuid.New(), ProjectId: projectId, ExceptionHash: "crash", StackTrace: "SIGSEGV", RecordedAt: start.Add(40 * time.Second), SessionId: &sessions[0].Id}
	if err := telemetry.ExceptionStackTraceRepository.InsertAsync(ctx, []models.ExceptionStackTrace{crash}); err != nil {
		t.Fatal(err)
	}
	RunOnce(ctx, now.Add(25*time.Hour))

	got := reloadSLO(t, entity.Id)
	if got.CurrentSli == nil || *got.CurrentSli != 90 {
		t.Fatalf("a crash uploaded on the next launch must still count: %v", derefSLI(got.CurrentSli))
	}
}

func TestWorkerWritesEveryBucketOfATickInOneInsert(t *testing.T) {
	projectId := setupSLODB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Minute).Add(30 * time.Second)
	if err := telemetry.EndpointRepository.InsertAsync(ctx, []models.Endpoint{endpointAt(projectId, 200, now.Add(-2*time.Minute)), endpointAt(projectId, 500, now.Add(-2*time.Minute))}); err != nil {
		t.Fatal(err)
	}
	first := createTestSLO(t, projectId, models.SLIKindEndpoint, `{"endpoint":"","good":"availability"}`, 99, 7, 1)
	second := createTestSLO(t, projectId, models.SLIKindEndpoint, `{"endpoint":"GET /users","good":"availability"}`, 40, 7, 1)
	results := []*tickResult{
		computeBuckets(ctx, first, now, time.Now().Add(backfillBudget), lateDataWindow),
		computeBuckets(ctx, second, now, time.Now().Add(backfillBudget), lateDataWindow),
	}
	if err := storeBuckets(ctx, results); err != nil {
		t.Fatal(err)
	}
	for _, entity := range []*models.SLO{first, second} {
		good, total, err := telemetry.SLOBucketRepository.SumRange(ctx, projectId, entity.Id, now.Add(-time.Hour), now)
		if err != nil || good != 1 || total != 2 {
			t.Fatalf("SLO %d: %d/%d (%v), want 1/2", entity.Id, good, total, err)
		}
	}
}

func derefSLI(sli *float64) any {
	if sli == nil {
		return nil
	}
	return *sli
}
