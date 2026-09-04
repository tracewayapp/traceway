package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	traceway "go.tracewayapp.com"
)

// AdapterSend is the outbox's registered sender: one attempt through the
// channel adapter built from the persisted config snapshot.
func AdapterSend(ctx context.Context, adapterType string, adapterConfig json.RawMessage, msg models.NotificationMessage) error {
	adapter, err := NewAdapter(adapterType, adapterConfig)
	if err != nil {
		return err
	}
	// GitHub deliveries are two-way: they open an issue and later close it,
	// and an opened one is recorded so the close can find it. Adapter.Send
	// only covers the open half.
	if github, ok := adapter.(*GitHubAdapter); ok {
		return sendGitHubIssue(ctx, github, msg)
	}
	return adapter.Send(ctx, msg)
}

// OnOutboxTerminal records the fired_notifications audit row for rule
// deliveries once their outcome is final (fired_notifications is append-only
// telemetry, so it must record the final truth). Page deliveries are logged in
// page_notifications instead, mirrored by the drain worker.
func OnOutboxTerminal(row *models.OutboxDelivery, status string, errorMsg string) {
	if row.Kind != models.OutboxKindRule || row.RuleId == nil || row.ProjectId == nil {
		return
	}
	var msg Message
	if err := json.Unmarshal(row.Message, &msg); err != nil {
		traceway.CaptureException(fmt.Errorf("failed to decode outbox message for audit (row=%d): %w", row.Id, err))
		return
	}
	ruleId := *row.RuleId
	projectId := *row.ProjectId
	adapterType := row.AdapterType
	channelName := row.ChannelName
	go func() {
		defer traceway.Recover()

		err := telemetry.FiredNotificationRepository.Insert(context.Background(), telemetry.FiredNotification{
			ProjectId:   projectId,
			RuleId:      ruleId,
			RuleType:    msg.RuleType,
			RuleName:    msg.RuleName,
			ChannelType: adapterType,
			ChannelName: channelName,
			Severity:    string(msg.Severity),
			Subject:     sanitizeForDB(msg.Subject),
			Body:        sanitizeForDB(msg.Body),
			Status:      status,
			ErrorMsg:    sanitizeForDB(errorMsg),
			Endpoint:    msg.Endpoint,
			URL:         msg.URL,
			FiredAt:     time.Now().UTC(),
		})
		if err != nil {
			traceway.CaptureException(fmt.Errorf("failed to record fired notification: %w", err))
		}
	}()
}
