//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package agentrunner

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/agents"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"strings"
	"testing"
	"time"
)

func TestRegressionClaudeSessionPersists(t *testing.T) {
	h := setupHarness(t, "")
	attempt := h.start("0123456789abcdef")
	parser := (agents.ClaudeCode{}).NewParser()
	events, err := parser.Parse([]byte(`{"type":"result","subtype":"success","session_id":"review-session","result":"STATUS: question\nSUBJECT: 0123456789abcdef\nWhich version?"}`))
	if err != nil {
		t.Fatal(err)
	}
	session, err := db.ExecuteTransaction(func(tx *sql.Tx) (string, error) {
		for _, event := range events {
			payload, err := eventPayload(event)
			if err != nil {
				return "", err
			}
			if _, err := agent.AppendEvent(tx, attempt.Id, event.Kind, payload, time.Now().UTC()); err != nil {
				return "", err
			}
		}
		return latestSessionId(tx, attempt.Id)
	})
	if err != nil {
		t.Fatal(err)
	}
	if session != "review-session" {
		t.Errorf("persisted session = %q; real Claude result loses session_id", session)
	}
}

func TestRegressionStaleClaimCannotMutateReplacement(t *testing.T) {
	h := setupHarness(t, "")
	attempt := h.start("0123456789abcdef")
	var oldClaim, newClaim string
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		now := time.Now().UTC()
		first, err := agent.Claim(tx, "worker", 1, now)
		if err != nil {
			return struct{}{}, err
		}
		oldClaim = first[0].ClaimedBy
		if _, err := agent.ReclaimStale(tx, now.Add(3*time.Minute)); err != nil {
			return struct{}{}, err
		}
		second, err := agent.Claim(tx, "worker", 1, now.Add(3*time.Minute))
		if err != nil {
			return struct{}{}, err
		}
		newClaim = second[0].ClaimedBy
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if oldClaim == newClaim {
		t.Fatal("reused claim identity")
	}
	stale := Local{Executor: "worker", ClaimID: oldClaim}
	checks := map[string]func() error{
		"transition": func() error { return stale.Transition(context.Background(), attempt.Id, models.AttemptPreparing) },
		"events": func() error {
			return stale.Events(context.Background(), attempt.Id, []agents.Event{{Kind: "stale", Text: "must not persist"}})
		},
		"finding": func() error { return stale.Finding(context.Background(), attempt.Id, "stale") },
		"blob": func() error {
			return stale.Blob(context.Background(), attempt.Id, agent.BlobReport, []byte("stale"), false)
		},
		"result": func() error {
			return stale.Finish(context.Background(), attempt.Id, Outcome{Status: agents.StatusError, Error: "stale"})
		},
		"token": func() error { _, err := stale.RenewRunToken(context.Background(), attempt.Id); return err },
	}
	for name, check := range checks {
		if err := check(); !errors.Is(err, ErrClaimLost) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if got := h.reload(attempt).Status; got != models.AttemptClaimed {
		t.Fatal(got)
	}
	for _, kind := range h.eventKinds(attempt) {
		if kind == "stale" {
			t.Fatal("stale event committed")
		}
	}
	if got := h.blob(attempt, agent.BlobReport); got != "" {
		t.Fatal("stale blob written")
	}
	if err := (Local{Executor: "worker", ClaimID: newClaim}).Transition(context.Background(), attempt.Id, models.AttemptPreparing); err != nil {
		t.Fatal(err)
	}
}

func TestRegressionRemoteAvailabilityWithinTransaction(t *testing.T) {
	setupHarness(t, "")
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	done := make(chan struct{})
	go func() { (RemoteExecutor{}).Available(context.Background(), tx); close(done) }()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		tx.Rollback()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("availability did not recover after releasing the outer transaction")
		}
		t.Error("remote availability blocked until the outer transaction was released; Preflight holds that transaction")
	}
}

