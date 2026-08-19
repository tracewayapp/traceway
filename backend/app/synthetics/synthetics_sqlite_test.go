//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package synthetics

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

func setupSyntheticsDB(t *testing.T) uuid.UUID {
	t.Helper()
	dbtest.SetupSQLite(t)
	t.Cleanup(func() { notifier = nil })

	project, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		return transactional.ProjectRepository.Create(tx, "synthetics-test", "gin")
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return project.Id
}

func createTestCheck(t *testing.T, projectId uuid.UUID, checkType string, config string, intervalSeconds int, failureThreshold int, nextRunAt time.Time) *models.SyntheticCheck {
	t.Helper()
	check := &models.SyntheticCheck{
		ProjectId:        projectId,
		Name:             "test check",
		CheckType:        checkType,
		Enabled:          true,
		IntervalSeconds:  intervalSeconds,
		TimeoutSeconds:   5,
		Config:           models.JSONText(config),
		FailureThreshold: failureThreshold,
		CurrentStatus:    models.CheckStatusUnknown,
		NextRunAt:        nextRunAt,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	id, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.SyntheticCheckRepository.Create(tx, check)
	})
	if err != nil {
		t.Fatalf("create check: %v", err)
	}
	check.Id = id
	return check
}

func reloadCheck(t *testing.T, id int) *models.SyntheticCheck {
	t.Helper()
	check, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.SyntheticCheck, error) {
		return transactional.SyntheticCheckRepository.FindById(tx, id)
	})
	if err != nil || check == nil {
		t.Fatalf("reload check %d: %v", id, err)
	}
	return check
}

func allRuns(t *testing.T) []*models.CheckRun {
	t.Helper()
	runs, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.CheckRun, error) {
		return lit.SelectNamed[models.CheckRun](
			tx,
			"SELECT id, check_id, project_id, check_type, status, scheduled_for, expires_at, claimed_at, claimed_by, created_at FROM check_runs ORDER BY id ASC",
			lit.P{},
		)
	})
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	return runs
}

func checkResults(t *testing.T, projectId uuid.UUID, checkId int) []*models.CheckResultEntry {
	t.Helper()
	results, _, err := telemetry.CheckResultRepository.FindByCheck(context.Background(), projectId, checkId, nil, nil, "", 1, 100)
	if err != nil {
		t.Fatalf("list check results: %v", err)
	}
	return results
}

func httpConfig(url string, extra string) string {
	if extra != "" {
		extra = "," + extra
	}
	return fmt.Sprintf(`{"url":%q,"method":"GET","followRedirects":true,"assertions":{}%s}`, url, extra)
}

func TestScheduleEnqueuesDueChecksOnce(t *testing.T) {
	projectId := setupSyntheticsDB(t)
	now := time.Now().UTC()
	check := createTestCheck(t, projectId, models.CheckTypeHttp, httpConfig("http://example.invalid", ""), 60, 1, now.Add(-time.Minute))

	ScheduleOnce(context.Background(), now)

	runs := allRuns(t)
	if len(runs) != 1 {
		t.Fatalf("expected 1 queued run, got %d", len(runs))
	}
	if runs[0].Status != models.CheckRunQueued || runs[0].CheckId != check.Id {
		t.Fatalf("unexpected run %+v", runs[0])
	}
	wantExpiry := now.Add(60 * time.Second)
	if !runs[0].ExpiresAt.Equal(wantExpiry) {
		t.Errorf("expires_at = %s, want %s", runs[0].ExpiresAt, wantExpiry)
	}
	reloaded := reloadCheck(t, check.Id)
	if !reloaded.NextRunAt.Equal(now.Add(60 * time.Second)) {
		t.Errorf("next_run_at = %s, want %s", reloaded.NextRunAt, now.Add(60*time.Second))
	}

	// The same tick must not double-enqueue.
	ScheduleOnce(context.Background(), now)
	if runs := allRuns(t); len(runs) != 1 {
		t.Fatalf("expected still 1 run after second tick, got %d", len(runs))
	}
}

