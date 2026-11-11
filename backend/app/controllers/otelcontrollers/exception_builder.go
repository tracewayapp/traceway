package otelcontrollers

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

const (
	attrSessionId           = "session.id"
	attrTracewayExceptionId = "traceway.exception.id"
	attrTracewayDebugIds    = "traceway.debug_ids"

	maxDebugIdsBytes = 64 << 10
	maxDebugIds      = 500
)

// Browser SDKs describe the user, browser and session on the resource rather than on each record, so these
// resource attributes are what an issue page needs to tell one client's exception from another's.
var clientResourcePrefixes = []string{"browser.", "os.", "device.", "user_agent.", "user.", "enduser.", "session.", "deployment."}

var clientResourceKeys = map[string]bool{"service.instance.id": true}

func firstStringAttribute(key string, sets ...[]*commonpb.KeyValue) string {
	for _, attrs := range sets {
		if v := getStringAttribute(attrs, key); v != "" {
			return v
		}
	}
	return ""
}

// debugIdsFrom reads the ECMA-426 debug ids the SDK found in the stack's bundles, a JSON object of file name to id.
func debugIdsFrom(sets ...[]*commonpb.KeyValue) map[string]string {
	raw := firstStringAttribute(attrTracewayDebugIds, sets...)
	if raw == "" || len(raw) > maxDebugIdsBytes {
		return nil
	}
	var ids map[string]string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil || len(ids) == 0 || len(ids) > maxDebugIds {
		return nil
	}
	return ids
}

// sessionIdFrom returns nil for a session id that is not a UUID: the session columns are UUID typed.
func sessionIdFrom(sets ...[]*commonpb.KeyValue) *uuid.UUID {
	raw := firstStringAttribute(attrSessionId, sets...)
	if raw == "" {
		return nil
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return nil
	}
	return &id
}

// exceptionIdFrom honours the id the SDK generated so a replay clip sent on another request can name the exception,
// and so a retried export writes the same id again.
func exceptionIdFrom(attrs []*commonpb.KeyValue, fallback func() uuid.UUID) uuid.UUID {
	if raw := getStringAttribute(attrs, attrTracewayExceptionId); raw != "" {
		if id, err := uuid.Parse(raw); err == nil && id != uuid.Nil {
			return id
		}
	}
	return fallback()
}

func clientResourceAttributes(resource []*commonpb.KeyValue) map[string]string {
	out := map[string]string{}
	for key, value := range extractAttributes(resource) {
		if isClientResourceKey(key) {
			out[key] = value
		}
	}
	return out
}

func isClientResourceKey(key string) bool {
	if clientResourceKeys[key] {
		return true
	}
	for _, prefix := range clientResourcePrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// exceptionAttributes lays the record's own attributes over the client resource attributes and drops the keys that
// only steer ingest, so they never show up on the issue page.
func exceptionAttributes(resource []*commonpb.KeyValue, own map[string]string) map[string]string {
	out := clientResourceAttributes(resource)
	for key, value := range own {
		out[key] = value
	}
	delete(out, "exception.stacktrace")
	delete(out, attrTracewayDebugIds)
	delete(out, attrTracewayExceptionId)
	return out
}
