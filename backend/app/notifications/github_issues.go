package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/outbox"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

// sendGitHubIssue performs one GitHub delivery: a close when the message names
// an issue to close, otherwise a create whose issue number is remembered so
// archiving the exception can close it later.
func sendGitHubIssue(ctx context.Context, adapter *GitHubAdapter, msg Message) error {
	if msg.GitHub != nil && msg.GitHub.CloseNumber > 0 {
		return adapter.CloseIssue(ctx, msg.GitHub.Owner, msg.GitHub.Repo, msg.GitHub.CloseNumber, msg.Body)
	}
	number, err := adapter.CreateIssue(ctx, msg)
	if err != nil {
		return err
	}
	recordGitHubIssue(adapter, msg, number)
	return nil
}

// recordGitHubIssue remembers a created issue so archiving the exception it
// tracks closes it. The issue already exists, so a failure here is reported and
// swallowed: failing the delivery would retry the create and open a duplicate.
func recordGitHubIssue(adapter *GitHubAdapter, msg Message, number int) {
	if msg.GitHub == nil || msg.GitHub.IssueKey == "" || number == 0 {
		return
	}
	projectId, err := uuid.Parse(msg.GitHub.ProjectId)
	if err != nil {
		traceway.CaptureException(fmt.Errorf("GitHub issue %s/%s#%d has an unusable project id %q: %w", adapter.Owner, adapter.Repo, number, msg.GitHub.ProjectId, err))
		return
	}
	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.GithubIssueRepository.Create(tx, &models.GithubIssue{
			ProjectId:   projectId,
			ChannelId:   msg.GitHub.ChannelId,
			IssueKey:    msg.GitHub.IssueKey,
			Owner:       adapter.Owner,
			Repo:        adapter.Repo,
			IssueNumber: number,
			CreatedAt:   time.Now().UTC(),
		})
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to record GitHub issue %s/%s#%d for exception %s: %w", adapter.Owner, adapter.Repo, number, msg.GitHub.IssueKey, err))
	}
}

// trackGitHubIssue marks a rule delivery so the issue it opens is remembered.
// Only issue-shaped rules qualify: an issue opened for a latency or metric rule
// has no exception to archive, so nothing would ever close it.
func trackGitHubIssue(msg *Message, rule *models.NotificationRuleWithChannel, channelId int) {
	if !models.IsIssueRuleType(rule.RuleType) || msg.DedupToken == "" {
		return
	}
	msg.GitHub = &models.NotificationGitHub{
		IssueKey:  msg.DedupToken,
		ProjectId: rule.ProjectId.String(),
		ChannelId: channelId,
	}
}

// CloseGitHubIssuesForArchived queues a close for every GitHub issue still open
// for the given exception hashes, in the caller's transaction. That commit is
// the durable promise, so the issues stop being tracked here rather than when
// the close lands: the outbox already retries the send, and a row left open
// would queue a second close on the next archive. Callers Wake the outbox after
// their commit. Returns how many closes were queued.
func CloseGitHubIssuesForArchived(tx *sql.Tx, projectId uuid.UUID, hashes []string) (int, error) {
	queued := 0
	now := time.Now().UTC()
	for _, hash := range hashes {
		issues, err := transactional.GithubIssueRepository.FindOpenByIssueKey(tx, projectId, hash)
		if err != nil {
			return queued, err
		}
		for _, issue := range issues {
			closed, err := transactional.GithubIssueRepository.MarkClosed(tx, issue.Id, now)
			if err != nil {
				return queued, err
			}
			if !closed {
				continue
			}
			channel, err := transactional.NotificationChannelRepository.FindById(tx, issue.ChannelId)
			if err != nil {
				return queued, err
			}
			// A channel retyped away from GitHub has no credentials left to
			// close with; stop tracking rather than queueing a send that can
			// only fail. A disabled one still closes: this is one-shot cleanup
			// of an issue that channel itself opened, not a new notification.
			if channel == nil || channel.ChannelType != "github" {
				continue
			}
			if _, err := outbox.Enqueue(tx, outbox.Delivery{
				Kind:          models.OutboxKindGithubClose,
				AdapterType:   channel.ChannelType,
				AdapterConfig: json.RawMessage(channel.Config),
				Message:       buildGitHubCloseMessage(issue),
				ProjectId:     &projectId,
				ChannelName:   channel.Name,
			}); err != nil {
				return queued, err
			}
			queued++
		}
	}
	return queued, nil
}

func buildGitHubCloseMessage(issue *models.GithubIssue) Message {
	var body strings.Builder
	body.WriteString("Closed automatically by Traceway: the issue this tracks was archived.")
	// Unlike dashboardURL's normal fallback (a bare relative path), the link is
	// left out entirely when unset: the comment it goes into is public, and a
	// relative or localhost path there helps nobody outside Traceway.
	if config.Config.PublicBaseURL() != "" {
		fmt.Fprintf(&body, "\n\n%s", dashboardURL("/issues/"+issue.IssueKey))
	}
	return Message{
		Subject:  "Issue archived in Traceway",
		Body:     body.String(),
		Severity: SeverityInfo,
		URL:      "/issues/" + issue.IssueKey,
		GitHub: &models.NotificationGitHub{
			IssueKey:    issue.IssueKey,
			CloseNumber: issue.IssueNumber,
			Owner:       issue.Owner,
			Repo:        issue.Repo,
		},
	}
}
