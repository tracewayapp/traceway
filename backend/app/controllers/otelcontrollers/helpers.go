package otelcontrollers

import (
	"strconv"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

func validOtelID(id []byte, size int) bool {
	if len(id) != size {
		return false
	}
	for _, b := range id {
		if b != 0 {
			return true
		}
	}
	return false
}

func extractAttributes(attrs []*commonpb.KeyValue) map[string]string {
	if len(attrs) == 0 {
		return nil
	}
	m := make(map[string]string, len(attrs))
	for _, kv := range attrs {
		if kv.Value == nil {
			continue
		}
		switch v := kv.Value.Value.(type) {
		case *commonpb.AnyValue_StringValue:
			m[kv.Key] = v.StringValue
		case *commonpb.AnyValue_IntValue:
			m[kv.Key] = strconv.FormatInt(v.IntValue, 10)
		case *commonpb.AnyValue_DoubleValue:
			m[kv.Key] = strconv.FormatFloat(v.DoubleValue, 'g', -1, 64)
		case *commonpb.AnyValue_BoolValue:
			m[kv.Key] = strconv.FormatBool(v.BoolValue)
		}
	}
	return m
}

func getStringAttribute(attrs []*commonpb.KeyValue, key string) string {
	for _, kv := range attrs {
		if kv.Key == key && kv.Value != nil {
			if sv, ok := kv.Value.Value.(*commonpb.AnyValue_StringValue); ok {
				return sv.StringValue
			}
		}
	}
	return ""
}

func getIntAttribute(attrs []*commonpb.KeyValue, key string) (int64, bool) {
	for _, kv := range attrs {
		if kv.Key == key && kv.Value != nil {
			if iv, ok := kv.Value.Value.(*commonpb.AnyValue_IntValue); ok {
				return iv.IntValue, true
			}
		}
	}
	return 0, false
}

func getFloatAttribute(attrs []*commonpb.KeyValue, key string) float64 {
	for _, kv := range attrs {
		if kv.Key == key && kv.Value != nil {
			switch v := kv.Value.Value.(type) {
			case *commonpb.AnyValue_DoubleValue:
				return v.DoubleValue
			case *commonpb.AnyValue_IntValue:
				return float64(v.IntValue)
			}
		}
	}
	return 0
}

// getStringValues returns all string values for an attribute. Handles both
// scalar string (`AnyValue_StringValue`) and the array form OTel uses for
// captured response headers (`AnyValue_ArrayValue` of `AnyValue_StringValue`).
func getStringValues(attrs []*commonpb.KeyValue, key string) []string {
	var out []string
	for _, kv := range attrs {
		if kv.Key != key || kv.Value == nil {
			continue
		}
		switch v := kv.Value.Value.(type) {
		case *commonpb.AnyValue_StringValue:
			if v.StringValue != "" {
				out = append(out, v.StringValue)
			}
		case *commonpb.AnyValue_ArrayValue:
			if v.ArrayValue == nil {
				continue
			}
			for _, item := range v.ArrayValue.Values {
				if item == nil {
					continue
				}
				if sv, ok := item.Value.(*commonpb.AnyValue_StringValue); ok && sv.StringValue != "" {
					out = append(out, sv.StringValue)
				}
			}
		}
	}
	return out
}

func getStringArray(attrs []*commonpb.KeyValue, key string) []string {
	for _, kv := range attrs {
		if kv.Key != key || kv.Value == nil {
			continue
		}
		av, ok := kv.Value.Value.(*commonpb.AnyValue_ArrayValue)
		if !ok || av.ArrayValue == nil {
			continue
		}
		out := make([]string, 0, len(av.ArrayValue.Values))
		for _, item := range av.ArrayValue.Values {
			s := ""
			if item != nil {
				if sv, ok := item.Value.(*commonpb.AnyValue_StringValue); ok {
					s = sv.StringValue
				}
			}
			out = append(out, s)
		}
		return out
	}
	return nil
}

func getIntArray(attrs []*commonpb.KeyValue, key string) []int64 {
	for _, kv := range attrs {
		if kv.Key != key || kv.Value == nil {
			continue
		}
		av, ok := kv.Value.Value.(*commonpb.AnyValue_ArrayValue)
		if !ok || av.ArrayValue == nil {
			continue
		}
		out := make([]int64, 0, len(av.ArrayValue.Values))
		for _, item := range av.ArrayValue.Values {
			var n int64
			if item != nil {
				switch iv := item.Value.(type) {
				case *commonpb.AnyValue_IntValue:
					n = iv.IntValue
				case *commonpb.AnyValue_DoubleValue:
					n = int64(iv.DoubleValue)
				}
			}
			out = append(out, n)
		}
		return out
	}
	return nil
}
