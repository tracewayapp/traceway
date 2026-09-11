package shared

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const SpanLookupBatchSize = 100

type SpanLookup struct {
	ProjectId uuid.UUID
	// TraceId is the owning occurrence ID, which may differ from the OTel trace ID.
	TraceId    uuid.UUID
	RecordedAt *time.Time
}

func SpanLookupQuery(lookups []SpanLookup, timeValue func(time.Time) any) (string, []any) {
	conditions := make([]string, 0, len(lookups))
	args := make([]any, 0, 4*len(lookups))
	for _, lookup := range lookups {
		condition := "project_id = ? AND trace_id = ?"
		args = append(args, lookup.ProjectId, lookup.TraceId)
		if lookup.RecordedAt != nil {
			from, to := TraceWindowBounds(*lookup.RecordedAt)
			condition += " AND recorded_at >= ? AND recorded_at <= ?"
			args = append(args, timeValue(from), timeValue(to))
		}
		conditions = append(conditions, "("+condition+")")
	}
	return `SELECT id, trace_id, project_id, name, start_time, duration, recorded_at, parent_span_id, attributes
		FROM spans WHERE ` + strings.Join(conditions, " OR ") + ` ORDER BY start_time ASC`, args
}
