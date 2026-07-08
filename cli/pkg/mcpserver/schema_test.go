package mcpserver

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/tracewayapp/traceway/cli/pkg/client"
)

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
