package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/outbox"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

func sanitizeForDB(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "\uFFFD")
	}
	return s
}

// dispatch durably commits the notification: escalation rules open a page,
// everything else lands in the outbox for the drain worker to deliver with
// retries. It performs no network I/O. Returns true only when the commitment
// is persisted; callers use it to gate dedup recording, and the cooldown is
// recorded here at enqueue time (the durable promise) so a rule cannot
// re-fire while the outbox is still retrying.
func dispatch(rule *models.NotificationRuleWithChannel, msg Message) bool {
	channel, dbErr := db.ExecuteTransaction(func(tx *sql.Tx) (*models.NotificationChannel, error) {
		return transactional.NotificationChannelRepository.FindById(tx, rule.ChannelId)
	})
	if dbErr != nil || channel == nil {
		recordFiredNotification(rule, msg, "failed", "failed to load channel")
		return false
	}

	msg.RuleType = rule.RuleType
	msg.RuleName = rule.Name
	if rule.Severity != "" {
		msg.Severity = Severity(rule.Severity)
	}
	msg.URL = dashboardURL(msg.URL)

	// Escalation channels do not send anything themselves: they open (or
	// dedup into) an on-call page, and the escalator + outbox do all
	// notifying. The page open is itself the durable commitment.
	if channel.ChannelType == "escalation" {
		if pageOpener == nil {
			recordFiredNotification(rule, msg, "failed", "escalation pager not initialized")
			return false
		}
		opened, err := pageOpener(channel.Config, rule, msg)
		if err != nil {
			recordFiredNotification(rule, msg, "failed", err.Error())
			traceway.CaptureException(fmt.Errorf("failed to open page (rule=%d): %w", rule.Id, err))
			return false
		}
		cooldowns.recordFire(rule.Id)
		status := "sent"
		if !opened {
			status = "deduped"
		}
		recordFiredNotification(rule, msg, status, "")
		return true
	}

	ruleId := rule.Id
	projectId := rule.ProjectId
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return outbox.Enqueue(tx, outbox.Delivery{
			Kind:        models.OutboxKindRule,
			AdapterType: channel.ChannelType,
			// Snapshot: later channel edits do not affect queued sends.
			AdapterConfig: json.RawMessage(channel.Config),
			Message:       msg,
			RuleId:        &ruleId,
			ProjectId:     &projectId,
			ChannelName:   channel.Name,
		})
	})
	if err != nil {
		// No outbox row exists, so the terminal hook can never record this
		// delivery; the audit row must be written here or the notification
		// vanishes from the history entirely.
		recordFiredNotification(rule, msg, "failed", "failed to enqueue: "+err.Error())
		traceway.CaptureException(fmt.Errorf("failed to enqueue notification (rule=%d, channel=%s): %w", rule.Id, rule.ChannelName, err))
		return false
	}

	cooldowns.recordFire(rule.Id)
	outbox.Wake()
	return true
}

func recordFiredNotification(rule *models.NotificationRuleWithChannel, msg Message, status string, errorMsg string) {
	go func() {
		defer traceway.Recover()

		err := telemetry.FiredNotificationRepository.Insert(context.Background(), telemetry.FiredNotification{
			ProjectId:   rule.ProjectId,
			RuleId:      rule.Id,
			RuleType:    rule.RuleType,
			RuleName:    rule.Name,
			ChannelType: rule.ChannelType,
			ChannelName: rule.ChannelName,
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
			traceway.CaptureException(fmt.Errorf("failed to record fired notification to ClickHouse: %w", err))
		}
	}()
}
