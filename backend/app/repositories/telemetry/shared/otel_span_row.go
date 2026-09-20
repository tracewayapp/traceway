package shared

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	traceway "go.tracewayapp.com"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

const OtelScalarColumns = `project_id, trace_id, span_id, parent_span_id, name, span_kind, status_code, status_message, service_name, scope_name, scope_version, start_time_unix_nano, end_time_unix_nano, duration, recorded_at, trace_state, flags, dropped_attributes_count, dropped_events_count, dropped_links_count, resource_schema_url, scope_schema_url`

const OtelTextStorageColumns = OtelScalarColumns + `, span_attributes, resource, scope, events, links, otlp`

var OtelTextInsertSQL = `INSERT INTO ` + SpansTable + ` (` + OtelTextStorageColumns + `) VALUES (` + strings.TrimSuffix(strings.Repeat("?,", len(strings.Split(OtelTextStorageColumns, ","))), ",") + `)`

var (
	deterministicProto    = proto.MarshalOptions{Deterministic: true}
	resourceSpansScopeTag = (&tracepb.ResourceSpans{}).ProtoReflect().Descriptor().Fields().ByName("scope_spans").Number()
	scopeSpansSpanTag     = (&tracepb.ScopeSpans{}).ProtoReflect().Descriptor().Fields().ByName("spans").Number()
)

type OtelSpanGroup struct {
	Resource          *resourcepb.Resource
	Scope             *commonpb.InstrumentationScope
	ResourceSchemaUrl string
	ScopeSchemaUrl    string
	ResourcePB        []byte
	ScopePB           []byte
	memo              map[string]string
}

func (g *OtelSpanGroup) Memo(key string, build func() (string, error)) (string, error) {
	if value, ok := g.memo[key]; ok {
		return value, nil
	}
	value, err := build()
	if err != nil {
		return "", err
	}
	g.memo[key] = value
	return value, nil
}

type OtelSpanGroups map[*tracepb.ResourceSpans]*OtelSpanGroup

func (groups OtelSpanGroups) group(context *tracepb.ResourceSpans) (*OtelSpanGroup, error) {
	if group, ok := groups[context]; ok {
		return group, nil
	}
	group := &OtelSpanGroup{memo: make(map[string]string)}
	if context != nil {
		if len(context.ScopeSpans) != 1 {
			return nil, fmt.Errorf("expected one instrumentation scope per stored span")
		}
		scopeEnvelope := context.ScopeSpans[0]
		if len(scopeEnvelope.Spans) > 0 {
			scopeEnvelope = proto.Clone(scopeEnvelope).(*tracepb.ScopeSpans)
			scopeEnvelope.Spans = nil
		}
		resourceEnvelope := proto.Clone(context).(*tracepb.ResourceSpans)
		resourceEnvelope.ScopeSpans = nil
		var err error
		if group.ResourcePB, err = deterministicProto.Marshal(resourceEnvelope); err != nil {
			return nil, err
		}
		if group.ScopePB, err = deterministicProto.Marshal(scopeEnvelope); err != nil {
			return nil, err
		}
		group.Resource, group.ResourceSchemaUrl = context.Resource, context.SchemaUrl
		group.Scope, group.ScopeSchemaUrl = scopeEnvelope.Scope, scopeEnvelope.SchemaUrl
	}
	groups[context] = group
	return group, nil
}

// Valid because protobuf accepts a message's fields in any order: each envelope only lacks its one nested field.
func AssembleOtelPayload(resourcePB, scopePB, spanPB []byte) []byte {
	scopeSpans := make([]byte, 0, len(scopePB)+len(spanPB)+8)
	scopeSpans = append(scopeSpans, scopePB...)
	scopeSpans = protowire.AppendTag(scopeSpans, scopeSpansSpanTag, protowire.BytesType)
	scopeSpans = protowire.AppendBytes(scopeSpans, spanPB)
	payload := make([]byte, 0, len(resourcePB)+len(scopeSpans)+8)
	payload = append(payload, resourcePB...)
	payload = protowire.AppendTag(payload, resourceSpansScopeTag, protowire.BytesType)
	return protowire.AppendBytes(payload, scopeSpans)
}

const rejectedSpanReportInterval = time.Minute

var rejectedSpans struct {
	sync.Mutex
	pending  uint64
	reported time.Time
}

func RecordRejectedOtelSpan(err error) {
	db.RecordTelemetryRowDropped(SpansTable)
	rejectedSpans.Lock()
	rejectedSpans.pending++
	var report uint64
	if time.Since(rejectedSpans.reported) >= rejectedSpanReportInterval {
		report, rejectedSpans.pending, rejectedSpans.reported = rejectedSpans.pending, 0, time.Now()
	}
	rejectedSpans.Unlock()
	if report > 0 {
		traceway.CaptureException(fmt.Errorf("otel span insert: rejected %d spans since last report, latest: %w", report, err))
	}
}

