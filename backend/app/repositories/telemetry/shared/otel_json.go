package shared

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math"
	"strconv"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func JSONAttributes(attributes []*commonpb.KeyValue) map[string]any {
	result := make(map[string]any, len(attributes))
	for _, attribute := range attributes {
		result[attribute.Key] = jsonAttributeValue(attribute.Value)
	}
	return result
}

// StringAttributes is the span_attributes value on every backend, a Map on ClickHouse and a JSON object elsewhere. It
// follows the OpenTelemetry Collector's ClickHouse exporter: keys verbatim, scalars as text, arrays and key/value lists
// as JSON. Exact types stay in the lossless payload.
func StringAttributes(attributes []*commonpb.KeyValue) map[string]string {
	result := make(map[string]string, len(attributes))
	for _, attribute := range attributes {
		if attribute.Key == "exception.stacktrace" {
			continue
		}
		switch value := jsonAttributeValue(attribute.Value).(type) {
		case nil:
		case string:
			result[attribute.Key] = value
		default:
			encoded, _ := json.Marshal(value)
			result[attribute.Key] = string(encoded)
		}
	}
	return result
}

func jsonAttributeValue(value *commonpb.AnyValue) any {
	if value == nil {
		return nil
	}
	switch v := value.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		return v.StringValue
	case *commonpb.AnyValue_IntValue:
		return v.IntValue
	case *commonpb.AnyValue_DoubleValue:
		if math.IsNaN(v.DoubleValue) || math.IsInf(v.DoubleValue, 0) {
			return strconv.FormatFloat(v.DoubleValue, 'g', -1, 64)
		}
		return v.DoubleValue
	case *commonpb.AnyValue_BoolValue:
		return v.BoolValue
	case *commonpb.AnyValue_BytesValue:
		return base64.StdEncoding.EncodeToString(v.BytesValue)
	case *commonpb.AnyValue_ArrayValue:
		values := make([]any, len(v.ArrayValue.Values))
		for i, value := range v.ArrayValue.Values {
			values[i] = jsonAttributeValue(value)
		}
		return values
	case *commonpb.AnyValue_KvlistValue:
		return JSONAttributes(v.KvlistValue.Values)
	default:
		return nil
	}
}

func MetadataJSON(message proto.Message, attributes []*commonpb.KeyValue) (string, error) {
	encoded, err := protojson.Marshal(message)
	if err != nil {
		return "", err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return "", err
	}
	text, err := json.Marshal(JSONAttributes(attributes))
	if err != nil {
		return "", err
	}
	fields["attributes"] = text
	encoded, err = json.Marshal(fields)
	return string(encoded), err
}

type eventText struct {
	TimeUnixNano           uint64         `json:"time_unix_nano"`
	Name                   string         `json:"name"`
	Attributes             map[string]any `json:"attributes"`
	DroppedAttributesCount uint32         `json:"dropped_attributes_count"`
}

type linkText struct {
	TraceId                string         `json:"trace_id"`
	SpanId                 string         `json:"span_id"`
	TraceState             string         `json:"trace_state"`
	Flags                  uint32         `json:"flags"`
	Attributes             map[string]any `json:"attributes"`
	DroppedAttributesCount uint32         `json:"dropped_attributes_count"`
}

func EventsJSON(events []*tracepb.Span_Event) (string, error) {
	if len(events) == 0 {
		return "[]", nil
	}
	values := make([]eventText, len(events))
	for i, event := range events {
		values[i] = eventText{event.TimeUnixNano, event.Name, JSONAttributes(event.Attributes), event.DroppedAttributesCount}
	}
	encoded, err := json.Marshal(values)
	return string(encoded), err
}

func LinksJSON(links []*tracepb.Span_Link) (string, error) {
	if len(links) == 0 {
		return "[]", nil
	}
	values := make([]linkText, len(links))
	for i, link := range links {
		values[i] = linkText{hex.EncodeToString(link.TraceId), hex.EncodeToString(link.SpanId), link.TraceState, link.Flags, JSONAttributes(link.Attributes), link.DroppedAttributesCount}
	}
	encoded, err := json.Marshal(values)
	return string(encoded), err
}
