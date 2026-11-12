package shared

import "github.com/google/uuid"

const (
	SessionIdAttribute = "session.id"
	// DebugIdsAttribute only steers symbolication at ingest. The lossless OTLP payload keeps it; no readable
	// attribute column does.
	DebugIdsAttribute = "traceway.debug_ids"
)

// CanonicalSessionId spells a session id the way session reads look it up, lowercase and dashed, whatever form
// uuid.Parse accepted from the SDK (upper case, no dashes, braces, urn:uuid:). Anything else is kept as sent.
func CanonicalSessionId(value string) string {
	if id, err := uuid.Parse(value); err == nil {
		return id.String()
	}
	return value
}

// NormalizeStoredAttributes prepares a readable attribute map for storage, in place: the session lookups (the
// ClickHouse session_id columns, the SQLite expression indexes) compare session.id with the canonical spelling.
func NormalizeStoredAttributes(attrs map[string]string) {
	delete(attrs, DebugIdsAttribute)
	if value, ok := attrs[SessionIdAttribute]; ok {
		attrs[SessionIdAttribute] = CanonicalSessionId(value)
	}
}
