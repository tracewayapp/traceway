//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb && !telemetry_firebolt

package notifications

import (
	"database/sql"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

type dispatchFixture struct {
	Rule    *models.NotificationRuleWithChannel
	Channel *models.NotificationChannel
}

func setupDispatchDB(t *testing.T) *dispatchFixture {
	t.Helper()

	dbtest.SetupSQLite(t)
	t.Cleanup(func() {
		cooldowns.mu.Lock()
		cooldowns.fired = make(map[int]time.Time)
		cooldowns.mu.Unlock()
	})

	fixture := &dispatchFixture{}
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		org, err := transactional.OrganizationRepository.Create(tx, "Acme", "UTC")
		if err != nil {
			return struct{}{}, err
		}
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "api", "gin", org.Id)
		if err != nil {
			return struct{}{}, err
		}
		now := time.Now().UTC()
		channel := &models.NotificationChannel{
			ProjectId:   project.Id,
			Name:        "Ops Slack",
			ChannelType: "slack",
			Config:      []byte(`{"webhookUrl":"https://hooks.example.com/original"}`),
			Enabled:     true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		channelId, err := transactional.NotificationChannelRepository.Create(tx, channel)
		if err != nil {
			return struct{}{}, err
		}
		channel.Id = channelId
		fixture.Channel = channel
		fixture.Rule = &models.NotificationRuleWithChannel{
			Id: 42, ProjectId: project.Id, ChannelId: channelId,
			Name: "New errors", RuleType: "new_error", CooldownMinutes: 15,
			ChannelType: "slack", ChannelName: "Ops Slack",
		}
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return fixture
}

func outboxRows(t *testing.T) []*models.OutboxDelivery {
	t.Helper()
	rows, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.OutboxDelivery, error) {
		return transactional.OutboxRepository.FindDue(tx, time.Now().UTC().Add(time.Minute), 100)
	})
	if err != nil {
		t.Fatalf("load outbox rows: %v", err)
	}
	return rows
}

func TestDispatchEnqueuesSnapshot(t *testing.T) {
	fixture := setupDispatchDB(t)

	if !dispatch(fixture.Rule, Message{Subject: "s", Body: "b", Severity: SeverityCritical}) {
		t.Fatal("dispatch should report a durable enqueue")
	}
	rows := outboxRows(t)
	if len(rows) != 1 {
		t.Fatalf("expected 1 outbox row, got %d", len(rows))
	}
	row := rows[0]
	if row.AdapterType != "slack" || string(row.AdapterConfig) != `{"webhookUrl":"https://hooks.example.com/original"}` {
		t.Errorf("snapshot mismatch: %s %s", row.AdapterType, row.AdapterConfig)
	}
	if row.RuleId == nil || *row.RuleId != 42 || row.ChannelName != "Ops Slack" {
		t.Errorf("bookkeeping fields wrong: %+v", row)
	}

	// Mutating the channel after enqueue must not affect the queued row.
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		fixture.Channel.Config = []byte(`{"webhookUrl":"https://hooks.example.com/CHANGED"}`)
		return struct{}{}, transactional.NotificationChannelRepository.Update(tx, fixture.Channel)
	})
	if err != nil {
		t.Fatalf("update channel: %v", err)
	}
	if row := outboxRows(t)[0]; string(row.AdapterConfig) != `{"webhookUrl":"https://hooks.example.com/original"}` {
		t.Errorf("queued row changed after channel edit: %s", row.AdapterConfig)
	}

	if cooldowns.canFire(fixture.Rule.Id, fixture.Rule.CooldownMinutes) {
		t.Error("cooldown should be recorded at enqueue")
	}
}

func TestDispatchFailureLeavesNoCooldown(t *testing.T) {
	fixture := setupDispatchDB(t)
	fixture.Rule.ChannelId = 99999

	if dispatch(fixture.Rule, Message{Subject: "s"}) {
		t.Fatal("dispatch with a dangling channel should fail")
	}
	if len(outboxRows(t)) != 0 {
		t.Error("no outbox row should exist after a failed dispatch")
	}
	if !cooldowns.canFire(fixture.Rule.Id, fixture.Rule.CooldownMinutes) {
		t.Error("cooldown must not be recorded when dispatch fails")
	}
}

func TestSeedCooldownsIncludesOutbox(t *testing.T) {
	fixture := setupDispatchDB(t)

	if !dispatch(fixture.Rule, Message{Subject: "s"}) {
		t.Fatal("dispatch failed")
	}
	// Fresh tracker simulating a restart before any terminal outcome exists.
	cooldowns.mu.Lock()
	cooldowns.fired = make(map[int]time.Time)
	cooldowns.mu.Unlock()

	enqueued, err := db.ExecuteTransaction(func(tx *sql.Tx) (map[int]time.Time, error) {
		return transactional.OutboxRepository.LastEnqueuedPerRule(tx)
	})
	if err != nil {
		t.Fatalf("seed query: %v", err)
	}
	cooldowns.seed(enqueued)
	if cooldowns.canFire(fixture.Rule.Id, fixture.Rule.CooldownMinutes) {
		t.Error("outbox-backed seeding should keep the rule in cooldown after restart")
	}
}
