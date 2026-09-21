package clientmodels

import (
	"testing"

	"github.com/google/uuid"
)

func TestNativeMissingIDsAreGeneratedOnce(t *testing.T) {
	for _, input := range []string{"", "invalid", uuid.Nil.String(), hexId(uuid.Nil)} {
		t.Run(input, func(t *testing.T) {
			run := &ClientTrace{Id: input, LegacyDistributedTraceId: uuid.Nil.String()}
			child := &ClientSpan{Id: input, ParentSpanId: uuid.Nil.String()}
			project := uuid.New()
			root := run.RootSpan(project, "")
			endpoint, task := run.ToEndpoint("", ""), run.ToTask("", "")
			if endpoint.Id == uuid.Nil || endpoint.Id != task.Id || root.TraceId != hexId(endpoint.Id) || root.SpanId != root.TraceId || endpoint.TraceId != root.TraceId || task.TraceId != root.TraceId {
				t.Fatalf("generated run identity is inconsistent: root=%+v endpoint=%+v task=%+v", root, endpoint, task)
			}
			span := child.ToSpan(run, project, "")
			if child.ParsedId() == uuid.Nil || span.SpanId != hexId(child.ParsedId()) || span.TraceId != root.TraceId || span.ParentSpanId != root.SpanId {
				t.Fatalf("generated child identity or parent changed: %+v", span)
			}
		})
	}
}

func TestNativeExceptionIgnoresZeroTraceIDs(t *testing.T) {
	run, distributed, zero := uuid.NewString(), uuid.NewString(), uuid.Nil.String()
	for _, tt := range []struct {
		name                string
		run, distributed    *string
		wantTrace, wantSpan string
	}{
		{"untraced", nil, nil, "", ""},
		{"zero IDs", &zero, &zero, "", ""},
		{"zero distributed", &run, &zero, hexId(uuid.MustParse(run)), hexId(uuid.MustParse(run))},
		{"missing distributed", &run, nil, hexId(uuid.MustParse(run)), hexId(uuid.MustParse(run))},
		{"valid distributed", &run, &distributed, hexId(uuid.MustParse(distributed)), hexId(uuid.MustParse(run))},
		{"zero run", &zero, &distributed, hexId(uuid.MustParse(distributed)), ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := &ClientExceptionStackTrace{TraceId: tt.run, LegacyDistributedTraceId: tt.distributed}
			got := input.ToExceptionStackTrace("hash", "", "")
			if got.TraceId != tt.wantTrace || got.SpanId != tt.wantSpan {
				t.Fatalf("exception identity = %q/%q, want %q/%q", got.TraceId, got.SpanId, tt.wantTrace, tt.wantSpan)
			}
		})
	}
}
