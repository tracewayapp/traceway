//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb && !telemetry_firebolt

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

// TestCheckDownOpensPageAndRecoveryAutoResolves exercises the full synthetic
// check alerting loop through an escalation channel: down opens a page,
// recovery resolves it with no resolving user.
func TestCheckDownOpensPageAndRecoveryAutoResolves(t *testing.T) {
	fixture := setupEscalatorDB(t)
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
			ProjectId: fixture.ProjectId,
			ChannelId: channelId,
			Name:      "Checks down",
			RuleType:  "check_down",
			Config:    []byte(`{}`),
			Enabled:   true,
			CreatedAt: now,
			UpdatedAt: now,
		})
	})
	if err != nil {
		t.Fatalf("seed rule: %v", err)
	}

	check := models.SyntheticCheck{
		Id:                  7,
		ProjectId:           fixture.ProjectId,
		Name:                "API health",
		CheckType:           models.CheckTypeHttp,
		ConsecutiveFailures: 2,
	}

	// Down transition opens a page under the rule-scoped dedup key.
	notifications.OnCheckStateChange(synthetics.StateTransition{
		Check:    check,
		From:     models.CheckStatusUp,
		To:       models.CheckStatusDown,
		ErrorMsg: "status code 503",
		At:       time.Now().UTC(),
	})

	dedupKey := fmt.Sprintf("%d|check:%d", ruleId, check.Id)
	page := findPageByDedupKey(t, dedupKey)
	if page == nil {
		t.Fatal("expected the down transition to open a page")
	}
	if page.Status != models.PageStatusOpen || page.RuleType != "check_down" {
		t.Fatalf("unexpected page %+v", page)
	}

	// Recovery must resolve the page (system resolve), never open another.
	notifications.OnCheckStateChange(synthetics.StateTransition{
		Check: check,
		From:  models.CheckStatusDown,
		To:    models.CheckStatusUp,
		At:    time.Now().UTC(),
	})

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
