package clientmodels

import "testing"

func TestSessionUsesCanonicalTraceID(t *testing.T) {
	const traceID = "0102030405060708090a0b0c0d0e0f10"
	for _, value := range []string{
		traceID,
		"01020304-0506-0708-090A-0B0C0D0E0F10",
	} {
		session := ClientSession{LegacyDistributedTraceId: value}
		if got := session.ToSession("", "").TraceId; got != traceID {
			t.Fatalf("session trace ID = %q, want %q", got, traceID)
		}
	}
	for _, value := range []string{"", "00000000000000000000000000000000", "invalid"} {
		session := ClientSession{LegacyDistributedTraceId: value}
		if got := session.ToSession("", "").TraceId; got != "" {
			t.Fatalf("invalid session trace ID became an association: %q", got)
		}
	}
}
