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

// LogAttributeFilter selects logs by an exact attribute value. Scope is one of
// "resource", "scope", or "log"; picks which of the three attribute maps to
// query (Map columns with bloom-filter indexes on ClickHouse, JSON on
// SQLite/DuckDB).
type LogAttributeFilter struct {
	Scope   string
	Key     string
	Value   string
	Exclude bool
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
