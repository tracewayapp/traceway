package shared

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// The move-over reads the tables V2 replaced. Endpoints, tasks, AI traces and exceptions come back in their V2 model
// with the old ids parked in the new fields exactly as they were stored: on an entity TraceId holds the old distributed
// trace id and SpanId the old span id, on an exception TraceId holds the old owner id and LinkedTraceId the old
// distributed trace id. The moveover package turns them into V2 ids, in one place for every backend.

// LegacySpan is a row of the old spans table, where trace_id named the owning endpoint, task or AI trace.
type LegacySpan struct {
	ProjectId    uuid.UUID
	Id           uuid.UUID
	OwnerId      uuid.UUID
	ParentSpanId string
	Name         string
	StartTime    time.Time
	Duration     time.Duration
	RecordedAt   time.Time
	Attributes   map[string]string
}

// LegacyTables maps each table V2 replaced to the table its rows move into.
var LegacyTables = map[string]string{
	"endpoints":              "endpoints_v2",
	"tasks":                  "tasks_v2",
	"ai_traces":              "ai_traces_v2",
	"exception_stack_traces": "exceptions_v2",
	"spans":                  SpansTable,
}

// LegacyTable guards the table names the move-over puts into SQL text.
func LegacyTable(name string) (string, error) {
	if _, known := LegacyTables[name]; known {
		return name, nil
	}
	for _, v2 := range LegacyTables {
		if v2 == name {
			return name, nil
		}
	}
	return "", fmt.Errorf("%q is not a table the move-over reads or writes", name)
}

// MovedSpanKey identifies a span already present in the V2 table when a day is resumed.
func MovedSpanKey(projectId uuid.UUID, traceId, spanId string) string {
	return OtelKey(projectId, traceId) + ":" + spanId
}
