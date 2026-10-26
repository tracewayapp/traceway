//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package notifications

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
			TargetPercent: 99, WindowDays: 7, BucketMinutes: 1, Enabled: true, Status: models.SLOStatusNoData, CreatedAt: now, UpdatedAt: now,
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

// seedBuckets writes minutes of buckets ending at the last closed minute, each with the given counts.
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
	// 10% bad against a 1% allowance is a 10x burn.
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
	// 20x over the hour, but the last five minutes are clean: no fire.
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

func TestSLOBudgetConsumedFiresAndRecovers(t *testing.T) {
	fixture := setupSLORuleDB(t, "slo_budget_consumed", `{"sloId":1,"consumedPercent":80}`, 0)
	ctx := context.Background()
	// 0.85% bad of a 1% allowance: 85% of the budget is gone.
	seedBuckets(t, fixture, 100, 9915, 10000)

	result, err := evaluateSLOBudgetConsumed(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil || !result.Fired || result.Message.Severity != SeverityWarning {
		t.Fatalf("expected a warning fire: %+v, %v", result, err)
	}
	seedBuckets(t, fixture, 100, 9950, 10000)
	result, err = evaluateSLOBudgetConsumed(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil || !result.Recovered {
		t.Fatalf("half the budget back is a recovery: %+v, %v", result, err)
	}
	seedBuckets(t, fixture, 100, 9800, 10000)
	result, err = evaluateSLOBudgetConsumed(ctx, fixture.Rule, fixture.ProjectId)
	if err != nil || !result.Fired || result.Message.Severity != SeverityCritical {
		t.Fatalf("an exhausted budget is critical: %+v, %v", result, err)
	}
}
