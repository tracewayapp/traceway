package models

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

// Attempt statuses, in lifecycle order. Everything before AttemptAnalyzed is
// active: at most one active attempt exists per project and subject, which
// the partial unique index agent_attempts_active_subject_unique enforces.
const (
	AttemptPendingApproval = "pending_approval"
	AttemptQueued          = "queued"
	AttemptClaimed         = "claimed"
	AttemptPreparing       = "preparing"
	AttemptRunning         = "running"
	AttemptVerifying       = "verifying"
	AttemptPublishing      = "publishing"
	AttemptNeedsInput      = "needs_input"
	AttemptAwaitingReview  = "awaiting_review"
	AttemptAnalyzed        = "analyzed"
	AttemptMerged          = "merged"
	AttemptClosed          = "closed"
	AttemptFailed          = "failed"
	AttemptCancelled       = "cancelled"
	AttemptTimedOut        = "timed_out"
)

var AttemptActiveStatuses = []string{
	AttemptPendingApproval, AttemptQueued, AttemptClaimed, AttemptPreparing, AttemptRunning,
	AttemptVerifying, AttemptPublishing, AttemptNeedsInput, AttemptAwaitingReview,
}

// AttemptLeasedStatuses are the statuses in which an executor holds the
// attempt under a lease; an expired lease in any of them means the executor
// died and the attempt goes back to the queue.
var AttemptLeasedStatuses = []string{
	AttemptClaimed, AttemptPreparing, AttemptRunning, AttemptVerifying, AttemptPublishing,
}

var AttemptTerminalStatuses = []string{
	AttemptAnalyzed, AttemptMerged, AttemptClosed, AttemptFailed, AttemptCancelled, AttemptTimedOut,
}

func AttemptIsActive(status string) bool {
	return slices.Contains(AttemptActiveStatuses, status)
}

const (
	AttemptKindFix = "fix"

	SubjectKindTracewayException = "traceway_exception"

	MessageInbound  = "in"
	MessageOutbound = "out"

	MessageKindProgress = "progress"
	MessageKindFinding  = "finding"
	MessageKindQuestion = "question"
	MessageKindAnswer   = "answer"
	MessageKindPR       = "pr"

	LinkKindOrigin = "origin"
	LinkKindThread = "thread"
	LinkKindIssue  = "issue"
	LinkKindPR     = "pr"
	LinkKindAlert  = "alert"
)

// Integration is one connected provider of an organization. Config holds
// the provider's settings with its credentials encrypted by app/secrets; it
// is never serialized directly, controllers mask it.
type Integration struct {
	Id             int         `json:"id" lit:"id"`
	OrganizationId int         `json:"organizationId" lit:"organization_id"`
	Provider       string      `json:"provider" lit:"provider"`
	Kinds          StringSlice `json:"kinds" lit:"kinds"`
	Name           string      `json:"name" lit:"name"`
	Config         JSONText    `json:"-" lit:"config"`
	Enabled        bool        `json:"enabled" lit:"enabled"`
	CreatedBy      *int        `json:"createdBy" lit:"created_by"`
	CreatedAt      time.Time   `json:"createdAt" lit:"created_at"`
	UpdatedAt      time.Time   `json:"updatedAt" lit:"updated_at"`
}

// Identity maps an external account (a Slack member, a GitHub login) to a
// Traceway user; every channel and trigger authorizes through it.
type Identity struct {
	Id         int       `json:"id" lit:"id"`
	UserId     int       `json:"userId" lit:"user_id"`
	Provider   string    `json:"provider" lit:"provider"`
	ExternalId string    `json:"externalId" lit:"external_id"`
	Display    string    `json:"display" lit:"display"`
	CreatedAt  time.Time `json:"createdAt" lit:"created_at"`
}

