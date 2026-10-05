//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package oncall

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/synthetics"
)

func seedCheckDownEscalationRule(t *testing.T, fixture *escalatorFixture) int {
	t.Helper()
	notifications.RegisterPageOpener(OpenPageFromDispatch)
	notifications.RegisterPageResolver(AutoResolveByDedupKey)

	policyId := createPolicy(t, fixture.OrgId, `{"schemaVersion":1,"steps":[{"targets":[{"type":"user","id":`+fmt.Sprintf("%d", fixture.Alice)+`}],"delayMinutes":5}]}`)

	ruleId, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		now := time.Now().UTC()
		channel := &models.NotificationChannel{
			ProjectId:   fixture.ProjectId,
			Name:        "Escalation",
			ChannelType: "escalation",
			Config:      []byte(fmt.Sprintf(`{"policyId":%d}`, policyId)),
			Enabled:     true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		channelId, err := transactional.NotificationChannelRepository.Create(tx, channel)
		if err != nil {
			return 0, err
		}
		return transactional.NotificationRuleRepository.Create(tx, &models.NotificationRule{
			ProjectId:       fixture.ProjectId,
			ChannelId:       channelId,
			Name:            "Checks down",
			RuleType:        "check_down",
			Config:          []byte(`{}`),
			Enabled:         true,
			CooldownMinutes: 15,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	})
	if err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	t.Cleanup(func() { notifications.ClearRuleState(ruleId) })
	return ruleId
}

func checkWent(check models.SyntheticCheck, from, to string) synthetics.StateTransition {
	return synthetics.StateTransition{Check: check, From: from, To: to, ErrorMsg: "status code 503", At: time.Now().UTC()}
}

// TestCheckDownOpensPageAndRecoveryAutoResolves exercises the full synthetic
// check alerting loop through an escalation channel: down opens a page,
// recovery resolves it with no resolving user.
func TestCheckDownOpensPageAndRecoveryAutoResolves(t *testing.T) {
	fixture := setupEscalatorDB(t)
	ruleId := seedCheckDownEscalationRule(t, fixture)

	check := models.SyntheticCheck{
		Id:                  7,
		ProjectId:           fixture.ProjectId,
		Name:                "API health",
		CheckType:           models.CheckTypeHttp,
		ConsecutiveFailures: 2,
	}

	// Down transition opens a page under the rule-scoped dedup key.
	notifications.OnCheckStateChange(checkWent(check, models.CheckStatusUp, models.CheckStatusDown))

	dedupKey := fmt.Sprintf("%d|check:%d", ruleId, check.Id)
	page := findPageByDedupKey(t, dedupKey)
	if page == nil {
		t.Fatal("expected the down transition to open a page")
	}
	if page.Status != models.PageStatusOpen || page.RuleType != "check_down" {
		t.Fatalf("unexpected page %+v", page)
	}

	// Recovery must resolve the page (system resolve), never open another.
	notifications.OnCheckStateChange(checkWent(check, models.CheckStatusDown, models.CheckStatusUp))

	if open := findPageByDedupKey(t, dedupKey); open != nil {
		t.Fatalf("expected the page resolved after recovery, still open: %+v", open)
	}
	resolved := reloadPage(t, page.Id)
	if resolved.Status != models.PageStatusResolved {
		t.Fatalf("page status = %s, want resolved", resolved.Status)
	}
	if resolved.ResolvedBy != nil {
		t.Fatalf("system resolve must leave resolved_by NULL, got %v", *resolved.ResolvedBy)
	}
}

// Issue #387: an outage that began inside the rule's cooldown opened no page,
// and nothing fired again for as long as it lasted.
func TestSecondOutageInsideCooldownOpensNewPage(t *testing.T) {
	fixture := setupEscalatorDB(t)
	ruleId := seedCheckDownEscalationRule(t, fixture)
	check := models.SyntheticCheck{Id: 8, ProjectId: fixture.ProjectId, Name: "API health", CheckType: models.CheckTypeHttp, ConsecutiveFailures: 1}
	dedupKey := fmt.Sprintf("%d|check:%d", ruleId, check.Id)

	notifications.OnCheckStateChange(checkWent(check, models.CheckStatusUp, models.CheckStatusDown))
	first := findPageByDedupKey(t, dedupKey)
	if first == nil {
		t.Fatal("expected the first outage to open a page")
	}
	notifications.OnCheckStateChange(checkWent(check, models.CheckStatusDown, models.CheckStatusUp))

	notifications.OnCheckStateChange(checkWent(check, models.CheckStatusUp, models.CheckStatusDown))
	second := findPageByDedupKey(t, dedupKey)
	if second == nil {
		t.Fatal("expected the second outage to open a page")
	}
	if second.Id == first.Id {
		t.Fatalf("the second outage reused page %d instead of opening a new one", first.Id)
	}
}

// Issue #391: a snooze muted the recovery along with the alerts, so the page
// an outage had already opened stayed open after the monitor came back.
func TestRecoveryResolvesPageWhileRuleSnoozed(t *testing.T) {
	fixture := setupEscalatorDB(t)
	ruleId := seedCheckDownEscalationRule(t, fixture)
	check := models.SyntheticCheck{Id: 9, ProjectId: fixture.ProjectId, Name: "API health", CheckType: models.CheckTypeHttp, ConsecutiveFailures: 1}
	dedupKey := fmt.Sprintf("%d|check:%d", ruleId, check.Id)

	notifications.OnCheckStateChange(checkWent(check, models.CheckStatusUp, models.CheckStatusDown))
	page := findPageByDedupKey(t, dedupKey)
	if page == nil {
		t.Fatal("expected the down transition to open a page")
	}

	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		until := time.Now().UTC().Add(time.Hour)
		return struct{}{}, transactional.NotificationRuleRepository.UpdateSnoozedUntil(tx, ruleId, &until)
	})
	if err != nil {
		t.Fatalf("snooze rule: %v", err)
	}

	notifications.OnCheckStateChange(checkWent(check, models.CheckStatusDown, models.CheckStatusUp))

	if status := reloadPage(t, page.Id).Status; status != models.PageStatusResolved {
		t.Fatalf("page status = %s, want resolved", status)
	}
}