func TestScheduleSkipsDisabledChecks(t *testing.T) {
	projectId := setupSyntheticsDB(t)
	now := time.Now().UTC()
	check := createTestCheck(t, projectId, models.CheckTypeHttp, httpConfig("http://example.invalid", ""), 60, 1, now.Add(-time.Minute))
	check.Enabled = false
	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, transactional.SyntheticCheckRepository.Update(tx, check)
	}); err != nil {
		t.Fatalf("disable check: %v", err)
	}

	ScheduleOnce(context.Background(), now)
	if runs := allRuns(t); len(runs) != 0 {
		t.Fatalf("expected no runs for a disabled check, got %d", len(runs))
	}
}

func TestAllowPrivateTargetsForcedOffInCloudMode(t *testing.T) {
	setupSyntheticsDB(t)
	previousCloud := config.Config.CloudMode
	previousAllow := config.Config.SyntheticsAllowPrivateTargets
	t.Cleanup(func() {
		config.Config.CloudMode = previousCloud
		config.Config.SyntheticsAllowPrivateTargets = previousAllow
	})

	config.Config.CloudMode = ""
	config.Config.SyntheticsAllowPrivateTargets = ""
	if !AllowPrivateTargets() {
		t.Fatal("self-hosted default must allow private targets")
	}

	// Cloud mode must force the SSRF guard on even when the env var says allow.
	config.Config.CloudMode = "true"
	config.Config.SyntheticsAllowPrivateTargets = "true"
	if AllowPrivateTargets() {
		t.Fatal("cloud mode must never allow private targets")
	}
}

func TestScheduleSkipsBrowserChecksWhenModeOff(t *testing.T) {
	projectId := setupSyntheticsDB(t)
	previousMode := config.Config.SyntheticsBrowserMode
	t.Cleanup(func() { config.Config.SyntheticsBrowserMode = previousMode })

	config.Config.SyntheticsBrowserMode = BrowserModeOff
	now := time.Now().UTC()
	check := createTestCheck(t, projectId, models.CheckTypeBrowser, `{"script":"import {} from '@playwright/test';"}`, 60, 1, now.Add(-time.Minute))

	ScheduleOnce(context.Background(), now)
	if runs := allRuns(t); len(runs) != 0 {
		t.Fatalf("expected no queued runs with browser mode off, got %d", len(runs))
	}
	// The pointer must still advance so the check resumes on schedule (and the
	// scheduler does not refetch it every tick) once the mode comes back.
	reloaded := reloadCheck(t, check.Id)
	if !reloaded.NextRunAt.Equal(now.Add(60 * time.Second)) {
		t.Errorf("next_run_at = %s, want %s", reloaded.NextRunAt, now.Add(60*time.Second))
	}

	config.Config.SyntheticsBrowserMode = BrowserModeRemote
	later := now.Add(2 * time.Minute)
	ScheduleOnce(context.Background(), later)
	if runs := allRuns(t); len(runs) != 1 {
		t.Fatalf("expected 1 queued run with browser mode on, got %d", len(runs))
	}
}

func TestExecuteHttpCheckRecordsUp(t *testing.T) {
	projectId := setupSyntheticsDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, "hello world")
	}))
	defer server.Close()

	now := time.Now().UTC()
	check := createTestCheck(t, projectId, models.CheckTypeHttp, httpConfig(server.URL, `"assertions":{"bodyContains":"hello"}`), 60, 1, now.Add(-time.Minute))

	ScheduleOnce(context.Background(), now)
	ExecuteOnce(context.Background(), now)

	if runs := allRuns(t); len(runs) != 0 {
		t.Fatalf("expected run row deleted after execution, got %d rows", len(runs))
	}
	reloaded := reloadCheck(t, check.Id)
	if reloaded.CurrentStatus != models.CheckStatusUp {
		t.Errorf("current_status = %s, want up", reloaded.CurrentStatus)
	}
	if reloaded.ConsecutiveFailures != 0 {
		t.Errorf("consecutive_failures = %d, want 0", reloaded.ConsecutiveFailures)
	}
	if reloaded.LastRunAt == nil {
		t.Error("last_run_at not set")
	}

	results := checkResults(t, projectId, check.Id)
	if len(results) != 1 {
		t.Fatalf("expected 1 telemetry result, got %d", len(results))
	}
	if results[0].Status != models.CheckResultUp || results[0].StatusCode != 200 {
		t.Errorf("unexpected result %+v", results[0])
	}
	if results[0].LatencyMs <= 0 {
		t.Errorf("latency_ms = %f, want > 0", results[0].LatencyMs)
	}
}

