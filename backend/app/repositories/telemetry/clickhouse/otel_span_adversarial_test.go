//go:build telemetry_ch

package clickhouse

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestOtelSpanAdversarialAttributes(t *testing.T) {
	ctx, conn := scratchDatabase(t)
	previous := chdb.Conn
	chdb.Conn = conn
	defer func() { chdb.Conn = previous }()
	createSpansTable(t, ctx, conn)

	double := func(v float64) *commonpb.AnyValue {
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: v}}
	}
	array := func(v ...*commonpb.AnyValue) *commonpb.AnyValue {
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: v}}}
	}
	kvlist := func(v ...*commonpb.KeyValue) *commonpb.AnyValue {
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_KvlistValue{KvlistValue: &commonpb.KeyValueList{Values: v}}}
	}
	deep := benchValue("leaf")
	for range 120 {
		deep = kvlist(benchKV("n", deep))
	}
	many := make([]*commonpb.KeyValue, 3000)
	for i := range many {
		many[i] = benchKV(fmt.Sprintf("bulk.key.%d", i), benchInt(int64(i)))
	}
	cases := map[string][]*commonpb.KeyValue{
		"baseline":                 {benchKV("http.method", benchValue("GET"))},
		"empty key":                {benchKV("", benchValue("x"))},
		"dot only key":             {benchKV(".", benchValue("x"))},
		"double dot key":           {benchKV("a..b", benchValue("x"))},
		"trailing dot key":         {benchKV("a.", benchValue("x"))},
		"leading dot key":          {benchKV(".a", benchValue("x"))},
		"scalar and dotted prefix": {benchKV("a", benchValue("x")), benchKV("a.b", benchValue("y"))},
		"nested and dotted prefix": {benchKV("p", kvlist(benchKV("q", benchValue("x")))), benchKV("p.q", benchValue("y"))},
		"kvlist with empty key":    {benchKV("obj", kvlist(benchKV("", benchValue("x"))))},
		"empty kvlist":             {benchKV("obj", kvlist())},
		"empty array":              {benchKV("arr", array())},
		"array with nulls":         {benchKV("arr", array(&commonpb.AnyValue{}, benchInt(1)))},
		"array of mixed values":    {benchKV("arr", array(kvlist(benchKV("k", benchInt(1))), benchValue("s"), benchInt(2)))},
		"array of arrays":          {benchKV("arr", array(array(benchInt(1)), array(benchValue("a"))))},
		"array of different maps":  {benchKV("arr", array(kvlist(benchKV("a", benchInt(1))), kvlist(benchKV("a", benchValue("s")), benchKV("b", benchInt(2)))))},
		"unset value":              {benchKV("nothing", &commonpb.AnyValue{})},
		"nil value":                {{Key: "nilval"}},
		"minimum integer":          {benchKV("n", benchInt(math.MinInt64))},
		"extreme doubles":          {benchKV("d", double(math.MaxFloat64)), benchKV("tiny", double(5e-324)), benchKV("negzero", double(math.Copysign(0, -1)))},
		"not a number":             {benchKV("d", double(math.NaN())), benchKV("inf", double(math.Inf(-1)))},
		"quotes and control chars": {benchKV("we`ird \"key\"\n\twith'quotes\\and/slash", benchValue("va\"l\nue"))},
		"nul bytes":                {benchKV("nul\x00key", benchValue("va\x00lue"))},
		"unicode":                  {benchKV("ключ.名前.🔥", benchValue("значение"))},
		"html sensitive":           {benchKV("a<b>&c", benchValue("<script>&amp;"))},
		"long key":                 {benchKV(strings.Repeat("k", 5000), benchValue("x"))},
		"one megabyte value":       {benchKV("big", benchValue(strings.Repeat("v", 1<<20)))},
		"deep nesting":             {benchKV("deep", deep)},
		"three thousand keys":      many,
		"duplicate keys":           {benchKV("dup", benchValue("first")), benchKV("dup", benchInt(2))},
		"percent and escapes":      {benchKV("100%", benchValue("x")), benchKV("a%2Eb", benchValue("y")), benchKV("a.b", benchValue("z"))},
		"numeric keys":             {benchKV("0", benchValue("x")), benchKV("1.5", benchValue("y"))},
	}
	project, start, index := uuid.New(), time.Now().UTC().Truncate(time.Microsecond), 0
	for name, attributes := range cases {
		index++
		trace := fmt.Sprintf("%032x", index)
		source := &tracepb.Span{TraceId: make([]byte, 16), SpanId: []byte{0, 0, 0, 0, 0, 0, 0, byte(index)}, Name: name, Attributes: attributes,
			StartTimeUnixNano: uint64(start.UnixNano()), EndTimeUnixNano: uint64(start.Add(time.Millisecond).UnixNano()),
			Events: []*tracepb.Span_Event{{Name: "event", TimeUnixNano: uint64(start.UnixNano()), Attributes: attributes}},
			Links:  []*tracepb.Span_Link{{TraceId: []byte{1}, SpanId: []byte{2}, Attributes: attributes}}}
		source.TraceId[15] = byte(index)
		span := models.OtelSpan{OTLP: source, Span: models.Span{ProjectId: project, TraceId: trace, SpanId: fmt.Sprintf("%016x", index), Name: name, StartTime: start, Duration: time.Millisecond},
			Context: &tracepb.ResourceSpans{Resource: &resourcepb.Resource{Attributes: attributes}, ScopeSpans: []*tracepb.ScopeSpans{{Scope: &commonpb.InstrumentationScope{Name: "scope", Attributes: attributes}}}}}
		if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{span}); err != nil {
			t.Errorf("%s: insert rejected: %v", name, err)
			continue
		}
		found, err := OtelSpanRepository.FindTraceTopology(ctx, []shared.SpanLookup{{ProjectId: project, TraceId: trace, SpanId: span.SpanId}}, 0)
		if err != nil || len(found) != 1 {
			t.Errorf("%s: graph read: %d rows, %v", name, len(found), err)
			continue
		}
		display, err := OtelSpanRepository.FindSpanAttributes(ctx, shared.SpanLookup{ProjectId: project, TraceId: trace}, []string{span.SpanId}, shared.OtelAttributeLimits{PerSpanBytes: shared.MaxOtelSpanAttributeBytes, BudgetBytes: shared.MaxOtelGraphBytes})
		if err != nil {
			t.Fatalf("%s: display attributes: %v", name, err)
		}
		if name == "one megabyte value" && display[span.SpanId].Omitted != models.SpanGraphAttributeSize {
			t.Fatal("large attributes must be marked omitted")
		}
		payload, err := OtelSpanRepository.FindOTLP(ctx, project, trace, span.SpanId, start)
		if err != nil {
			t.Errorf("%s: payload read: %v", name, err)
			continue
		}
		expected := proto.Clone(span.Context).(*tracepb.ResourceSpans)
		expected.ScopeSpans[0].Spans = []*tracepb.Span{source}
		var restored tracepb.ResourceSpans
		if err := proto.Unmarshal(payload, &restored); err != nil || !proto.Equal(expected, &restored) {
			t.Errorf("%s: lossless payload changed: %v", name, err)
		}
		var validText uint8
		if err := conn.QueryRow(ctx, "SELECT isValidJSON(resource) AND isValidJSON(scope) AND isValidJSON(events) AND isValidJSON(links) FROM spans_v2 WHERE project_id = ? AND span_id = ?", project, span.SpanId).Scan(&validText); err != nil || validText != 1 {
			t.Errorf("%s: text columns are not valid JSON: %d %v", name, validText, err)
		}
	}
}
