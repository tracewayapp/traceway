//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package notifications

import (
	"context"
	"database/sql"
	"strings"
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

type sloRuleFixture struct {
	ProjectId uuid.UUID
	SLO       *models.SLO
	Rule      *models.NotificationRule
}

func setupSLORuleDB(t *testing.T, ruleType, config string, cooldownMinutes int) *sloRuleFixture {
	t.Helper()
	dbtest.SetupSQLite(t)
	t.Cleanup(func() {
		cooldowns.mu.Lock()
		cooldowns.fired = make(map[int]time.Time)
		cooldowns.mu.Unlock()
	})
	fixture := &sloRuleFixture{}
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		project, err := transactional.ProjectRepository.Create(tx, "api", "gin")
		if err != nil {
			return struct{}{}, err
		}
		fixture.ProjectId = project.Id
		now := time.Now().UTC()
		entity := &models.SLO{
			ProjectId: project.Id, Name: "Checkout availability", SliKind: models.SLIKindEndpoint, SliConfig: models.JSONText(`{"endpoint":"","good":"availability"}`),
			TargetPercent: 99, WindowDays: 7, BucketMinutes: 1, Enabled: true, Status: models.SLOStatusNoData, LastBucketAt: &now, CreatedAt: now, UpdatedAt: now,
		}
		sloId, err := transactional.SLORepository.Create(tx, entity)
		if err != nil {
			return struct{}{}, err
		}
		entity.Id = sloId
		fixture.SLO = entity
		channel := &models.NotificationChannel{ProjectId: project.Id, Name: "Ops", ChannelType: "slack", Config: []byte(`{"webhookUrl":"https://hooks.example.com/x"}`), Enabled: true, CreatedAt: now, UpdatedAt: now}
		channelId, err := transactional.NotificationChannelRepository.Create(tx, channel)
		if err != nil {
			return struct{}{}, err
		}
		rule := &models.NotificationRule{ProjectId: project.Id, ChannelId: channelId, Name: "burn", RuleType: ruleType, Config: []byte(config), Enabled: true, CooldownMinutes: cooldownMinutes, CreatedAt: now, UpdatedAt: now}
		ruleId, err := transactional.NotificationRuleRepository.Create(tx, rule)
		if err != nil {
			return struct{}{}, err
		}
		rule.Id = ruleId
		fixture.Rule = rule
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return fixture
}

func seedBuckets(t *testing.T, fixture *sloRuleFixture, minutes int, good, total int64) {
	t.Helper()
	end := time.Now().UTC().Truncate(time.Minute)
	buckets := make([]shared.SLOBucket, 0, minutes)
	for i := 1; i <= minutes; i++ {
		buckets = append(buckets, shared.SLOBucket{ProjectId: fixture.ProjectId, SLOId: fixture.SLO.Id, BucketStart: end.Add(-time.Duration(i) * time.Minute), Good: good, Total: total, ComputedAt: end})
	}
	if err := telemetry.SLOBucketRepository.Upsert(context.Background(), buckets); err != nil {
		t.Fatal(err)
	}
}

func ruleState(t *testing.T, ruleId int) bool {
	t.Helper()
	state, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.NotificationRuleState, error) {
		return transactional.NotificationRuleStateRepository.Find(tx, ruleId)
	})
	if err != nil {
		t.Fatal(err)
	}
	return state != nil && state.Firing
}