func TestFailureThresholdIncidentAndRecovery(t *testing.T) {
	projectId := setupSyntheticsDB(t)
	var failing atomic.Bool
	failing.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	transitions := make(chan StateTransition, 4)
	RegisterNotifier(func(transition StateTransition) { transitions <- transition })

	now := time.Now().UTC()
	check := createTestCheck(t, projectId, models.CheckTypeHttp, httpConfig(server.URL, ""), 60, 2, now.Add(-time.Minute))

	runOnce := func(tick time.Time) {
		ScheduleOnce(context.Background(), tick)
		ExecuteOnce(context.Background(), tick)
	}

	// First failure: below the threshold, no transition.
	runOnce(now)
	reloaded := reloadCheck(t, check.Id)
	if reloaded.CurrentStatus != models.CheckStatusUnknown || reloaded.ConsecutiveFailures != 1 {
		t.Fatalf("after first failure: status=%s failures=%d, want unknown/1", reloaded.CurrentStatus, reloaded.ConsecutiveFailures)
	}
	select {
	case tr := <-transitions:
		t.Fatalf("unexpected transition after first failure: %+v", tr)
	case <-time.After(100 * time.Millisecond):
	}

	// Second failure crosses the threshold: down + incident + notification.
	runOnce(now.Add(61 * time.Second))
	reloaded = reloadCheck(t, check.Id)
	if reloaded.CurrentStatus != models.CheckStatusDown || reloaded.ConsecutiveFailures != 2 {
		t.Fatalf("after second failure: status=%s failures=%d, want down/2", reloaded.CurrentStatus, reloaded.ConsecutiveFailures)
	}
	select {
	case tr := <-transitions:
		if tr.To != models.CheckStatusDown || tr.Check.Id != check.Id {
			t.Fatalf("unexpected transition %+v", tr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected a down transition notification")
	}
	openIncident, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.CheckIncident, error) {
		return transactional.CheckIncidentRepository.FindOpenByCheck(tx, check.Id)
	})
	if err != nil || openIncident == nil {
		t.Fatalf("expected an open incident, got %+v (%v)", openIncident, err)
	}
	if openIncident.CheckId == nil || *openIncident.CheckId != check.Id {
		t.Fatalf("open incident check id = %v, want %d", openIncident.CheckId, check.Id)
	}
	if openIncident.StatusPageId != nil {
		t.Fatalf("auto incident carries a status page id: %+v", openIncident)
	}
	openUpdates, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.IncidentUpdate, error) {
		return transactional.IncidentUpdateRepository.FindByIncident(tx, openIncident.Id)
	})
	if err != nil {
		t.Fatalf("find incident updates: %v", err)
	}
	if len(openUpdates) != 1 || openUpdates[0].Status != models.IncidentUpdateInvestigating {
		t.Fatalf("expected one seeded investigating update, got %+v", openUpdates)
	}
	if openUpdates[0].Message != "" {
		t.Fatalf("seeded update must not leak the probe error, got %q", openUpdates[0].Message)
	}

	// A third failure must not re-notify or duplicate the incident.
	runOnce(now.Add(122 * time.Second))
	select {
	case tr := <-transitions:
		t.Fatalf("unexpected transition while already down: %+v", tr)
	case <-time.After(100 * time.Millisecond):
	}

	// Recovery: up + incident resolved + notification.
	failing.Store(false)
	runOnce(now.Add(183 * time.Second))
	reloaded = reloadCheck(t, check.Id)
	if reloaded.CurrentStatus != models.CheckStatusUp || reloaded.ConsecutiveFailures != 0 {
		t.Fatalf("after recovery: status=%s failures=%d, want up/0", reloaded.CurrentStatus, reloaded.ConsecutiveFailures)
	}
	select {
	case tr := <-transitions:
		if tr.From != models.CheckStatusDown || tr.To != models.CheckStatusUp {
			t.Fatalf("unexpected recovery transition %+v", tr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected a recovery transition notification")
	}
	stillOpen, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.CheckIncident, error) {
		return transactional.CheckIncidentRepository.FindOpenByCheck(tx, check.Id)
	})
	if err != nil {
		t.Fatalf("find open incident: %v", err)
	}
	if stillOpen != nil {
		t.Fatalf("incident not resolved: %+v", stillOpen)
	}
	resolvedUpdates, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.IncidentUpdate, error) {
		return transactional.IncidentUpdateRepository.FindByIncident(tx, openIncident.Id)
	})
	if err != nil {
		t.Fatalf("find incident updates after recovery: %v", err)
	}
	if len(resolvedUpdates) != 2 || resolvedUpdates[0].Status != models.IncidentUpdateResolved {
		t.Fatalf("expected a seeded resolved update on top, got %+v", resolvedUpdates)
	}

	results := checkResults(t, projectId, check.Id)
	if len(results) != 4 {
		t.Fatalf("expected 4 telemetry results, got %d", len(results))
	}
}

