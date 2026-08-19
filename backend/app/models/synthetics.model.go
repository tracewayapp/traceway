package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	CheckTypeHttp    = "http"
	CheckTypeTcp     = "tcp"
	CheckTypeBrowser = "browser"

	CheckStatusUp      = "up"
	CheckStatusDown    = "down"
	CheckStatusUnknown = "unknown"

	CheckRunQueued  = "queued"
	CheckRunClaimed = "claimed"

	CheckResultUp     = "up"
	CheckResultDown   = "down"
	CheckResultMissed = "missed"
)

type SyntheticCheck struct {
	Id                  int        `json:"id" lit:"id"`
	ProjectId           uuid.UUID  `json:"projectId" lit:"project_id"`
	Name                string     `json:"name" lit:"name"`
	CheckType           string     `json:"checkType" lit:"check_type"`
	Enabled             bool       `json:"enabled" lit:"enabled"`
	IntervalSeconds     int        `json:"intervalSeconds" lit:"interval_seconds"`
	TimeoutSeconds      int        `json:"timeoutSeconds" lit:"timeout_seconds"`
	Config              JSONText   `json:"config" lit:"config"`
	FailureThreshold    int        `json:"failureThreshold" lit:"failure_threshold"`
	CurrentStatus       string     `json:"currentStatus" lit:"current_status"`
	ConsecutiveFailures int        `json:"consecutiveFailures" lit:"consecutive_failures"`
	LastRunAt           *time.Time `json:"lastRunAt" lit:"last_run_at"`
	LastStateChangeAt   *time.Time `json:"lastStateChangeAt" lit:"last_state_change_at"`
	NextRunAt           time.Time  `json:"nextRunAt" lit:"next_run_at"`
	CreatedAt           time.Time  `json:"createdAt" lit:"created_at"`
	UpdatedAt           time.Time  `json:"updatedAt" lit:"updated_at"`
}

type CheckRun struct {
	Id           int        `json:"id" lit:"id"`
	CheckId      int        `json:"checkId" lit:"check_id"`
	ProjectId    uuid.UUID  `json:"projectId" lit:"project_id"`
	CheckType    string     `json:"checkType" lit:"check_type"`
	Status       string     `json:"status" lit:"status"`
	ScheduledFor time.Time  `json:"scheduledFor" lit:"scheduled_for"`
	ExpiresAt    time.Time  `json:"expiresAt" lit:"expires_at"`
	ClaimedAt    *time.Time `json:"claimedAt" lit:"claimed_at"`
	ClaimedBy    string     `json:"claimedBy" lit:"claimed_by"`
	CreatedAt    time.Time  `json:"createdAt" lit:"created_at"`
}

type CheckIncident struct {
	Id           int        `json:"id" lit:"id"`
	CheckId      *int       `json:"checkId" lit:"check_id"`
	ProjectId    *uuid.UUID `json:"projectId" lit:"project_id"`
	StatusPageId *int       `json:"statusPageId" lit:"status_page_id"`
	Title        string     `json:"title" lit:"title"`
	StartedAt    time.Time  `json:"startedAt" lit:"started_at"`
	ResolvedAt   *time.Time `json:"resolvedAt" lit:"resolved_at"`
	ErrorMessage string     `json:"errorMessage" lit:"error_message"`
}

// SyntheticRunner is a liveness record for a self-registered browser-run
// executor. Runners are operator infrastructure: they authenticate with the
// shared SYNTHETICS_RUNNER_SECRET and register themselves by name on first
// poll, so rows carry no credentials and no tenant scoping.
type SyntheticRunner struct {
	Id          int       `json:"id" lit:"id"`
	Name        string    `json:"name" lit:"name"`
	Version     string    `json:"version" lit:"version"`
	FirstSeenAt time.Time `json:"firstSeenAt" lit:"first_seen_at"`
	LastSeenAt  time.Time `json:"lastSeenAt" lit:"last_seen_at"`
}

