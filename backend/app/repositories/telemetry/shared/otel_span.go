package shared

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

const SpansTable = "spans_v2"

// OtelKey is the DuckDB trace_key: that backend keeps one index, and it has to cover project and trace together.
func OtelKey(project uuid.UUID, id string) string { return project.String() + ":" + id }

func OtelTracePredicate(lookups []SpanLookup, combinedKey bool) (string, []any) {
	conditions := make([]string, 0, len(lookups))
	args := make([]any, 0, len(lookups)*2)
	for _, lookup := range lookups {
		if combinedKey {
			conditions = append(conditions, "?")
			args = append(args, OtelKey(lookup.ProjectId, lookup.TraceId))
		} else {
			conditions = append(conditions, "(project_id = ? AND trace_id = ?)")
			args = append(args, lookup.ProjectId.String(), lookup.TraceId)
		}
	}
	if combinedKey {
		return "trace_key IN (" + strings.Join(conditions, ",") + ")", args
	}
	return "(" + strings.Join(conditions, " OR ") + ")", args
}

func preferOtelSpan(a, b models.OtelSpan) bool {
	if a.Duration != b.Duration {
		return a.Duration > b.Duration
	}
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(aa) > string(bb)
}

// NormalizeTraceId turns any spelling of an id (a UUID with dashes, upper case hex) into the stored one: lowercase hex.
func NormalizeTraceId(id string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(id), "-", ""))
}