// Repository binds a project to the code repository the agent works in. The
// credential lives on the integration, not here.
type Repository struct {
	Id            int       `json:"id" lit:"id"`
	ProjectId     uuid.UUID `json:"projectId" lit:"project_id"`
	IntegrationId *int      `json:"integrationId" lit:"integration_id"`
	Owner         string    `json:"owner" lit:"owner"`
	Name          string    `json:"name" lit:"name"`
	DefaultBranch string    `json:"defaultBranch" lit:"default_branch"`
	Image         string    `json:"image" lit:"image"`
	SetupCommand  string    `json:"setupCommand" lit:"setup_command"`
	TestCommand   string    `json:"testCommand" lit:"test_command"`
	CreatedAt     time.Time `json:"createdAt" lit:"created_at"`
	UpdatedAt     time.Time `json:"updatedAt" lit:"updated_at"`
}

// AgentProfile is an organization's configuration of one coding agent:
// which agent and model, where model calls go, and the limits an attempt
// runs under. Credential is the provider key, encrypted by app/secrets.
type AgentProfile struct {
	Id             int         `json:"id" lit:"id"`
	OrganizationId int         `json:"organizationId" lit:"organization_id"`
	Name           string      `json:"name" lit:"name"`
	Agent          string      `json:"agent" lit:"agent"`
	Package        string      `json:"package" lit:"package"`
	PackageVersion string      `json:"packageVersion" lit:"package_version"`
	Model          string      `json:"model" lit:"model"`
	Provider       string      `json:"provider" lit:"provider"`
	BaseURL        string      `json:"baseUrl" lit:"base_url"`
	Credential     string      `json:"-" lit:"credential"`
	MaxTurns       int         `json:"maxTurns" lit:"max_turns"`
	TimeoutMinutes int         `json:"timeoutMinutes" lit:"timeout_minutes"`
	BudgetUSD      float64     `json:"budgetUsd" lit:"budget_usd"`
	AllowedTools   StringSlice `json:"allowedTools" lit:"allowed_tools"`
	NetworkPolicy  JSONText    `json:"networkPolicy" lit:"network_policy"`
	IsDefault      bool        `json:"isDefault" lit:"is_default"`
	CreatedAt      time.Time   `json:"createdAt" lit:"created_at"`
	UpdatedAt      time.Time   `json:"updatedAt" lit:"updated_at"`
}

// AgentAttempt is one run of the agent against a subject. External artifacts
// (threads, issues, pull requests) live in agent_links, the conversation in
// agent_messages, and the event stream in agent_attempt_events.
type AgentAttempt struct {
	Id             uuid.UUID  `json:"id" lit:"id"`
	OrganizationId int        `json:"organizationId" lit:"organization_id"`
	ProjectId      uuid.UUID  `json:"projectId" lit:"project_id"`
	RepositoryId   *int       `json:"repositoryId" lit:"repository_id"`
	ProfileId      *int       `json:"profileId" lit:"profile_id"`
	Number         int        `json:"number" lit:"number"`
	Kind           string     `json:"kind" lit:"kind"`
	SubjectKind    string     `json:"subjectKind" lit:"subject_kind"`
	SubjectRef     string     `json:"subjectRef" lit:"subject_ref"`
	Executor       string     `json:"executor" lit:"executor"`
	Status         string     `json:"status" lit:"status"`
	Resume         bool       `json:"resume" lit:"resume"`
	BaseBranch     string     `json:"baseBranch" lit:"base_branch"`
	FixBranch      string     `json:"fixBranch" lit:"fix_branch"`
	Agent          string     `json:"agent" lit:"agent"`
	Model          string     `json:"model" lit:"model"`
	CostUSD        float64    `json:"costUsd" lit:"cost_usd"`
	InputTokens    int64      `json:"inputTokens" lit:"input_tokens"`
	OutputTokens   int64      `json:"outputTokens" lit:"output_tokens"`
	Turns          int        `json:"turns" lit:"turns"`
	ClaimedBy      string     `json:"claimedBy" lit:"claimed_by"`
	LeaseExpiresAt *time.Time `json:"leaseExpiresAt" lit:"lease_expires_at"`
	RequestedBy    *int       `json:"requestedBy" lit:"requested_by"`
	ApprovedBy     *int       `json:"approvedBy" lit:"approved_by"`
	Error          string     `json:"error" lit:"error"`
	ReportKey      string     `json:"reportKey" lit:"report_key"`
	CreatedAt      time.Time  `json:"createdAt" lit:"created_at"`
	StartedAt      *time.Time `json:"startedAt" lit:"started_at"`
	FinishedAt     *time.Time `json:"finishedAt" lit:"finished_at"`
	UpdatedAt      time.Time  `json:"updatedAt" lit:"updated_at"`
}

