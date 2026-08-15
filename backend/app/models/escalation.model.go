package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type EscalationPolicy struct {
	Id             int       `json:"id" lit:"id"`
	OrganizationId int       `json:"organizationId" lit:"organization_id"`
	Name           string    `json:"name" lit:"name"`
	Definition     JSONText  `json:"definition" lit:"definition"`
	CreatedBy      *int      `json:"createdBy" lit:"created_by"`
	CreatedAt      time.Time `json:"createdAt" lit:"created_at"`
	UpdatedAt      time.Time `json:"updatedAt" lit:"updated_at"`
}

const (
	PageStatusOpen         = "open"
	PageStatusAcknowledged = "acknowledged"
	PageStatusResolved     = "resolved"
)

type Page struct {
	Id               int        `json:"id" lit:"id"`
	OrganizationId   int        `json:"organizationId" lit:"organization_id"`
	ProjectId        uuid.UUID  `json:"projectId" lit:"project_id"`
	PolicyId         *int       `json:"policyId" lit:"policy_id"`
	PolicySnapshot   JSONText   `json:"policySnapshot" lit:"policy_snapshot"`
	RuleId           *int       `json:"ruleId" lit:"rule_id"`
	RuleName         string     `json:"ruleName" lit:"rule_name"`
	RuleType         string     `json:"ruleType" lit:"rule_type"`
	Subject          string     `json:"subject" lit:"subject"`
	Body             string     `json:"body" lit:"body"`
	URL              string     `json:"url" lit:"url"`
	Severity         string     `json:"severity" lit:"severity"`
	Urgency          string     `json:"urgency" lit:"urgency"`
	Status           string     `json:"status" lit:"status"`
	DedupKey         string     `json:"-" lit:"dedup_key"`
	EventCount       int        `json:"eventCount" lit:"event_count"`
	LastEventAt      time.Time  `json:"lastEventAt" lit:"last_event_at"`
	EscalationLevel  int        `json:"escalationLevel" lit:"escalation_level"`
	RepeatIteration  int        `json:"repeatIteration" lit:"repeat_iteration"`
	NextEscalationAt *time.Time `json:"nextEscalationAt" lit:"next_escalation_at"`
	LastEscalatedAt  *time.Time `json:"lastEscalatedAt" lit:"last_escalated_at"`
	AcknowledgedBy   *int       `json:"acknowledgedBy" lit:"acknowledged_by"`
	AcknowledgedVia  string     `json:"acknowledgedVia" lit:"acknowledged_via"`
	AcknowledgedAt   *time.Time `json:"acknowledgedAt" lit:"acknowledged_at"`
	ResolvedBy       *int       `json:"resolvedBy" lit:"resolved_by"`
	ResolvedAt       *time.Time `json:"resolvedAt" lit:"resolved_at"`
	CreatedAt        time.Time  `json:"createdAt" lit:"created_at"`
	UpdatedAt        time.Time  `json:"updatedAt" lit:"updated_at"`
}

// IssueHash returns the exception hash a page was opened for, or "" when the
// page is not issue-linked. New-error and regression rules carry the hash as
// the dedup token after the "ruleId|" prefix (see oncall.pageDedupKey).
func (p *Page) IssueHash() string {
	if p.RuleType != "new_error" && p.RuleType != "error_regression" {
		return ""
	}
	_, hash, found := strings.Cut(p.DedupKey, "|")
	if !found {
		return ""
	}
	return hash
}

type UserContactMethod struct {
	Id                    int        `json:"id" lit:"id"`
	UserId                int        `json:"userId" lit:"user_id"`
	MethodType            string     `json:"methodType" lit:"method_type"`
	Config                JSONText   `json:"config" lit:"config"`
	Enabled               bool       `json:"enabled" lit:"enabled"`
	Verified              bool       `json:"verified" lit:"verified"`
	VerificationCodeHash  string     `json:"-" lit:"verification_code_hash"`
	VerificationExpiresAt *time.Time `json:"-" lit:"verification_expires_at"`
	VerificationAttempts  int        `json:"-" lit:"verification_attempts"`
	CreatedAt             time.Time  `json:"createdAt" lit:"created_at"`
}

const (
	UrgencyHigh = "high"
	UrgencyLow  = "low"
)

type UserNotificationRule struct {
	Id              int       `json:"id" lit:"id"`
	UserId          int       `json:"userId" lit:"user_id"`
	Urgency         string    `json:"urgency" lit:"urgency"`
	Position        int       `json:"position" lit:"position"`
	DelayMinutes    int       `json:"delayMinutes" lit:"delay_minutes"`
	ContactMethodId int       `json:"contactMethodId" lit:"contact_method_id"`
	CreatedAt       time.Time `json:"createdAt" lit:"created_at"`
}

const (
	PageNotificationPending   = "pending"
	PageNotificationSent      = "sent"
	PageNotificationFailed    = "failed"
	PageNotificationCancelled = "cancelled"
)

type PageNotification struct {
	Id           int        `json:"id" lit:"id"`
	PageId       int        `json:"pageId" lit:"page_id"`
	Level        int        `json:"level" lit:"level"`
	Iteration    int        `json:"iteration" lit:"iteration"`
	UserId       *int       `json:"userId" lit:"user_id"`
	TargetDesc   string     `json:"targetDesc" lit:"target_desc"`
	MethodType   string     `json:"methodType" lit:"method_type"`
	Status       string     `json:"status" lit:"status"`
	ErrorMsg     string     `json:"errorMsg" lit:"error_msg"`
	ScheduledFor *time.Time `json:"scheduledFor" lit:"scheduled_for"`
	AckTokenHash string     `json:"-" lit:"ack_token_hash"`
	CreatedAt    time.Time  `json:"createdAt" lit:"created_at"`
	SentAt       *time.Time `json:"sentAt" lit:"sent_at"`
}
