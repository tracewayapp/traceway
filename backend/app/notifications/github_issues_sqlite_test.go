//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

type githubFixture struct {
	ProjectId uuid.UUID
	Channel   *models.NotificationChannel
	Rule      *models.NotificationRuleWithChannel
}

func setupGitHubDB(t *testing.T) *githubFixture {
	t.Helper()

	dbtest.SetupSQLite(t)
	t.Cleanup(func() {
		cooldowns.mu.Lock()
		cooldowns.fired = make(map[int]time.Time)
		cooldowns.mu.Unlock()
	})

	fixture := &githubFixture{}
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		org, err := transactional.OrganizationRepository.Create(tx, "Acme", "UTC")
		if err != nil {
			return struct{}{}, err
		}
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "api", "gin", org.Id)
		if err != nil {
			return struct{}{}, err
		}
		fixture.ProjectId = project.Id
		now := time.Now().UTC()
		channel := &models.NotificationChannel{
			ProjectId:   project.Id,
			Name:        "Backlog",
			ChannelType: "github",
			Config:      []byte(`{"token":"ghp_x","owner":"acme","repo":"backend"}`),
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
			Id: 7, ProjectId: project.Id, ChannelId: channelId,
			Name: "New errors", RuleType: "new_error", CooldownMinutes: 15,
			ChannelType: "github", ChannelName: "Backlog",
		}
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return fixture
}

func recordIssue(t *testing.T, fixture *githubFixture, issueKey string, number int) int {
	t.Helper()
	id, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.GithubIssueRepository.Create(tx, &models.GithubIssue{
			ProjectId:   fixture.ProjectId,
			ChannelId:   fixture.Channel.Id,
			IssueKey:    issueKey,
			Owner:       "acme",
			Repo:        "backend",
			IssueNumber: number,
			CreatedAt:   time.Now().UTC(),
		})
	})
	if err != nil {
		t.Fatalf("record issue: %v", err)
	}
	return id
}

func closeForArchived(t *testing.T, projectId uuid.UUID, hashes []string) int {
	t.Helper()
	queued, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return CloseGitHubIssuesForArchived(tx, projectId, hashes)
	})
	if err != nil {
		t.Fatalf("close for archived: %v", err)
	}
	return queued
}

func decodeMessage(t *testing.T, row *models.OutboxDelivery) Message {
	t.Helper()
	var msg Message
	if err := json.Unmarshal(row.Message, &msg); err != nil {
		t.Fatalf("decode outbox message: %v", err)
	}
	return msg
}

func TestDispatchTagsGitHubIssueDeliveries(t *testing.T) {
	fixture := setupGitHubDB(t)

	if !dispatch(fixture.Rule, Message{Subject: "New error", Body: "b", DedupToken: "hash-1"}) {
		t.Fatal("dispatch should report a durable enqueue")
	}
	rows := outboxRows(t)
	if len(rows) != 1 {
		t.Fatalf("expected 1 outbox row, got %d", len(rows))
	}
	msg := decodeMessage(t, rows[0])
	if msg.GitHub == nil {
		t.Fatal("github rule delivery should carry the tracking payload")
	}
	if msg.GitHub.IssueKey != "hash-1" {
		t.Errorf("issue key = %q, expected hash-1", msg.GitHub.IssueKey)
	}
	if msg.GitHub.ProjectId != fixture.ProjectId.String() {
		t.Errorf("project id = %q, expected %s", msg.GitHub.ProjectId, fixture.ProjectId)
	}
	if msg.GitHub.ChannelId != fixture.Channel.Id {
		t.Errorf("channel id = %d, expected %d", msg.GitHub.ChannelId, fixture.Channel.Id)
	}
}

func TestDispatchDoesNotTagNonIssueRules(t *testing.T) {
	fixture := setupGitHubDB(t)
	fixture.Rule.RuleType = "error_rate"

	if !dispatch(fixture.Rule, Message{Subject: "Error rate high", DedupToken: "GET /users"}) {
		t.Fatal("dispatch should report a durable enqueue")
	}
	// An issue opened for an endpoint has no exception to archive, so tracking
	// it would leave a row nothing ever closes.
	if msg := decodeMessage(t, outboxRows(t)[0]); msg.GitHub != nil {
		t.Errorf("non-issue rule should not be tracked, got %+v", msg.GitHub)
	}
}

