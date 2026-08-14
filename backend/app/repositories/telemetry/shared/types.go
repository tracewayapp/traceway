package shared

import (
	"time"

	"github.com/google/uuid"
)

type FiredNotification struct {
	ProjectId   uuid.UUID
	RuleId      int
	RuleType    string
	RuleName    string
	ChannelType string
	ChannelName string
	Severity    string
	Subject     string
	Body        string
	Status      string
	ErrorMsg    string
	Endpoint    string
	URL         string
	FiredAt     time.Time
}

// CheckResult is one synthetic-check probe outcome. RecordedAt is the
// scheduled time (the uptime timeline position); ExecutedAt is when the probe
// actually ran.
type CheckResult struct {
	ProjectId        uuid.UUID
	CheckId          int
	RunId            int
	CheckType        string
	RecordedAt       time.Time
	ExecutedAt       time.Time
	Status           string
	LatencyMs        float64
	StatusCode       int
	ErrorMsg         string
	ExecutedBy       string
	ScreenshotKey    string
	// OutputKey references the stored Playwright output (stdout/stderr tail
	// + report error) of a failed browser run, next to the screenshot.
	OutputKey        string
	TlsDaysRemaining int
}

// LogAttributeFilter selects logs by attribute value. Scope is one of
// "resource", "scope", or "log"; picks which of the three attribute maps to
// query (Map columns with bloom-filter indexes on ClickHouse, JSON on
// SQLite/DuckDB). Contains switches from exact match to case-insensitive
// substring match; Exclude negates either mode. Negated filters keep rows
// that don't carry the attribute at all.
type LogAttributeFilter struct {
	Scope    string
	Key      string
	Value    string
	Exclude  bool
	Contains bool
}

type LogSearchParams struct {
	ProjectId        uuid.UUID
	FromDate         time.Time
	ToDate           time.Time
	Search           string
	SearchType       string
	MinSeverity      uint8
	ServiceName      string
	TraceId          string
	TraceIds         []string
	SpanId           string
	ScopeName        string
	Body             string
	AttributeFilters []LogAttributeFilter
	OrderBy          string
	SortDirection    string
	Page             int
	PageSize         int
}