func TestExpiredQueuedRunRecordedAsMissed(t *testing.T) {
	projectId := setupSyntheticsDB(t)
	now := time.Now().UTC()
	check := createTestCheck(t, projectId, models.CheckTypeHttp, httpConfig("http://example.invalid", ""), 30, 1, now.Add(time.Hour))

	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.CheckRunRepository.Enqueue(tx, &models.CheckRun{
			CheckId:      check.Id,
			ProjectId:    projectId,
			CheckType:    check.CheckType,
			Status:       models.CheckRunQueued,
			ScheduledFor: now.Add(-10 * time.Minute),
			ExpiresAt:    now.Add(-9 * time.Minute),
			CreatedAt:    now.Add(-10 * time.Minute),
		})
	}); err != nil {
		t.Fatalf("enqueue stale run: %v", err)
	}

	ScheduleOnce(context.Background(), now)

	if runs := allRuns(t); len(runs) != 0 {
		t.Fatalf("expected expired run deleted, got %d rows", len(runs))
	}
	results := checkResults(t, projectId, check.Id)
	if len(results) != 1 || results[0].Status != models.CheckResultMissed {
		t.Fatalf("expected 1 missed result, got %+v", results)
	}
	// A missed probe is no evidence of health: state must be untouched.
	if reloaded := reloadCheck(t, check.Id); reloaded.CurrentStatus != models.CheckStatusUnknown {
		t.Errorf("current_status = %s, want unknown", reloaded.CurrentStatus)
	}
}