// AttemptOutcome is what an executor reports back onto the attempt row:
// the columns it owns, separate from the status the control plane owns.
type AttemptOutcome struct {
	Executor     string
	Agent        string
	Model        string
	BaseBranch   string
	FixBranch    string
	CostUSD      float64
	InputTokens  int64
	OutputTokens int64
	Turns        int
	ReportKey    string
	Error        string
}

type AgentAttemptStatusCount struct {
	Status string `lit:"status"`
	Count  int    `lit:"count"`
}

// AgentAttemptEvent is one entry of an attempt's append-only, sequenced
// event stream: phase changes, assistant text, tool call summaries, usage.
type AgentAttemptEvent struct {
	Id        int       `json:"id" lit:"id"`
	AttemptId uuid.UUID `json:"attemptId" lit:"attempt_id"`
	Seq       int       `json:"seq" lit:"seq"`
	Kind      string    `json:"kind" lit:"kind"`
	Payload   JSONText  `json:"payload" lit:"payload"`
	CreatedAt time.Time `json:"createdAt" lit:"created_at"`
}

// AgentMessage is one entry of an attempt's conversation thread, whichever
// surface it came from or went to.
type AgentMessage struct {
	Id          int        `json:"id" lit:"id"`
	AttemptId   uuid.UUID  `json:"attemptId" lit:"attempt_id"`
	Direction   string     `json:"direction" lit:"direction"`
	Provider    string     `json:"provider" lit:"provider"`
	LinkId      *int       `json:"linkId" lit:"link_id"`
	IdentityId  *int       `json:"identityId" lit:"identity_id"`
	Kind        string     `json:"kind" lit:"kind"`
	Body        string     `json:"body" lit:"body"`
	ExternalRef string     `json:"externalRef" lit:"external_ref"`
	DeliveredAt *time.Time `json:"deliveredAt" lit:"delivered_at"`
	CreatedAt   time.Time  `json:"createdAt" lit:"created_at"`
}

// AgentLink is one external artifact of an attempt: its origin, a chat
// thread, an issue, a pull request, an alert.
type AgentLink struct {
	Id            int       `json:"id" lit:"id"`
	AttemptId     uuid.UUID `json:"attemptId" lit:"attempt_id"`
	IntegrationId *int      `json:"integrationId" lit:"integration_id"`
	Provider      string    `json:"provider" lit:"provider"`
	Kind          string    `json:"kind" lit:"kind"`
	ExternalRef   string    `json:"externalRef" lit:"external_ref"`
	URL           string    `json:"url" lit:"url"`
	CreatedAt     time.Time `json:"createdAt" lit:"created_at"`
}

// ProjectTelemetrySource says which integration answers a telemetry domain
// for a project, in priority order. Traceway itself is implicit.
type ProjectTelemetrySource struct {
	Id            int       `json:"id" lit:"id"`
	ProjectId     uuid.UUID `json:"projectId" lit:"project_id"`
	Domain        string    `json:"domain" lit:"domain"`
	IntegrationId int       `json:"integrationId" lit:"integration_id"`
	Priority      int       `json:"priority" lit:"priority"`
}

// AgentRunner is the liveness row of a remote agent runner, self-registered
// by name like a synthetic runner.
type AgentRunner struct {
	Id           int       `json:"id" lit:"id"`
	Name         string    `json:"name" lit:"name"`
	Version      string    `json:"version" lit:"version"`
	Capabilities JSONText  `json:"capabilities" lit:"capabilities"`
	FirstSeenAt  time.Time `json:"firstSeenAt" lit:"first_seen_at"`
	LastSeenAt   time.Time `json:"lastSeenAt" lit:"last_seen_at"`
}
