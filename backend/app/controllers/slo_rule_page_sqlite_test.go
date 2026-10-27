//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package controllers

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

func TestResolveSLORulePageClosesTheAlertPage(t *testing.T) {
	dbtest.SetupSQLite(t)
	orgId := createTestOrg(t, "org")
	now := time.Now().UTC()
	type seeded struct {
		rule   *models.NotificationRule
		pageId int
	}
	s, err := db.ExecuteTransaction(func(tx *sql.Tx) (seeded, error) {
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "api", "gin", orgId)
		if err != nil {
			return seeded{}, err
		}
		channel := &models.NotificationChannel{ProjectId: project.Id, Name: "Pager", ChannelType: "escalation", Config: []byte(`{"policyId":1}`), Enabled: true, CreatedAt: now, UpdatedAt: now}
		channelId, err := transactional.NotificationChannelRepository.Create(tx, channel)
		if err != nil {
			return seeded{}, err
		}
		rule := &models.NotificationRule{ProjectId: project.Id, ChannelId: channelId, Name: "burn", RuleType: "slo_burn_rate", Config: []byte(`{"sloId":7}`), Enabled: true, CooldownMinutes: 15, CreatedAt: now, UpdatedAt: now}
		if rule.Id, err = transactional.NotificationRuleRepository.Create(tx, rule); err != nil {
			return seeded{}, err
		}
		page := &models.Page{OrganizationId: orgId, ProjectId: project.Id, RuleId: &rule.Id, RuleName: rule.Name, RuleType: rule.RuleType, Subject: "burn", Severity: "warning", Urgency: "high", Status: "open",
			DedupKey: fmt.Sprintf("%d|slo:7", rule.Id), EventCount: 1, LastEventAt: now, PolicySnapshot: models.JSONText(`{}`), CreatedAt: now, UpdatedAt: now}
		pageId, err := transactional.PageRepository.Create(tx, page)
		if err != nil {
			return seeded{}, err
		}
		_, err = transactional.NotificationRuleStateRepository.Transition(tx, rule.Id, true, now)
		return seeded{rule, pageId}, err
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, resolveSLORulePage(tx, s.rule)
	}); err != nil {
		t.Fatal(err)
	}

	page, state, err := func() (*models.Page, *models.NotificationRuleState, error) {
		var page *models.Page
		state, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.NotificationRuleState, error) {
			var err error
			if page, err = transactional.PageRepository.FindById(tx, s.pageId); err != nil {
				return nil, err
			}
			return transactional.NotificationRuleStateRepository.Find(tx, s.rule.Id)
		})
		return page, state, err
	}()
	if err != nil {
		t.Fatal(err)
	}
	if page == nil || page.Status != "resolved" {
		t.Fatalf("the page the alert opened must be resolved: %+v", page)
	}
	if state != nil {
		t.Fatalf("the firing state must be forgotten: %+v", state)
	}
}
