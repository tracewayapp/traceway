//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package notifications

import (
	"database/sql"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/v2/app/synthetics"
)

func seedCheckDownRule(t *testing.T, fixture *dispatchFixture, snoozedUntil *time.Time) int {
	t.Helper()
	ruleId, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		now := time.Now().UTC()
		return transactional.NotificationRuleRepository.Create(tx, &models.NotificationRule{
			ProjectId:       fixture.Channel.ProjectId,
			ChannelId:       fixture.Channel.Id,
			Name:            "Monitors",
			RuleType:        "check_down",
			Config:          []byte(`{}`),
			Enabled:         true,
			CooldownMinutes: 15,
			SnoozedUntil:    snoozedUntil,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	})
	if err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	t.Cleanup(func() { ClearRuleState(ruleId) })
	return ruleId
}

func queuedSubjects(t *testing.T) []string {
	t.Helper()
	rows := outboxRows(t)
	slices.SortFunc(rows, func(a, b *models.OutboxDelivery) int { return a.Id - b.Id })
	subjects := make([]string, 0, len(rows))
	for _, row := range rows {
		var msg Message
		if err := json.Unmarshal(row.Message, &msg); err != nil {
			t.Fatalf("decode queued message: %v", err)
		}
		subjects = append(subjects, msg.Subject)
	}
	return subjects
}

func checkWent(check models.SyntheticCheck, from, to string) synthetics.StateTransition {
	return synthetics.StateTransition{Check: check, From: from, To: to, ErrorMsg: "connection refused", At: time.Now().UTC()}
}

// Issue #387: the cooldown swallowed the alert for an outage that began inside
// it, and nothing fired again for as long as that outage lasted.
func TestCheckDownAlertsForEveryOutage(t *testing.T) {
	fixture := setupDispatchDB(t)
	seedCheckDownRule(t, fixture, nil)
	check := models.SyntheticCheck{Id: 7, ProjectId: fixture.Channel.ProjectId, Name: "api health", ConsecutiveFailures: 1}

	OnCheckStateChange(checkWent(check, models.CheckStatusUp, models.CheckStatusDown))
	OnCheckStateChange(checkWent(check, models.CheckStatusDown, models.CheckStatusUp))
	OnCheckStateChange(checkWent(check, models.CheckStatusUp, models.CheckStatusDown))
	OnCheckStateChange(checkWent(check, models.CheckStatusDown, models.CheckStatusUp))

	want := []string{
		`[api] Check "api health" is down`,
		`[api] Check "api health" recovered`,
		`[api] Check "api health" is down`,
		`[api] Check "api health" recovered`,
	}
	if got := queuedSubjects(t); !slices.Equal(got, want) {
		t.Errorf("queued notifications:\n got %q\nwant %q", got, want)
	}
}

func TestCheckDownCooldownHoldsWithinOneOutage(t *testing.T) {
	fixture := setupDispatchDB(t)
	seedCheckDownRule(t, fixture, nil)
	check := models.SyntheticCheck{Id: 7, ProjectId: fixture.Channel.ProjectId, Name: "api health", ConsecutiveFailures: 1}

	OnCheckStateChange(checkWent(check, models.CheckStatusUp, models.CheckStatusDown))
	OnCheckStateChange(checkWent(check, models.CheckStatusUnknown, models.CheckStatusDown))

	want := []string{`[api] Check "api health" is down`}
	if got := queuedSubjects(t); !slices.Equal(got, want) {
		t.Errorf("queued notifications:\n got %q\nwant %q", got, want)
	}
}

func TestSnoozedCheckDownRuleSendsNothing(t *testing.T) {
	fixture := setupDispatchDB(t)
	snoozedUntil := time.Now().UTC().Add(time.Hour)
	seedCheckDownRule(t, fixture, &snoozedUntil)
	check := models.SyntheticCheck{Id: 7, ProjectId: fixture.Channel.ProjectId, Name: "api health", ConsecutiveFailures: 1}

	OnCheckStateChange(checkWent(check, models.CheckStatusUp, models.CheckStatusDown))
	OnCheckStateChange(checkWent(check, models.CheckStatusDown, models.CheckStatusUp))

	if got := queuedSubjects(t); len(got) != 0 {
		t.Errorf("a snoozed rule must stay silent, queued %q", got)
	}
}

func TestRecoveryOnlyResetsRecoveredMonitorCooldown(t *testing.T) {
	fixture := setupDispatchDB(t)
	seedCheckDownRule(t, fixture, nil)
	first := models.SyntheticCheck{Id: 7, ProjectId: fixture.Channel.ProjectId, Name: "first"}
	second := models.SyntheticCheck{Id: 8, ProjectId: fixture.Channel.ProjectId, Name: "second"}
	OnCheckStateChange(checkWent(first, models.CheckStatusUp, models.CheckStatusDown))
	OnCheckStateChange(checkWent(second, models.CheckStatusUp, models.CheckStatusDown))
	OnCheckStateChange(checkWent(first, models.CheckStatusDown, models.CheckStatusUp))
	OnCheckStateChange(checkWent(second, models.CheckStatusUnknown, models.CheckStatusDown))
	OnCheckStateChange(checkWent(first, models.CheckStatusUp, models.CheckStatusDown))
	want := []string{`[api] Check "first" is down`, `[api] Check "second" is down`, `[api] Check "first" recovered`, `[api] Check "first" is down`}
	if got := queuedSubjects(t); !slices.Equal(got, want) {
		t.Errorf("queued notifications: got %q, want %q", got, want)
	}
}

func TestSnoozedRecoveryResetsCooldownWithoutNotification(t *testing.T) {
	fixture := setupDispatchDB(t)
	ruleId := seedCheckDownRule(t, fixture, nil)
	check := models.SyntheticCheck{Id: 7, ProjectId: fixture.Channel.ProjectId, Name: "api health"}
	OnCheckStateChange(checkWent(check, models.CheckStatusUp, models.CheckStatusDown))
	setSnooze := func(until *time.Time) {
		t.Helper()
		_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
			return struct{}{}, transactional.NotificationRuleRepository.UpdateSnoozedUntil(tx, ruleId, until)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	until := time.Now().UTC().Add(time.Hour)
	setSnooze(&until)
	OnCheckStateChange(checkWent(check, models.CheckStatusDown, models.CheckStatusUp))
	setSnooze(nil)
	OnCheckStateChange(checkWent(check, models.CheckStatusUp, models.CheckStatusDown))
	want := []string{`[api] Check "api health" is down`, `[api] Check "api health" is down`}
	if got := queuedSubjects(t); !slices.Equal(got, want) {
		t.Errorf("queued notifications: got %q, want %q", got, want)
	}
}