func TestCloseGitHubIssuesForArchivedQueuesClose(t *testing.T) {
	fixture := setupGitHubDB(t)
	recordIssue(t, fixture, "hash-1", 42)

	if queued := closeForArchived(t, fixture.ProjectId, []string{"hash-1"}); queued != 1 {
		t.Fatalf("queued = %d, expected 1", queued)
	}

	rows := outboxRows(t)
	if len(rows) != 1 {
		t.Fatalf("expected 1 outbox row, got %d", len(rows))
	}
	row := rows[0]
	if row.Kind != models.OutboxKindGithubClose || row.AdapterType != "github" {
		t.Errorf("wrong delivery shape: kind=%s adapter=%s", row.Kind, row.AdapterType)
	}
	if string(row.AdapterConfig) != string(fixture.Channel.Config) {
		t.Errorf("close should snapshot the channel config, got %s", row.AdapterConfig)
	}
	msg := decodeMessage(t, row)
	if msg.GitHub == nil || msg.GitHub.CloseNumber != 42 || msg.GitHub.Owner != "acme" || msg.GitHub.Repo != "backend" {
		t.Fatalf("close payload wrong: %+v", msg.GitHub)
	}

	// The issue stops being tracked at enqueue, so archiving again is a no-op
	// rather than a second close.
	if queued := closeForArchived(t, fixture.ProjectId, []string{"hash-1"}); queued != 0 {
		t.Errorf("second archive queued %d closes, expected 0", queued)
	}
	if len(outboxRows(t)) != 1 {
		t.Errorf("second archive enqueued another delivery")
	}

	open, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.GithubIssue, error) {
		return transactional.GithubIssueRepository.FindOpenByIssueKey(tx, fixture.ProjectId, "hash-1")
	})
	if err != nil {
		t.Fatalf("reload issue: %v", err)
	}
	if len(open) != 0 {
		t.Errorf("issue should no longer be open, got %+v", open[0])
	}
}

func TestCloseGitHubIssuesIgnoresOtherProjectsAndHashes(t *testing.T) {
	fixture := setupGitHubDB(t)
	recordIssue(t, fixture, "hash-1", 42)

	if queued := closeForArchived(t, fixture.ProjectId, []string{"hash-2"}); queued != 0 {
		t.Errorf("unrelated hash queued %d closes", queued)
	}
	if queued := closeForArchived(t, uuid.New(), []string{"hash-1"}); queued != 0 {
		t.Errorf("another project queued %d closes", queued)
	}
}

func TestCloseGitHubIssuesSkipsRetypedChannel(t *testing.T) {
	fixture := setupGitHubDB(t)
	recordIssue(t, fixture, "hash-1", 42)

	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		fixture.Channel.ChannelType = "slack"
		fixture.Channel.Config = []byte(`{"webhookUrl":"https://hooks.example.com/x"}`)
		return struct{}{}, transactional.NotificationChannelRepository.Update(tx, fixture.Channel)
	})
	if err != nil {
		t.Fatalf("retype channel: %v", err)
	}

	if queued := closeForArchived(t, fixture.ProjectId, []string{"hash-1"}); queued != 0 {
		t.Errorf("retyped channel queued %d closes", queued)
	}
	if len(outboxRows(t)) != 0 {
		t.Error("a channel that is no longer GitHub must not be sent to")
	}
	if queued := closeForArchived(t, fixture.ProjectId, []string{"hash-1"}); queued != 0 {
		t.Error("the untrackable issue should have been dropped, not retried")
	}
}

func TestSendGitHubIssueRecordsCreatedIssue(t *testing.T) {
	fixture := setupGitHubDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"number":99}`))
	}))
	defer server.Close()

	adapter := &GitHubAdapter{Token: "ghp_x", Owner: "acme", Repo: "backend", baseURL: server.URL}
	msg := Message{
		Subject: "New error",
		GitHub: &models.NotificationGitHub{
			IssueKey:  "hash-1",
			ProjectId: fixture.ProjectId.String(),
			ChannelId: fixture.Channel.Id,
		},
	}
	if err := sendGitHubIssue(context.Background(), adapter, msg); err != nil {
		t.Fatalf("send: %v", err)
	}

	open, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.GithubIssue, error) {
		return transactional.GithubIssueRepository.FindOpenByIssueKey(tx, fixture.ProjectId, "hash-1")
	})
	if err != nil {
		t.Fatalf("load issues: %v", err)
	}
	if len(open) != 1 {
		t.Fatalf("expected the created issue to be tracked, got %d rows", len(open))
	}
	if open[0].IssueNumber != 99 || open[0].Owner != "acme" || open[0].Repo != "backend" {
		t.Errorf("recorded issue wrong: %+v", open[0])
	}
}

func TestSendGitHubIssueSkipsRecordingUntrackedDeliveries(t *testing.T) {
	setupGitHubDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"number":99}`))
	}))
	defer server.Close()

	adapter := &GitHubAdapter{Token: "ghp_x", Owner: "acme", Repo: "backend", baseURL: server.URL}
	if err := sendGitHubIssue(context.Background(), adapter, Message{Subject: "Error rate high"}); err != nil {
		t.Fatalf("send: %v", err)
	}

	count, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		var n int
		return n, tx.QueryRow("SELECT COUNT(*) FROM github_issues").Scan(&n)
	})
	if err != nil {
		t.Fatalf("count issues: %v", err)
	}
	if count != 0 {
		t.Errorf("untracked delivery recorded %d rows", count)
	}
}
