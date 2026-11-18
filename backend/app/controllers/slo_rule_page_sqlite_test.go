//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package controllers

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/dbtest"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
)

// seedEscalationRule creates a rule on an escalation channel with one open page
// per dedup token, then lets seed record the rule's firing state.
func seedEscalationRule(t *testing.T, ruleType, config string, tokens []string, seed func(tx *sql.Tx, rule *models.NotificationRule) error) (*models.NotificationRule, []int) {
	t.Helper()
	dbtest.SetupSQLite(t)
	orgId := createTestOrg(t, "org")
	now := time.Now().UTC()
	var rule *models.NotificationRule
	var pageIds []int
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "api", "gin", orgId)
		if err != nil {
			return struct{}{}, err
		}
		channel := &models.NotificationChannel{ProjectId: project.Id, Name: "Pager", ChannelType: "escalation", Config: []byte(`{"policyId":1}`), Enabled: true, CreatedAt: now, UpdatedAt: now}
		channelId, err := transactional.NotificationChannelRepository.Create(tx, channel)
		if err != nil {
			return struct{}{}, err
		}
		rule = &models.NotificationRule{ProjectId: project.Id, ChannelId: channelId, Name: "alert", RuleType: ruleType, Config: []byte(config), Enabled: true, CooldownMinutes: 15, CreatedAt: now, UpdatedAt: now}
		if rule.Id, err = transactional.NotificationRuleRepository.Create(tx, rule); err != nil {
			return struct{}{}, err
		}
		for _, token := range tokens {
			page := &models.Page{OrganizationId: orgId, ProjectId: project.Id, RuleId: &rule.Id, RuleName: rule.Name, RuleType: rule.RuleType, Subject: token, Severity: "warning", Urgency: "high", Status: "open",
				DedupKey: fmt.Sprintf("%d|%s", rule.Id, token), EventCount: 1, LastEventAt: now, PolicySnapshot: models.JSONText(`{}`), CreatedAt: now, UpdatedAt: now}
			pageId, err := transactional.PageRepository.Create(tx, page)
			if err != nil {
				return struct{}{}, err
			}
			pageIds = append(pageIds, pageId)
		}
		return struct{}{}, seed(tx, rule)
	})
	if err != nil {
		t.Fatal(err)
	}
	return rule, pageIds
}

func resolveRulePagesAndCheck(t *testing.T, rule *models.NotificationRule, pageIds []int) {
	t.Helper()
	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, resolveRulePages(tx, rule)
	}); err != nil {
		t.Fatal(err)
	}
	for _, pageId := range pageIds {
		page, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Page, error) {
			return transactional.PageRepository.FindById(tx, pageId)
		})
		if err != nil {
			t.Fatal(err)
		}
		if page == nil || page.Status != "resolved" {
			t.Errorf("the page the alert opened must be resolved: %+v", page)
		}
	}
}

func TestResolveSLORulePageClosesTheAlertPage(t *testing.T) {
	rule, pageIds := seedEscalationRule(t, "slo_burn_rate", `{"sloId":7}`, []string{"slo:7"}, func(tx *sql.Tx, rule *models.NotificationRule) error {
		_, err := transactional.NotificationRuleStateRepository.Transition(tx, rule.Id, true, time.Now().UTC())
		return err
	})

	resolveRulePagesAndCheck(t, rule, pageIds)

	state, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.NotificationRuleState, error) {
		return transactional.NotificationRuleStateRepository.Find(tx, rule.Id)
	})
	if err != nil {
		t.Fatal(err)
	}
	if state != nil {
		t.Fatalf("the firing state must be forgotten: %+v", state)
	}
}

func TestResolveRulePagesClosesTheSeriesPages(t *testing.T) {
	tokens := []string{"radius.freshness{site=RUINS}", "radius.freshness:overflow"}
	rule, pageIds := seedEscalationRule(t, "metric_threshold", `{"metricName":"radius.freshness","groupBy":["site"]}`, tokens, func(tx *sql.Tx, rule *models.NotificationRule) error {
		for _, token := range tokens {
			if _, err := transactional.NotificationRuleFiringRepository.Insert(tx, rule.Id, token, time.Now()); err != nil {
				return err
			}
		}
		return nil
	})

	resolveRulePagesAndCheck(t, rule, pageIds)

	firing, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.NotificationRuleFiring, error) {
		return transactional.NotificationRuleFiringRepository.FindByRule(tx, rule.Id)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(firing) != 0 {
		t.Fatalf("the open alerts must be forgotten: %+v", firing)
	}
}

func TestUpgradeRecoversExistingMetricPages(t *testing.T) {
	const short = "latency{site=west}"
	long := "latency{site=" + strings.Repeat("x", 320) + "}"
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(long)))[:32]
	tokens := []string{short, digest, "latency:overflow", "latency{site=closed}"}
	rule, pages := seedEscalationRule(t, "metric_threshold", `{"metricName":"latency","groupBy":["site"]}`, tokens, func(*sql.Tx, *models.NotificationRule) error { return nil })
	if _, err := db.DB.Exec("UPDATE pages SET body = ?, status = 'acknowledged' WHERE id = ?", "The metric "+long+" has a max of 9.00 over the last 5 minutes which violates the threshold gt 8.00.", pages[1]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("UPDATE pages SET status = 'resolved' WHERE id = ?", pages[3]); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../migrations/sqlite/0094_seed_grouped_metric_pages.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM notification_rule_firing WHERE rule_id = ?", rule.Id).Scan(&count); err != nil || count != 3 {
		t.Fatalf("seeded %d open series, err=%v", count, err)
	}
	resolveRulePagesAndCheck(t, rule, pages[:3])
}
