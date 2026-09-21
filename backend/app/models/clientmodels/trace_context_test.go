package clientmodels

import (
	"encoding/json"
	"testing"
)

func TestReportIgnoresLegacyDistributedTraceID(t *testing.T) {
	const run = "11223344-5566-7788-99aa-bbccddeeff00"
	const runHex = "112233445566778899aabbccddeeff00"
	for _, value := range []any{
		"0102030405060708090a0b0c0d0e0f10",
		"01020304-0506-0708-090A-0B0C0D0E0F10",
		"", "00000000000000000000000000000000", "invalid", nil, 42,
	} {
		body, err := json.Marshal(map[string]any{"id": run, "traceId": run, "distributedTraceId": value})
		if err != nil {
			t.Fatal(err)
		}
		var trace ClientTrace
		var exception ClientExceptionStackTrace
		var session ClientSession
		for _, target := range []any{&trace, &exception, &session} {
			if err := json.Unmarshal(body, target); err != nil {
				t.Fatal(err)
			}
		}
		if trace.ToEndpoint("", "").TraceId != runHex || trace.ToTask("", "").TraceId != runHex {
			t.Fatalf("legacy field changed native run identity: %v", value)
		}
		got := exception.ToExceptionStackTrace("hash", "", "")
		if got.TraceId != runHex || got.SpanId != runHex {
			t.Fatalf("legacy field changed exception ownership: %+v", got)
		}
		if got := session.ToSession("", "").TraceId; got != "" {
			t.Fatalf("legacy field created session association: %q", got)
		}
	}
}
