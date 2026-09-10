//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// approvalChannel records approval requests the way a chat surface would.
type approvalChannel struct {
	requested []uuid.UUID
}

func (c *approvalChannel) Provider() string                 { return "approvals" }
func (c *approvalChannel) Kinds() []string                  { return []string{KindChat} }
func (c *approvalChannel) Fields() []Field                  { return nil }
func (c *approvalChannel) SetupFlow() *SetupFlow            { return nil }
func (c *approvalChannel) Validate(map[string]string) error { return nil }
func (c *approvalChannel) Open(context.Context, *models.Integration, *models.AgentAttempt, *Link) (Link, error) {
	return Link{}, nil
}
func (c *approvalChannel) Post(_ context.Context, _ *models.Integration, thread Link, _ Message) (Link, error) {
	return thread, nil
}
func (c *approvalChannel) Inbound(context.Context, *models.Integration, *http.Request) ([]InboundMessage, error) {
	return nil, nil
}
func (c *approvalChannel) RequestApproval(_ context.Context, in *models.Integration, attempt *models.AgentAttempt) (Link, error) {
	c.requested = append(c.requested, attempt.Id)
	return Link{Provider: "approvals", Kind: models.LinkKindThread, ExternalRef: "approval/" + attempt.Id.String(), IntegrationId: in.Id}, nil
}

func TestStartFromRule(t *testing.T) {
	fx := setup(t)
	approvals := &approvalChannel{}
	RegisterChannel(approvals)
	orgId := *fx.project.OrganizationId
	inTx(t, func(tx *sql.Tx) (int, error) {
		return transactional.IntegrationRepository.Create(tx, &models.Integration{OrganizationId: orgId, Provider: "approvals", Kinds: models.StringSlice{KindChat}, Name: "chat", Config: models.JSONText(`{}`), Enabled: true})
	})
	rule := &models.NotificationRuleWithChannel{Id: 7, ProjectId: fx.project.Id, RuleType: "new_error", Name: "New errors"}
	message := func(hash string) notifications.Message {
		return notifications.Message{Subject: "New error", Body: "boom", DedupToken: hash, ProjectId: fx.project.Id.String()}
	}

	result, err := StartFromRule(json.RawMessage(`{"approval":"auto"}`), rule, message("0123456789abcdef"))
	if err != nil || !result.Started || result.Pending {
		t.Fatalf("auto = %+v, %v", result, err)
	}
	auto := inTx(t, func(tx *sql.Tx) (*models.AgentAttempt, error) {
		return transactional.AgentAttemptRepository.FindActiveBySubject(tx, fx.project.Id, models.SubjectKindTracewayException, "0123456789abcdef")
	})
	if auto == nil || auto.Status != models.AttemptQueued || auto.RequestedBy != nil {
		t.Fatalf("auto attempt = %+v", auto)
	}
	links := inTx(t, func(tx *sql.Tx) ([]*models.AgentLink, error) {
		return transactional.AgentLinkRepository.FindByAttempt(tx, auto.Id)
	})
	if len(links) != 1 || links[0].Provider != ProviderRule || links[0].ExternalRef != "rule:7" {
		t.Fatalf("origin links = %+v", links)
	}

	again, err := StartFromRule(json.RawMessage(`{"approval":"auto"}`), rule, message("0123456789abcdef"))
	if err != nil || !again.Existing || again.Started {
		t.Fatalf("second fire = %+v, %v", again, err)
	}

	result, err = StartFromRule(json.RawMessage(`{"approval":"ask"}`), rule, message("fedcba9876543210"))
	if err != nil || !result.Started || !result.Pending {
		t.Fatalf("ask = %+v, %v", result, err)
	}
	pending := inTx(t, func(tx *sql.Tx) (*models.AgentAttempt, error) {
		return transactional.AgentAttemptRepository.FindActiveBySubject(tx, fx.project.Id, models.SubjectKindTracewayException, "fedcba9876543210")
	})
	if pending == nil || pending.Status != models.AttemptPendingApproval {
		t.Fatalf("ask attempt = %+v", pending)
	}
	if len(approvals.requested) != 1 || approvals.requested[0] != pending.Id {
		t.Fatalf("approval requested for %v", approvals.requested)
	}
	pendingLinks := inTx(t, func(tx *sql.Tx) ([]*models.AgentLink, error) {
		return transactional.AgentLinkRepository.FindByAttempt(tx, pending.Id)
	})
	var thread *models.AgentLink
	for _, link := range pendingLinks {
		if link.Kind == models.LinkKindThread {
			thread = link
		}
	}
	if thread == nil || thread.Provider != "approvals" || thread.IntegrationId == nil {
		t.Fatalf("approval thread = %+v", pendingLinks)
	}

	if _, err := StartFromRule(json.RawMessage(`{"approval":"auto"}`), rule, message("/api/users")); err == nil {
		t.Fatal("a fire without an exception hash must not start an attempt")
	}
	if _, err := StartFromRule(json.RawMessage(`{"approval":"maybe"}`), rule, message("0123456789abcdef")); err == nil {
		t.Fatal("an unknown approval mode must be refused")
	}
	other := message("0123456789abcdef")
	other.ProjectId = uuid.NewString()
	if _, err := StartFromRule(json.RawMessage(`{}`), rule, other); err == nil {
		t.Fatal("a message for another project must be refused")
	}
	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) { return struct{}{}, nil }); err != nil {
		t.Fatal(err)
	}
}
