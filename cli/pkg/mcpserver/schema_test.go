package mcpserver

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// TestOutSchema_matchesMarshaledShape validates a marshaled response against
// its generated schema, covering the tricky encodings: uuid.UUID as string,
// nil maps as null, nil pointer structs as null, and json.RawMessage as
// arbitrary JSON. The contract suite covers the same end to end against a
// real backend; this pins the mechanism without one.
func TestOutSchema_matchesMarshaledShape(t *testing.T) {
	traceID := uuid.New()
	resp := client.EndpointDetailResponse{
		Endpoint: &client.Endpoint{
			Id:                 uuid.New(),
			ProjectId:          uuid.New(),
			Endpoint:           "GET /api/users",
			DistributedTraceId: &traceID,
		},
		Spans: []client.Span{{Id: uuid.New(), TraceId: traceID, ProjectId: uuid.New(), Name: "db"}},
	}
	validateAgainstSchema(t, resp)

	byID := client.ExceptionByIdResponse{
		Exception:        &client.ExceptionStackTrace{Id: uuid.New(), ExceptionHash: "abc123def4567890"},
		SessionRecording: json.RawMessage(`{"segments": [1, 2]}`),
	}
	validateAgainstSchema(t, byID)

	metrics := client.QueryMetricsResponse{
		Results: []client.MetricQueryResult{{Name: "cpu", Series: map[string][]client.TimeSeriesPoint{"all": {{Value: 1.5}}}}},
	}
	validateAgainstSchema(t, metrics)
}

func validateAgainstSchema[T any](t *testing.T, v T) {
	t.Helper()
	resolved, err := outSchema[T]().Resolve(nil)
	if err != nil {
		t.Fatalf("resolve schema for %T: %v", v, err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(&m); err != nil {
		t.Errorf("marshaled %T does not validate against its schema: %v\njson: %s", v, err, b)
	}
}
