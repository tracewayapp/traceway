//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package controllers

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/storage"
)

// TestReportExternalRecordsACIRun drives the CI bridge: the handler manages
// its own transactions, so the fixture is committed first and the request
// context carries none.
func TestReportExternalRecordsACIRun(t *testing.T) {
	setupAgentControllerDB(t)
	local, err := storage.NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	prev := storage.Store
	storage.Store = local
	t.Cleanup(func() { storage.Store = prev })

	var ownerId int
	var projectId uuid.UUID
	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		var orgId int
		ownerId, orgId = createSetupTestAccount(t, tx, "owner@example.com", "owner")
		projectId = createChannelTestProject(t, tx, orgId)
		return struct{}{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	hash := "0123456789abcdef"

	c, rec := agentRequest(t, nil, projectId, ownerId, "POST", "/agent/attempts/report", `{"hash":"`+hash+`","status":"fixed","branch":"traceway/fix-1","pullRequestUrl":"https://github.com/acme/app/pull/12","report":"Fixed the nil map write."}`)
	AgentAttemptController.ReportExternal(c)
	if rec.Code != 201 {
		t.Fatalf("report: %d %s", rec.Code, rec.Body.String())
	}
	attempt, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentAttempt, error) {
		return transactional.AgentAttemptRepository.FindActiveBySubject(tx, projectId, models.SubjectKindTracewayException, hash)
	})
	if err != nil || attempt == nil {
		t.Fatalf("attempt = %v, %v", attempt, err)
	}
	if attempt.Status != models.AttemptAwaitingReview || attempt.Executor != ExecutorCI || !strings.HasPrefix(attempt.ClaimedBy, ExecutorCI+"/") || attempt.FixBranch != "traceway/fix-1" || attempt.ReportKey == "" {
		t.Fatalf("attempt = %+v", attempt)
	}
	links, _ := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentLink, error) {
		return transactional.AgentLinkRepository.FindByAttempt(tx, attempt.Id)
	})
	var pr *models.AgentLink
	for _, link := range links {
		if link.Kind == models.LinkKindPR {
			pr = link
		}
	}
	if pr == nil || pr.ExternalRef != "acme/app#12" || pr.Provider != "github" {
		t.Fatalf("links = %+v", links)
	}
	report, _ := storage.Store.Read(c.Request.Context(), agent.BlobKey(attempt.Id, agent.BlobReport))
	if !strings.Contains(string(report), "nil map write") {
		t.Fatalf("report blob = %q", report)
	}

	c, rec = agentRequest(t, nil, projectId, ownerId, "POST", "/agent/attempts/report", `{"hash":"`+hash+`","status":"analysis","report":"still looking"}`)
	AgentAttemptController.ReportExternal(c)
	if rec.Code != 409 {
		t.Fatalf("a second report while the first waits for review = %d %s", rec.Code, rec.Body.String())
	}

	c, rec = agentRequest(t, nil, projectId, ownerId, "POST", "/agent/attempts/report", `{"hash":"fedcba9876543210","status":"analysis","report":"No safe fix: the cause is upstream."}`)
	AgentAttemptController.ReportExternal(c)
	if rec.Code != 201 {
		t.Fatalf("analysis report: %d %s", rec.Code, rec.Body.String())
	}
	analyzed, _ := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentAttempt, error) {
		return transactional.AgentAttemptRepository.FindBySubject(tx, projectId, models.SubjectKindTracewayException, "fedcba9876543210")
	})
	if len(analyzed) != 1 || analyzed[0].Status != models.AttemptAnalyzed {
		t.Fatalf("analysis attempt = %+v", analyzed)
	}
	messages, _ := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentMessage, error) {
		return transactional.AgentMessageRepository.ListAfter(tx, analyzed[0].Id, 0, "", 10)
	})
	if len(messages) != 1 || messages[0].Kind != models.MessageKindFinding {
		t.Fatalf("messages = %+v", messages)
	}

	for name, body := range map[string]string{
		"bad hash":    `{"hash":"nope","status":"fixed","pullRequestUrl":"https://github.com/acme/app/pull/1","report":"r"}`,
		"bad status":  `{"hash":"0123456789abcde0","status":"done","report":"r"}`,
		"fixed no pr": `{"hash":"0123456789abcde0","status":"fixed","report":"r"}`,
		"no report":   `{"hash":"0123456789abcde0","status":"analysis"}`,
	} {
		c, rec = agentRequest(t, nil, projectId, ownerId, "POST", "/agent/attempts/report", body)
		AgentAttemptController.ReportExternal(c)
		if rec.Code != 422 {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
}

func TestApproveAnswers409WhenNotPending(t *testing.T) {
	setupAgentControllerDB(t)
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.Rollback() })
	ownerId, orgId := createSetupTestAccount(t, tx, "owner@example.com", "owner")
	projectId := createChannelTestProject(t, tx, orgId)
	project, err := transactional.ProjectRepository.FindById(tx, projectId)
	if err != nil {
		t.Fatal(err)
	}
	started, err := agent.StartAttempt(tx, project, agent.Subject{Kind: models.SubjectKindTracewayException, Ref: "0123456789abcdef", ProjectId: projectId}, agent.StartOptions{RequireApproval: true, RequestedBy: &ownerId})
	if err != nil {
		t.Fatal(err)
	}
	path := "/agent/attempts/" + started.Attempt.Id.String() + "/approve"
	c, rec := agentRequest(t, tx, projectId, ownerId, "POST", path, "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: started.Attempt.Id.String()})
	AgentAttemptController.Approve(c)
	if rec.Code != 200 {
		t.Fatalf("approve: %d %s", rec.Code, rec.Body.String())
	}
	c, rec = agentRequest(t, tx, projectId, ownerId, "POST", path, "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: started.Attempt.Id.String()})
	AgentAttemptController.Approve(c)
	if rec.Code != 409 {
		t.Fatalf("second approve: %d %s", rec.Code, rec.Body.String())
	}
}
