//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package notifications

import (
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

func agentRuleFixture(t *testing.T, approval string) (*dispatchFixture, *models.NotificationRuleWithChannel) {
	t.Helper()
	fixture := setupDispatchDB(t)
	channelId, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		now := time.Now().UTC()
		return transactional.NotificationChannelRepository.Create(tx, &models.NotificationChannel{
			ProjectId: fixture.Rule.ProjectId, Name: "Fix agent", ChannelType: AgentChannelType,
			Config: []byte(`{"approval":"` + approval + `"}`), Enabled: true, CreatedAt: now, UpdatedAt: now,
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	rule := &models.NotificationRuleWithChannel{Id: 99, ProjectId: fixture.Rule.ProjectId, ChannelId: channelId, Name: "Agent on new errors", RuleType: "new_error", CooldownMinutes: 15, ChannelType: AgentChannelType, ChannelName: "Fix agent"}
	return fixture, rule
}

func TestAgentChannelDispatchesThroughTheStarter(t *testing.T) {
	fixture, rule := agentRuleFixture(t, "ask")
	var calls []string
	RegisterAttemptStarter(func(config json.RawMessage, r *models.NotificationRuleWithChannel, msg Message) (AttemptStartResult, error) {
		cfg, problem := ParseAgentChannelConfig(config)
		if problem != "" {
			return AttemptStartResult{}, errors.New(problem)
		}
		calls = append(calls, msg.DedupToken+"/"+cfg.Approval+"/"+msg.ProjectId)
		if msg.DedupToken == "existing000000000" {
			return AttemptStartResult{Existing: true}, nil
		}
		return AttemptStartResult{Started: true, Pending: cfg.Approval == AgentApprovalAsk}, nil
	})
	t.Cleanup(func() { RegisterAttemptStarter(nil) })

	msg := Message{Subject: "New error", Body: "boom", URL: "/issues/0123456789abcdef", DedupToken: "0123456789abcdef"}
	if !dispatch(rule, msg) {
		t.Fatal("a fire with a hash must be committed")
	}
	if len(calls) != 1 || calls[0] != "0123456789abcdef/ask/"+fixture.Rule.ProjectId.String() {
		t.Fatalf("starter calls = %v", calls)
	}
	if cooldowns.canFire(rule.Id, rule.CooldownMinutes) {
		t.Fatal("the cooldown must start at the fire")
	}
	if rows := outboxRows(t); len(rows) != 0 {
		t.Fatalf("an agent channel must not enqueue a delivery: %d rows", len(rows))
	}

	noHash := Message{Subject: "Error rate", Body: "5%", URL: "/endpoints", DedupToken: "GET /api/users"}
	noHash.DedupToken = ""
	if dispatch(rule, noHash) {
		t.Fatal("a fire without an exception must not count as committed")
	}
	if len(calls) != 1 {
		t.Fatalf("the starter must not see a fire without a hash: %v", calls)
	}

	RegisterAttemptStarter(nil)
	if dispatch(rule, msg) {
		t.Fatal("without a starter the fire must fail")
	}
}