type StatusPage struct {
	Id             int       `json:"id" lit:"id"`
	OrganizationId int       `json:"organizationId" lit:"organization_id"`
	Slug           string    `json:"slug" lit:"slug"`
	Name           string    `json:"name" lit:"name"`
	IsPublic       bool      `json:"isPublic" lit:"is_public"`
	CheckIds       JSONText  `json:"checkIds" lit:"check_ids"`
	Description    string    `json:"description" lit:"description"`
	LogoKey        string    `json:"logoKey" lit:"logo_key"`
	CustomDomain   string    `json:"customDomain" lit:"custom_domain"`
	CreatedAt      time.Time `json:"createdAt" lit:"created_at"`
	UpdatedAt      time.Time `json:"updatedAt" lit:"updated_at"`
}

// Config document shapes stored in synthetic_checks.config, one per check
// type. Auth secrets live inside the main DB like notification channel
// configs, and like channel configs they are returned to any project member
// with read access; readonly gating restricts writes, not reads.

type HttpCheckConfig struct {
	URL             string            `json:"url"`
	Method          string            `json:"method"`
	Headers         map[string]string `json:"headers"`
	Auth            HttpCheckAuth     `json:"auth"`
	Body            string            `json:"body"`
	BodyContentType string            `json:"bodyContentType"`
	FollowRedirects bool              `json:"followRedirects"`
	SkipTlsVerify   bool              `json:"skipTlsVerify"`
	Assertions      HttpAssertions    `json:"assertions"`
}

type HttpCheckAuth struct {
	Type        string `json:"type"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Token       string `json:"token"`
	HeaderName  string `json:"headerName"`
	HeaderValue string `json:"headerValue"`
}

type HttpAssertions struct {
	StatusCodes         []int  `json:"statusCodes"`
	BodyContains        string `json:"bodyContains"`
	BodyRegex           string `json:"bodyRegex"`
	MaxResponseTimeMs   int    `json:"maxResponseTimeMs"`
	TlsMinDaysRemaining int    `json:"tlsMinDaysRemaining"`
}

type TcpCheckConfig struct {
	Host                string `json:"host"`
	Port                int    `json:"port"`
	UseTls              bool   `json:"useTls"`
	SendPayload         string `json:"sendPayload"`
	ExpectContains      string `json:"expectContains"`
	TlsMinDaysRemaining int    `json:"tlsMinDaysRemaining"`
}

type BrowserCheckConfig struct {
	Script string            `json:"script"`
	Env    map[string]string `json:"env"`
}

type CheckRunHealthCounts struct {
	QueuedCount  int `lit:"queued_count"`
	ClaimedCount int `lit:"claimed_count"`
}

// CheckAggregates summarizes a check's results over a time range. Latency
// aggregates exclude missed rows (they carry no measurement).
type CheckAggregates struct {
	Total        int64   `json:"total"`
	Up           int64   `json:"up"`
	Down         int64   `json:"down"`
	Missed       int64   `json:"missed"`
	AvgLatencyMs float64 `json:"avgLatencyMs"`
}

// CheckSeriesPoint is one time bucket of a check's results.
type CheckSeriesPoint struct {
	Bucket       time.Time `json:"bucket"`
	Total        int64     `json:"total"`
	Up           int64     `json:"up"`
	Missed       int64     `json:"missed"`
	AvgLatencyMs float64   `json:"avgLatencyMs"`
	MaxLatencyMs float64   `json:"maxLatencyMs"`
}

// CheckResultEntry is the JSON shape of one telemetry check_results row.
type CheckResultEntry struct {
	RunId            int       `json:"runId"`
	CheckType        string    `json:"checkType"`
	RecordedAt       time.Time `json:"recordedAt"`
	ExecutedAt       time.Time `json:"executedAt"`
	Status           string    `json:"status"`
	LatencyMs        float64   `json:"latencyMs"`
	StatusCode       int       `json:"statusCode"`
	ErrorMessage     string    `json:"errorMessage"`
	ExecutedBy       string    `json:"executedBy"`
	ScreenshotKey    string    `json:"screenshotKey"`
	OutputKey        string    `json:"outputKey"`
	TlsDaysRemaining int       `json:"tlsDaysRemaining"`
}