func TestStaleClaimedRunIsReclaimed(t *testing.T) {
	projectId := setupSyntheticsDB(t)
	now := time.Now().UTC()
	check := createTestCheck(t, projectId, models.CheckTypeHttp, httpConfig("http://example.invalid", ""), 600, 1, now.Add(time.Hour))

	runId, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		id, err := transactional.CheckRunRepository.Enqueue(tx, &models.CheckRun{
			CheckId:      check.Id,
			ProjectId:    projectId,
			CheckType:    check.CheckType,
			Status:       models.CheckRunQueued,
			ScheduledFor: now.Add(-time.Minute),
			ExpiresAt:    now.Add(4 * time.Minute),
			CreatedAt:    now.Add(-time.Minute),
		})
		if err != nil {
			return 0, err
		}
		// Claimed 10 minutes ago by an executor that died.
		if _, err := transactional.CheckRunRepository.Claim(tx, id, "dead-instance", now.Add(-10*time.Minute)); err != nil {
			return 0, err
		}
		return id, nil
	})
	if err != nil {
		t.Fatalf("enqueue+claim: %v", err)
	}

	ScheduleOnce(context.Background(), now)

	runs := allRuns(t)
	if len(runs) != 1 || runs[0].Id != runId {
		t.Fatalf("expected the run to survive, got %+v", runs)
	}
	if runs[0].Status != models.CheckRunQueued || runs[0].ClaimedAt != nil {
		t.Fatalf("expected the stale claim reclaimed to queued, got %+v", runs[0])
	}
}

func TestProcessOutcomeDiscardsLostClaim(t *testing.T) {
	projectId := setupSyntheticsDB(t)
	now := time.Now().UTC()
	check := createTestCheck(t, projectId, models.CheckTypeHttp, httpConfig("http://example.invalid", ""), 60, 1, now.Add(time.Hour))

	run := &models.CheckRun{
		CheckId:      check.Id,
		ProjectId:    projectId,
		CheckType:    check.CheckType,
		Status:       models.CheckRunQueued,
		ScheduledFor: now,
		ExpiresAt:    now.Add(time.Minute),
		CreatedAt:    now,
	}
	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		id, err := transactional.CheckRunRepository.Enqueue(tx, run)
		run.Id = id
		if err != nil {
			return 0, err
		}
		_, err = transactional.CheckRunRepository.Claim(tx, id, "owner-a", now)
		return id, err
	}); err != nil {
		t.Fatalf("enqueue+claim: %v", err)
	}

	// owner-b lost the claim to owner-a; its outcome must be discarded.
	ProcessOutcome(context.Background(), run, "owner-b", Outcome{
		Status: models.CheckResultDown, ErrorMsg: "boom", ExecutedAt: now, ExecutedBy: "owner-b", TlsDaysRemaining: -1,
	})

	if reloaded := reloadCheck(t, check.Id); reloaded.CurrentStatus != models.CheckStatusUnknown || reloaded.ConsecutiveFailures != 0 {
		t.Fatalf("lost claim must not change state, got %+v", reloaded)
	}
	if results := checkResults(t, projectId, check.Id); len(results) != 0 {
		t.Fatalf("lost claim must not record telemetry, got %d rows", len(results))
	}
	if runs := allRuns(t); len(runs) != 1 {
		t.Fatalf("run row must survive a lost finalize, got %d", len(runs))
	}
}

func TestTcpCheckUpAndDown(t *testing.T) {
	projectId := setupSyntheticsDB(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			fmt.Fprint(conn, "WELCOME traceway\n")
			conn.Close()
		}
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	now := time.Now().UTC()
	upCheck := createTestCheck(t, projectId, models.CheckTypeTcp,
		fmt.Sprintf(`{"host":"127.0.0.1","port":%d,"expectContains":"WELCOME"}`, port), 60, 1, now.Add(-time.Minute))

	closedPort := port + 1
	downCheck := createTestCheck(t, projectId, models.CheckTypeTcp,
		fmt.Sprintf(`{"host":"127.0.0.1","port":%d}`, closedPort), 60, 1, now.Add(-time.Minute))

	ScheduleOnce(context.Background(), now)
	ExecuteOnce(context.Background(), now)

	if reloaded := reloadCheck(t, upCheck.Id); reloaded.CurrentStatus != models.CheckStatusUp {
		t.Errorf("tcp up check status = %s, want up", reloaded.CurrentStatus)
	}
	if reloaded := reloadCheck(t, downCheck.Id); reloaded.CurrentStatus != models.CheckStatusDown {
		t.Errorf("tcp down check status = %s, want down", reloaded.CurrentStatus)
	}
}
