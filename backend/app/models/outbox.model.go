package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	OutboxKindRule         = "rule"
	OutboxKindPage         = "page"
	OutboxKindVerification = "verification"

	OutboxPending   = "pending"
	OutboxSending   = "sending"
	OutboxSent      = "sent"
	OutboxFailed    = "failed"
	OutboxCancelled = "cancelled"
)

// OutboxDelivery is one persisted notification send. AdapterConfig can hold
// secrets (SMTP credentials, webhook secrets), so it is never serialized to
// JSON responses.
type OutboxDelivery struct {
	Id                 int        `json:"id" lit:"id"`
	Kind               string     `json:"kind" lit:"kind"`
	Status             string     `json:"status" lit:"status"`
	AdapterType        string     `json:"adapterType" lit:"adapter_type"`
	AdapterConfig      JSONText   `json:"-" lit:"adapter_config"`
	Message            JSONText   `json:"-" lit:"message"`
	Attempts           int        `json:"attempts" lit:"attempts"`
	NextAttemptAt      time.Time  `json:"nextAttemptAt" lit:"next_attempt_at"`
	ClaimedAt          *time.Time `json:"claimedAt" lit:"claimed_at"`
	CancelKey          string     `json:"cancelKey" lit:"cancel_key"`
	PageNotificationId *int       `json:"pageNotificationId" lit:"page_notification_id"`
	RuleId             *int       `json:"ruleId" lit:"rule_id"`
	ProjectId          *uuid.UUID `json:"projectId" lit:"project_id"`
	ChannelName        string     `json:"channelName" lit:"channel_name"`
	LastError          string     `json:"lastError" lit:"last_error"`
	CreatedAt          time.Time  `json:"createdAt" lit:"created_at"`
	SentAt             *time.Time `json:"sentAt" lit:"sent_at"`
}

type OutboxRuleEnqueue struct {
	RuleId         int       `lit:"rule_id"`
	LastEnqueuedAt time.Time `lit:"last_enqueued_at"`
}

type OutboxHealthCounts struct {
	PendingCount int `lit:"pending_count"`
	SendingCount int `lit:"sending_count"`
	FailedCount  int `lit:"failed_count"`
}