func TestRegressionCodeHostEventCannotCloseOtherOrganization(t *testing.T) {
	h := setupHarness(t, "")
	h.scriptAgent(fixedReport)
	h.start("0123456789abcdef")
	attempt := h.claimAndRun()
	if attempt.Status != models.AttemptAwaitingReview {
		t.Fatal(attempt.Status, attempt.Error)
	}
	wrongIntegration := &models.Integration{Id: 999999, OrganizationId: attempt.OrganizationId + 1, Provider: "fakehost", Enabled: true}
	_, err := agent.Dispatch(context.Background(), "fakehost", wrongIntegration, agent.Inbound{Events: []agent.CodeHostEvent{{Kind: agent.CodeHostEventPullRequestClosed, Link: agent.Link{Provider: "fakehost", Kind: models.LinkKindPR, ExternalRef: "acme/app#1", IntegrationId: wrongIntegration.Id}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := h.reload(attempt).Status; got != models.AttemptAwaitingReview {
		t.Errorf("another organization's integration changed attempt to %s", got)
	}
}

func TestRegressionReadonlyReplyCannotResume(t *testing.T) {
	h := setupHarness(t, "")
	attempt := h.start("0123456789abcdef")
	var integration *models.Integration
	reader, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		repo, err := transactional.RepositoryRepository.FindByProject(tx, attempt.ProjectId)
		if err != nil {
			return 0, err
		}
		integration, err = transactional.IntegrationRepository.FindById(tx, *repo.IntegrationId)
		if err != nil {
			return 0, err
		}
		u, err := transactional.UserRepository.Create(tx, "reader@example.com", "Reader", "hashed")
		if err != nil {
			return 0, err
		}
		if _, err := transactional.OrganizationRepository.AddUser(tx, attempt.OrganizationId, u.Id, "readonly"); err != nil {
			return 0, err
		}
		if _, err := agent.Claim(tx, "test", 1, time.Now().UTC()); err != nil {
			return 0, err
		}
		for _, status := range []string{models.AttemptPreparing, models.AttemptRunning, models.AttemptNeedsInput} {
			if err := agent.Transition(tx, attempt.Id, status, time.Now().UTC()); err != nil {
				return 0, err
			}
		}
		_, err = agent.RecordLink(tx, attempt.Id, agent.Link{Provider: "fakehost", Kind: models.LinkKindThread, ExternalRef: "review-thread", IntegrationId: integration.Id}, time.Now().UTC())
		return u.Id, err
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = agent.Dispatch(context.Background(), "fakehost", integration, agent.Inbound{Messages: []agent.InboundMessage{{Thread: agent.Link{Kind: models.LinkKindThread, ExternalRef: "review-thread"}, Author: agent.Identity{UserId: reader}, Body: "Please continue", ExternalRef: "review-reply"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := h.reload(attempt).Status; got != models.AttemptNeedsInput {
		t.Errorf("readonly reply resumed attempt to %s", got)
	}
}

func TestRegressionFollowupReusesPullRequest(t *testing.T) {
	h := setupHarness(t, "")
	h.scriptAgent(fixedReport)
	h.start("0123456789abcdef")
	attempt := h.claimAndRun()
	if attempt.Status != models.AttemptAwaitingReview {
		t.Fatal(attempt.Status, attempt.Error)
	}
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentMessage, error) {
		return agent.Post(tx, attempt, agent.Message{Provider: "web", Direction: models.MessageInbound, Kind: models.MessageKindAnswer, Body: "Please add logging", Author: &agent.Identity{UserId: h.userId}}, nil, "", time.Now().UTC())
	})
	if err != nil {
		t.Fatal(err)
	}
	h.scriptAgent(strings.ReplaceAll(fixedReport, "func main() {}", "func main() { println(1) }"))
	resumed := h.claimAndRun()
	if resumed.Id != attempt.Id || resumed.Status != models.AttemptAwaitingReview {
		t.Fatal(resumed.Status, resumed.Error)
	}
	if len(h.host.pulls) != 1 || resumed.FixBranch != attempt.FixBranch {
		t.Fatal("followup created another pull request")
	}
}

func TestRegressionUsageAccumulatesOncePerCompletedClaim(t *testing.T) {
	h := setupHarness(t, "")
	attempt := h.start("0123456789abcdef")
	for session := 1; session <= 2; session++ {
		if session == 2 {
			current := h.reload(attempt)
			_, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentMessage, error) {
				return agent.Post(tx, current, agent.Message{Provider: "web", Direction: models.MessageInbound, Kind: models.MessageKindAnswer, Body: "continue"}, nil, "", time.Now().UTC())
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		claims, err := h.runner.Claimer.Claim(context.Background(), 1)
		if err != nil {
			t.Fatal(err)
		}
		local := Local{Executor: claims[0].Executor, ClaimID: claims[0].ClaimedBy}
		for _, status := range []string{models.AttemptPreparing, models.AttemptRunning} {
			if err := local.Transition(context.Background(), attempt.Id, status); err != nil {
				t.Fatal(err)
			}
		}
		outcome := Outcome{Status: agents.StatusQuestion, Report: "question", Usage: agents.Usage{CostUSD: float64(session), InputTokens: 10, Turns: 1}}
		if err := local.Finish(context.Background(), attempt.Id, outcome); err != nil {
			t.Fatal(err)
		}
		if err := local.Finish(context.Background(), attempt.Id, outcome); !errors.Is(err, ErrClaimLost) {
			t.Fatalf("repeated completion: %v", err)
		}
	}
	row := h.reload(attempt)
	if row.CostUSD != 3 || row.InputTokens != 20 || row.Turns != 2 {
		t.Fatalf("accumulated usage: %+v", row)
	}
}

func TestRegressionRunTokenRefreshStaysClaimScoped(t *testing.T) {
	h := setupHarness(t, "")
	h.start("0123456789abcdef")
	rows, err := h.runner.Claimer.Claim(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	local := Local{Executor: rows[0].Executor, ClaimID: rows[0].ClaimedBy}
	token, err := local.RenewRunToken(context.Background(), rows[0].Id)
	if err != nil || token == nil || !token.ExpiresAt.After(time.Now()) {
		t.Fatalf("token renewal: %v", err)
	}
	if _, err := local.RenewRunToken(context.Background(), uuid.New()); !errors.Is(err, ErrClaimLost) {
		t.Fatalf("foreign attempt renewal: %v", err)
	}
}
