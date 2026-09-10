package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

var exceptionHashRe = regexp.MustCompile(`^[0-9a-f]{16}$`)

// ProviderRule marks attempts a notification rule started, so the event
// stream and the thread say nobody clicked.
const ProviderRule = "rule"

// StartFromRule is the notifications package's AttemptStarter: a rule on
// new errors or regressions that targets an agent channel turns a fire
// into an attempt, queued or waiting for approval as the channel says. It
// runs in its own transaction and asks the chat surfaces for approval
// afterwards, since that talks to providers.
func StartFromRule(channelConfig json.RawMessage, rule *models.NotificationRuleWithChannel, msg notifications.Message) (notifications.AttemptStartResult, error) {
	cfg, problem := notifications.ParseAgentChannelConfig(channelConfig)
	if problem != "" {
		return notifications.AttemptStartResult{}, errors.New(problem)
	}
	if !exceptionHashRe.MatchString(msg.DedupToken) {
		return notifications.AttemptStartResult{}, fmt.Errorf("the rule fired on %q, which is not an exception hash", msg.DedupToken)
	}
	projectId, err := uuid.Parse(msg.ProjectId)
	if err != nil || projectId != rule.ProjectId {
		return notifications.AttemptStartResult{}, errors.New("the message names another project than the rule")
	}
	started, err := db.ExecuteTransaction(func(tx *sql.Tx) (*StartResult, error) {
		project, err := transactional.ProjectRepository.FindById(tx, projectId)
		if err != nil {
			return nil, err
		}
		if project == nil {
			return nil, errors.New("the rule's project is gone")
		}
		origin := Link{Provider: ProviderRule, Kind: models.LinkKindAlert, ExternalRef: fmt.Sprintf("rule:%d", rule.Id)}
		subject := Subject{Kind: models.SubjectKindTracewayException, Ref: msg.DedupToken, ProjectId: project.Id}
		return StartAttempt(tx, project, subject, StartOptions{ProfileId: cfg.ProfileId, RequireApproval: cfg.Approval == notifications.AgentApprovalAsk, Origin: &origin})
	})
	if err != nil {
		return notifications.AttemptStartResult{}, err
	}
	if started.Existing {
		return notifications.AttemptStartResult{Existing: true}, nil
	}
	if started.Attempt.Status == models.AttemptPendingApproval {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := RequestApproval(ctx, started.Attempt); err != nil {
			traceway.CaptureException(fmt.Errorf("agent: request approval for attempt %s: %w", started.Attempt.Id, err))
		}
		return notifications.AttemptStartResult{Started: true, Pending: true}, nil
	}
	Wake()
	return notifications.AttemptStartResult{Started: true}, nil
}