func TestSLOBurnRateFiresOnceAndRecovers(t *testing.T) {
	fixture := setupSLORuleDB(t, "slo_burn_rate", `{"sloId":1,"longWindowMinutes":60,"shortWindowMinutes":5,"burnRateThreshold":6}`, 0)
	ctx := context.Background()
	seedBuckets(t, fixture, 60, 90, 100)

	result, err := evaluateSLOBurnRate(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Fired || result.Recovered {
		t.Fatalf("expected a fire: %+v", result)
	}
	if result.Message.Severity != SeverityWarning || result.Message.DedupToken != SLODedupToken(fixture.SLO.Id) || result.Message.URL != "/slos/1" {
		t.Fatalf("message: %+v", result.Message)
	}
	if !ruleState(t, fixture.Rule.Id) {
		t.Fatal("firing state not persisted")
	}

	result, err = evaluateSLOBurnRate(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil {
		t.Fatal(err)
	}
	if result.Fired || result.Recovered {
		t.Fatalf("a burning SLO must not fire again: %+v", result)
	}

	seedBuckets(t, fixture, 60, 100, 100)
	result, err = evaluateSLOBurnRate(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Recovered || result.Fired || !result.Message.Recovered || result.Message.Severity != SeverityInfo {
		t.Fatalf("expected a recovery: %+v", result)
	}
	if ruleState(t, fixture.Rule.Id) {
		t.Fatal("firing state not cleared")
	}
	result, err = evaluateSLOBurnRate(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil || result.Fired || result.Recovered {
		t.Fatalf("a recovered SLO is quiet: %+v, %v", result, err)
	}
}

func TestSLOBurnRateCriticalAndShortWindow(t *testing.T) {
	fixture := setupSLORuleDB(t, "slo_burn_rate", `{"sloId":1,"longWindowMinutes":60,"shortWindowMinutes":5,"burnRateThreshold":6}`, 0)
	ctx := context.Background()
	seedBuckets(t, fixture, 60, 80, 100)
	end := time.Now().UTC().Truncate(time.Minute)
	var clean []shared.SLOBucket
	for i := 1; i <= 5; i++ {
		clean = append(clean, shared.SLOBucket{ProjectId: fixture.ProjectId, SLOId: fixture.SLO.Id, BucketStart: end.Add(-time.Duration(i) * time.Minute), Good: 100, Total: 100, ComputedAt: end})
	}
	if err := telemetry.SLOBucketRepository.Upsert(ctx, clean); err != nil {
		t.Fatal(err)
	}
	result, err := evaluateSLOBurnRate(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil || result.Fired {
		t.Fatalf("short window must also burn: %+v, %v", result, err)
	}
	seedBuckets(t, fixture, 60, 80, 100)
	result, err = evaluateSLOBurnRate(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil || !result.Fired || result.Message.Severity != SeverityCritical {
		t.Fatalf("20x against 6x is critical: %+v, %v", result, err)
	}
}

func TestSLOBurnRateHonoursCooldown(t *testing.T) {
	fixture := setupSLORuleDB(t, "slo_burn_rate", `{"sloId":1,"longWindowMinutes":60,"shortWindowMinutes":5,"burnRateThreshold":6}`, 15)
	ctx := context.Background()
	seedBuckets(t, fixture, 60, 90, 100)
	cooldowns.recordFire(fixture.Rule.Id)

	result, err := evaluateSLOBurnRate(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil || result.Fired {
		t.Fatalf("cooldown suppresses the fire: %+v, %v", result, err)
	}
	if ruleState(t, fixture.Rule.Id) {
		t.Fatal("a suppressed fire must not be recorded as firing")
	}
	cooldowns.mu.Lock()
	delete(cooldowns.fired, fixture.Rule.Id)
	cooldowns.mu.Unlock()
	result, err = evaluateSLOBurnRate(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil || !result.Fired {
		t.Fatalf("fires once the cooldown passed: %+v, %v", result, err)
	}
}

func TestSLORecoveryDoesNotRestartTheCooldown(t *testing.T) {
	fixture := setupSLORuleDB(t, "slo_burn_rate", `{"sloId":1,"longWindowMinutes":60,"shortWindowMinutes":5,"burnRateThreshold":6}`, 15)
	ctx := context.Background()
	seedBuckets(t, fixture, 60, 90, 100)
	evaluatePolledRules(ctx)
	if !ruleState(t, fixture.Rule.Id) {
		t.Fatal("expected the first fire")
	}
	cooldowns.mu.Lock()
	cooldowns.fired[fixture.Rule.Id] = time.Now().Add(-40 * time.Minute)
	cooldowns.mu.Unlock()

	seedBuckets(t, fixture, 60, 100, 100)
	evaluatePolledRules(ctx)
	if ruleState(t, fixture.Rule.Id) {
		t.Fatal("expected the recovery")
	}
	seedBuckets(t, fixture, 60, 90, 100)
	evaluatePolledRules(ctx)
	if !ruleState(t, fixture.Rule.Id) {
		t.Fatal("a burn 40 minutes after the last alert must fire again; the recovery notice is not a fire")
	}
}

func setBudgetRemaining(t *testing.T, fixture *sloRuleFixture, remaining *float64) {
	t.Helper()
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		_, err := tx.Exec("UPDATE slos SET budget_remaining = ? WHERE id = ?", remaining, fixture.SLO.Id)
		return struct{}{}, err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func budget(remaining float64) *float64 {
	return &remaining
}

func TestSLOBudgetConsumedFiresAndRecovers(t *testing.T) {
	fixture := setupSLORuleDB(t, "slo_budget_consumed", `{"sloId":1,"consumedPercent":80}`, 0)
	ctx := context.Background()
	setBudgetRemaining(t, fixture, budget(15))

	result, err := evaluateSLOBudgetConsumed(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil || !result.Fired || result.Message.Severity != SeverityWarning {
		t.Fatalf("expected a warning fire: %+v, %v", result, err)
	}
	setBudgetRemaining(t, fixture, budget(50))
	result, err = evaluateSLOBudgetConsumed(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil || !result.Recovered {
		t.Fatalf("half the budget back is a recovery: %+v, %v", result, err)
	}
	setBudgetRemaining(t, fixture, budget(-100))
	result, err = evaluateSLOBudgetConsumed(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil || !result.Fired || result.Message.Severity != SeverityCritical {
		t.Fatalf("an exhausted budget is critical: %+v, %v", result, err)
	}
}

func TestSLOBurnRateShortWindowCoversOneSlice(t *testing.T) {
	fixture := setupSLORuleDB(t, "slo_burn_rate", `{"sloId":1,"longWindowMinutes":60,"shortWindowMinutes":5,"burnRateThreshold":6}`, 0)
	ctx := context.Background()
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		_, err := tx.Exec("UPDATE slos SET bucket_minutes = 60 WHERE id = ?", fixture.SLO.Id)
		return struct{}{}, err
	})
	if err != nil {
		t.Fatal(err)
	}
	lastSlice := time.Now().UTC().Truncate(time.Hour).Add(-time.Hour)
	bucket := shared.SLOBucket{ProjectId: fixture.ProjectId, SLOId: fixture.SLO.Id, BucketStart: lastSlice, Good: 0, Total: 1, ComputedAt: time.Now().UTC()}
	if err := telemetry.SLOBucketRepository.Upsert(ctx, []shared.SLOBucket{bucket}); err != nil {
		t.Fatal(err)
	}

	result, err := evaluateSLOBurnRate(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Fired {
		t.Fatalf("a 5m short window on hourly slices must read the last slice and fire: %+v", result)
	}
}

func TestSLORuleRecoversWhenSLODisabled(t *testing.T) {
	fixture := setupSLORuleDB(t, "slo_burn_rate", `{"sloId":1,"longWindowMinutes":60,"shortWindowMinutes":5,"burnRateThreshold":6}`, 0)
	ctx := context.Background()
	seedBuckets(t, fixture, 60, 90, 100)
	if result, err := evaluateSLOBurnRate(ctx, fixture.Rule, fixture.ProjectId); err != nil || !result.Fired {
		t.Fatalf("expected a fire: %+v %v", result, err)
	}
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		_, err := tx.Exec("UPDATE slos SET enabled = 0 WHERE id = ?", fixture.SLO.Id)
		return struct{}{}, err
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := evaluateSLOBurnRate(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Recovered || ruleState(t, fixture.Rule.Id) {
		t.Fatalf("disabling the SLO must end the firing episode: %+v", result)
	}
	result, err = evaluateSLOBurnRate(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil || result.Recovered || result.Fired {
		t.Fatalf("a disabled SLO stays quiet afterwards: %+v %v", result, err)
	}
}

func TestSLOBudgetConsumedRecoveryWithoutData(t *testing.T) {
	fixture := setupSLORuleDB(t, "slo_budget_consumed", `{"sloId":1,"consumedPercent":80}`, 0)
	ctx := context.Background()
	setBudgetRemaining(t, fixture, budget(10))
	if result, err := evaluateSLOBudgetConsumed(ctx, fixture.Rule, fixture.ProjectId); err != nil || !result.Fired {
		t.Fatalf("expected a fire: %+v, %v", result, err)
	}
	setBudgetRemaining(t, fixture, nil)
	result, err := evaluateSLOBudgetConsumed(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil || !result.Recovered {
		t.Fatalf("an SLO without data recovers: %+v, %v", result, err)
	}
	if !strings.Contains(result.Message.Body, "no data") || strings.Contains(result.Message.Body, "100.0%") {
		t.Fatalf("the recovery must not claim the budget is consumed: %q", result.Message.Body)
	}
}

func TestSLORuleHoldsWhileTheHistoryIsRecomputed(t *testing.T) {
	fixture := setupSLORuleDB(t, "slo_budget_consumed", `{"sloId":1,"consumedPercent":80}`, 0)
	ctx := context.Background()
	setBudgetRemaining(t, fixture, budget(10))
	if result, err := evaluateSLOBudgetConsumed(ctx, fixture.Rule, fixture.ProjectId); err != nil || !result.Fired {
		t.Fatalf("expected a fire: %+v, %v", result, err)
	}
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, transactional.SLORepository.ResetHistory(tx, fixture.SLO.Id, time.Now().UTC())
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := evaluateSLOBudgetConsumed(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil || result.Recovered || result.Fired {
		t.Fatalf("a definition change must not send a recovery before the worker recomputed the SLO: %+v, %v", result, err)
	}
	if !ruleState(t, fixture.Rule.Id) {
		t.Fatal("the rule keeps firing until the recomputed snapshot says otherwise")
	}
}

func TestSLOBurnRateRecoversOnTheShortWindow(t *testing.T) {
	fixture := setupSLORuleDB(t, "slo_burn_rate", `{"sloId":1,"longWindowMinutes":60,"shortWindowMinutes":5,"burnRateThreshold":6}`, 0)
	ctx := context.Background()
	seedBuckets(t, fixture, 60, 90, 100)
	if result, err := evaluateSLOBurnRate(ctx, fixture.Rule, fixture.ProjectId); err != nil || !result.Fired {
		t.Fatalf("expected a fire: %+v, %v", result, err)
	}
	seedBuckets(t, fixture, 5, 100, 100)
	result, err := evaluateSLOBurnRate(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil || !result.Recovered {
		t.Fatalf("a clean short window ends the episode although the hour still burns: %+v, %v", result, err)
	}
}

func TestSLORuleStateTransitionsOnce(t *testing.T) {
	fixture := setupSLORuleDB(t, "slo_burn_rate", `{"sloId":1}`, 0)
	now := time.Now().UTC()
	transition := func(firing bool) bool {
		changed, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
			return transactional.NotificationRuleStateRepository.Transition(tx, fixture.Rule.Id, firing, now)
		})
		if err != nil {
			t.Fatal(err)
		}
		return changed
	}
	if !transition(true) || transition(true) {
		t.Fatal("only the first of two instances may record the fire")
	}
	if !transition(false) || transition(false) {
		t.Fatal("only the first of two instances may record the recovery")
	}
}

func TestSLORuleRetriesWhenTheSendFails(t *testing.T) {
	fixture := setupSLORuleDB(t, "slo_burn_rate", `{"sloId":1,"longWindowMinutes":60,"shortWindowMinutes":5,"burnRateThreshold":6}`, 0)
	seedBuckets(t, fixture, 60, 90, 100)
	if _, err := db.DB.Exec(`UPDATE notification_channels SET channel_type = 'escalation', config = '{"policyId":1}'`); err != nil {
		t.Fatal(err)
	}
	previous := pageOpener
	pageOpener = nil
	t.Cleanup(func() { pageOpener = previous })

	evaluatePolledRules(context.Background())
	if ruleState(t, fixture.Rule.Id) {
		t.Fatal("a rule whose page could not be opened must not stay firing")
	}

	result, err := evaluateSLOBurnRate(context.Background(), fixture.Rule, fixture.ProjectId)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Fired {
		t.Fatalf("the next poll should try again: %+v", result)
	}
}
