package mcpserver

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/tracewayapp/traceway/cli/pkg/client"
)

func TestOutSchema_matchesMarshaledShape(t *testing.T) {
	traceID := "0af7651916cd43dd8448eb211c80319c"
	resp := client.EndpointDetailResponse{
		Endpoint: &client.Endpoint{
			Id:        uuid.New(),
			ProjectId: uuid.New(),
			Endpoint:  "GET /api/users",
			TraceId:   traceID,
			SpanId:    "b7ad6b7169203331",
		},
		SpanGraphStatus: &client.SpanGraphStatus{State: "partial", Reasons: []string{"row_limit"}},
		Spans:           []client.Span{{ProjectId: uuid.New(), TraceId: traceID, SpanId: "00f067aa0ba902b7", ParentSpanId: "b7ad6b7169203331", Name: "db"}},
	}
	validateAgainstSchema(t, resp)

	byID := client.ExceptionByIdResponse{
		Exception:        &client.ExceptionStackTrace{Id: uuid.New(), ExceptionHash: "abc123def4567890", TraceId: traceID, SpanId: "00f067aa0ba902b7"},
		RelatedEntity:    &client.RelatedEntity{TraceType: "endpoint", Id: uuid.New(), Name: "GET /api/users", TraceId: traceID},
		SessionRecording: json.RawMessage(`{"segments": [1, 2]}`),
	}
	validateAgainstSchema(t, byID)

	trace := client.DistributedTraceResponse{
		TraceId: traceID,
		Nodes: []client.DistributedTraceNode{{
			ProjectId: uuid.New(), ProjectName: "api", TraceType: "endpoint", TraceId: traceID, SpanId: "b7ad6b7169203331",
			Endpoint: resp.Endpoint, Spans: resp.Spans, ParentEntitySpanId: "53995c3f42cd8ad8",
		}},
	}
	validateAgainstSchema(t, trace)

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
