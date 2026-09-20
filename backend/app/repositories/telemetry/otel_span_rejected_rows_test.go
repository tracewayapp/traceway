package telemetry

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

func TestOtelSpanInsertSkipsAndCountsAnUnstorableRow(t *testing.T) {
	setupTestDB(t)
	project, trace := uuid.New(), uuid.New()
	first := canonicalSpan(project, trace, uuid.MustParse("00000000-0000-0000-0101-010101010101"), nil)
	last := canonicalSpan(project, trace, uuid.MustParse("00000000-0000-0000-0303-030303030303"), nil)
	unstorable := canonicalSpan(project, trace, uuid.MustParse("00000000-0000-0000-0202-020202020202"), nil)
	unstorable.SpanId = "not hexadecimal"

	before, _, _ := db.GetTelemetryIngestCounters()
	rejected, err := OtelSpanRepository.InsertAsync(context.Background(), []models.OtelSpan{first, unstorable, last})
	after, _, _ := db.GetTelemetryIngestCounters()
	if err != nil || rejected != 1 {
		t.Fatalf("one bad row must not fail the batch: rejected=%d err=%v", rejected, err)
	}
	if after[shared.SpansTable] != before[shared.SpansTable]+1 {
		t.Fatalf("the rejected row must be counted: %d -> %d", before[shared.SpansTable], after[shared.SpansTable])
	}
	stored, err := OtelSpanRepository.FindTraceTopology(context.Background(), []shared.SpanLookup{{ProjectId: project, TraceId: first.TraceId}}, 0)
	if err != nil || len(stored) != 2 {
		t.Fatalf("rows around the rejected one must persist: %d %v", len(stored), err)
	}
}