type OtelSpanRow struct {
	Span          models.Span
	Source        *tracepb.Span
	Group         *OtelSpanGroup
	SpanPB        []byte
	Synthesized   bool
	PartitionTime time.Time
}

// NewOtelSpanRow prepares one span for storage. A span that did not arrive as OTLP, one from the native protocol, gets
// a payload built from its fields, so every stored span can be returned in the same format.
func NewOtelSpanRow(span models.OtelSpan, groups OtelSpanGroups) (*OtelSpanRow, error) {
	traceID, err := hex.DecodeString(span.TraceId)
	if err != nil || len(traceID) != 16 {
		return nil, fmt.Errorf("invalid trace ID %q", span.TraceId)
	}
	sourceID, err := hex.DecodeString(span.SpanId)
	if err != nil || len(sourceID) == 0 {
		return nil, fmt.Errorf("invalid span ID %q", span.SpanId)
	}
	row := &OtelSpanRow{Span: span.Span, Source: span.OTLP}
	if row.Source == nil {
		row.Synthesized = true
		row.Source = &tracepb.Span{TraceId: traceID, SpanId: sourceID, Name: span.Name, Kind: tracepb.Span_SpanKind(span.SpanKind),
			StartTimeUnixNano: uint64(span.StartTime.UnixNano()), EndTimeUnixNano: uint64(span.StartTime.Add(span.Duration).UnixNano())}
		if span.StatusCode != 0 {
			row.Source.Status = &tracepb.Status{Code: tracepb.Status_StatusCode(span.StatusCode)}
		}
		if parent, err := hex.DecodeString(span.ParentSpanId); err == nil && len(parent) > 0 {
			row.Source.ParentSpanId = parent
		}
	}
	if row.Group, err = groups.group(span.Context); err != nil {
		return nil, err
	}
	var fallback bool
	if row.PartitionTime, _, fallback = OtelStorageTimes(row.Source.StartTimeUnixNano, time.Now()); fallback {
		db.RecordSpanPartitionTimeFallback()
	}
	if row.SpanPB, err = deterministicProto.Marshal(row.Source); err != nil {
		return nil, err
	}
	return row, nil
}

func (row *OtelSpanRow) Payload() []byte {
	return AssembleOtelPayload(row.Group.ResourcePB, row.Group.ScopePB, row.SpanPB)
}

type OtelValueCodec struct {
	UUID   func(uuid.UUID) any
	Time   func(time.Time) any
	Nanos  func(uint64) any
	Uint32 func(uint32) any
}

// Order must match OtelScalarColumns.
func (row *OtelSpanRow) ScalarValues(codec OtelValueCodec) ([]any, error) {
	span, source, group := row.Span, row.Source, row.Group
	return []any{
		codec.UUID(span.ProjectId), span.TraceId, span.SpanId, span.ParentSpanId, span.Name, span.SpanKind, span.StatusCode, source.GetStatus().GetMessage(),
		span.ServiceName, span.ScopeName, group.Scope.GetVersion(), codec.Nanos(source.StartTimeUnixNano), codec.Nanos(source.EndTimeUnixNano),
		int64(span.Duration), codec.Time(row.PartitionTime), source.TraceState, codec.Uint32(source.Flags),
		codec.Uint32(source.DroppedAttributesCount), codec.Uint32(source.DroppedEventsCount), codec.Uint32(source.DroppedLinksCount),
		group.ResourceSchemaUrl, group.ScopeSchemaUrl,
	}, nil
}

// NestedValues holds the variable-size parts of a span the way every backend stores them: the attributes as a string
// map, then resource, scope, events and links as JSON text. Resource and scope are encoded once per group.
func (row *OtelSpanRow) NestedValues() ([]any, error) {
	attributes := StringAttributes(row.Source.GetAttributes())
	if row.Synthesized {
		for key, value := range row.Span.Attributes {
			attributes[key] = value
		}
	}
	resource, err := row.Group.Memo("resource", func() (string, error) {
		return MetadataJSON(row.Group.Resource, row.Group.Resource.GetAttributes())
	})
	if err != nil {
		return nil, err
	}
	scope, err := row.Group.Memo("scope", func() (string, error) {
		return MetadataJSON(row.Group.Scope, row.Group.Scope.GetAttributes())
	})
	if err != nil {
		return nil, err
	}
	events, err := EventsJSON(row.Source.GetEvents())
	if err != nil {
		return nil, err
	}
	links, err := LinksJSON(row.Source.GetLinks())
	if err != nil {
		return nil, err
	}
	return []any{attributes, resource, scope, events, links}, nil
}

// Order must match the columns OtelTextStorageColumns adds after OtelScalarColumns.
func (row *OtelSpanRow) TextValues() ([]any, error) {
	values, err := row.NestedValues()
	if err != nil {
		return nil, err
	}
	attributes, err := json.Marshal(values[0])
	if err != nil {
		return nil, err
	}
	values[0] = string(attributes)
	return append(values, row.Payload()), nil
}
