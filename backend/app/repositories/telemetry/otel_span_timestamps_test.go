package telemetry

import (
	"context"
	"encoding/hex"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestOtelSpanTimestampBoundariesKeepExactSourceValues(t *testing.T) {
	setupTestDB(t)
	ctx, project := context.Background(), uuid.New()
	historical := uint64(time.Date(2019, 3, 4, 5, 6, 7, 123456789, time.UTC).UnixNano())
	cases := map[string][2]uint64{
		"missing start and end":      {0, 0},
		"historical":                 {historical, historical + 1_000_000},
		"reversed interval":          {historical + 5_000, historical},
		"beyond signed nanoseconds":  {math.MaxInt64 + 1_000, math.MaxInt64 + 2_000},
		"largest unsigned timestamp": {math.MaxUint64 - 1, math.MaxUint64},
	}
	fallbacksBefore := db.GetSpanPartitionTimeFallbacks()
	index := byte(0)
	for name, times := range cases {
		index++
		trace := uuid.New()
		source := &tracepb.Span{TraceId: trace[:], SpanId: []byte{index, 2, 3, 4, 5, 6, 7, 8}, Name: name, StartTimeUnixNano: times[0], EndTimeUnixNano: times[1]}
		source.Attributes = []*commonpb.KeyValue{{Key: "test.case", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: name}}}}
		span := models.OtelSpan{OTLP: source, Span: models.Span{ProjectId: project, TraceId: hex.EncodeToString(trace[:]), SpanId: hex.EncodeToString(source.SpanId),
			Name: name, StartTime: shared.OtelNanosToTime(times[0]), Duration: shared.OtelDuration(times[0], times[1])}}
		if rejected, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{span}); err != nil || rejected != 0 {
			t.Fatalf("%s: rejected=%d err=%v", name, rejected, err)
		}
		// Reads are anchored on the partition time, which is the ingest time when the source start cannot be stored.
		partition, _, _ := shared.OtelStorageTimes(times[0], time.Now())
		payload, err := OtelSpanRepository.FindOTLP(ctx, project, span.TraceId, span.SpanId, partition)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var restored tracepb.ResourceSpans
		if err := proto.Unmarshal(payload, &restored); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		stored := restored.ScopeSpans[0].Spans[0]
		if stored.StartTimeUnixNano != times[0] || stored.EndTimeUnixNano != times[1] {
			t.Errorf("%s: exact source times changed to %d %d", name, stored.StartTimeUnixNano, stored.EndTimeUnixNano)
		}
		found, err := OtelSpanRepository.FindTraceTopology(ctx, []shared.SpanLookup{{ProjectId: project, TraceId: span.TraceId, RecordedAt: &partition}}, 0)
		if err != nil || len(found) != 1 {
			t.Fatalf("%s: by-ID retrieval must not depend on the start hint: %d rows %v", name, len(found), err)
		}
		if found[0].Duration < 0 {
			t.Errorf("%s: derived duration %d is negative", name, found[0].Duration)
		}
		if !found[0].StartTime.Equal(span.StartTime) || found[0].RecordedAt.Sub(partition).Abs() >= time.Minute {
			t.Errorf("%s: topology must keep source start %s and storage time near %s, got %s and %s", name, span.StartTime, partition, found[0].StartTime, found[0].RecordedAt)
		}
		results, total, err := OtelSpanRepository.Search(ctx, shared.OtelSpanSearch{
			ProjectId: project, TraceId: span.TraceId, From: partition.Add(-time.Minute), To: partition.Add(time.Minute), Page: 1, PageSize: 10,
		})
		if err != nil || total != 1 || len(results) != 1 {
			t.Fatalf("%s: search must find the stored span: %d rows, total %d, %v", name, len(results), total, err)
		}
		result := results[0]
		graph, err := SpanRepository.FindTrace(ctx, []uuid.UUID{project}, result.TraceId, result.RecordedAt)
		if err != nil || graph == nil || len(graph.Spans) != 1 {
			t.Fatalf("%s: search result at=%s must reopen its trace: %+v %v", name, result.RecordedAt, graph, err)
		}
		reopened := graph.Spans[0]
		if !reopened.RecordedAt.Equal(result.RecordedAt) || !reopened.StartTime.Equal(span.StartTime) {
			t.Errorf("%s: trace outline must retain storage and source times: %+v", name, reopened)
		}
		attributes, err := SpanRepository.FindSpanAttributes(ctx, project, reopened.TraceId, reopened.SpanId, reopened.RecordedAt)
		if err != nil || attributes == nil || attributes.Attributes["test.case"] != name {
			t.Fatalf("%s: reopened span must load its attributes: %+v %v", name, attributes, err)
		}
		reopenedPayload, err := OtelSpanRepository.FindOTLP(ctx, project, reopened.TraceId, reopened.SpanId, reopened.RecordedAt)
		var reopenedSource tracepb.ResourceSpans
		if err != nil || len(reopenedPayload) == 0 || proto.Unmarshal(reopenedPayload, &reopenedSource) != nil || !proto.Equal(&restored, &reopenedSource) {
			t.Fatalf("%s: reopening must export the same original payload: %v", name, err)
		}
	}
	if fallbacks := db.GetSpanPartitionTimeFallbacks() - fallbacksBefore; fallbacks != 3 {
		t.Errorf("missing and unrepresentable starts must be counted, got %d fallbacks", fallbacks)
	}
}
