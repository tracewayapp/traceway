package otelcontrollers

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// OTLP uses hex IDs, whereas ordinary protobuf JSON encodes bytes as base64.
// Walking the descriptor avoids rewriting similarly named user attributes.
func normalizeOTLPJSON(body []byte, descriptor protoreflect.MessageDescriptor) ([]byte, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil {
		return nil, err
	}
	for key, value := range object {
		field := descriptor.Fields().ByJSONName(key)
		if field == nil {
			field = descriptor.Fields().ByName(protoreflect.Name(key))
		}
		if field == nil {
			continue
		}
		if field.Kind() == protoreflect.BytesKind && (field.Name() == "trace_id" || field.Name() == "span_id" || field.Name() == "parent_span_id") {
			var id string
			if err := json.Unmarshal(value, &id); err != nil {
				return nil, err
			}
			decoded, err := hex.DecodeString(id)
			if err != nil {
				return nil, fmt.Errorf("invalid hexadecimal %s", key)
			}
			object[key], _ = json.Marshal(base64.StdEncoding.EncodeToString(decoded))
		} else if field.Kind() == protoreflect.MessageKind && (field.Name() == "resource_spans" || field.Name() == "scope_spans" || field.Name() == "spans" || field.Name() == "links" ||
			field.Name() == "resource_logs" || field.Name() == "scope_logs" || field.Name() == "log_records") {
			var values []json.RawMessage
			if err := json.Unmarshal(value, &values); err != nil {
				return nil, err
			}
			for i := range values {
				normalized, err := normalizeOTLPJSON(values[i], field.Message())
				if err != nil {
					return nil, err
				}
				values[i] = normalized
			}
			object[key], _ = json.Marshal(values)
		}
	}
	return json.Marshal(object)
}
