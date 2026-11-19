package contract

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestContract_otelGraphLateParent(t *testing.T) {
	trace := uuid.New()
	rootBytes := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	sourceRoot := hex.EncodeToString(rootBytes)
	now := time.Now().UTC()
	traceHex := hex.EncodeToString(trace[:])
	makeSpan := func(id, parent string, kind int) map[string]any {
		return map[string]any{"traceId": traceHex, "spanId": id, "parentSpanId": parent, "name": "otel-graph-contract", "kind": kind,
			"startTimeUnixNano": fmt.Sprint(now.UnixNano()), "endTimeUnixNano": fmt.Sprint(now.Add(time.Millisecond).UnixNano())}
	}
	// Export the descendants before their product root in separate HTTP requests.
	for _, spans := range [][]map[string]any{
		{makeSpan("1112131415161718", sourceRoot, 1), makeSpan("2122232425262728", "1112131415161718", 1)},
		{makeSpan(sourceRoot, "", 5)},
	} {
		body, err := json.Marshal(map[string]any{"resourceSpans": []any{map[string]any{"scopeSpans": []any{map[string]any{"spans": spans}}}}})
		if err != nil {
			t.Fatal(err)
		}
		request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/api/otel/v1/traces", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+seedToken)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		payload, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			t.Fatalf("OTLP export: %d %s", response.StatusCode, payload)
		}
	}
	owner := uuid.NewSHA1(uuid.MustParse(projectID), append(trace[:], rootBytes...))
	c, rt := capturedClient()
	if _, err := c.GetTask(context.Background(), projectID, owner.String(), now); err != nil {
		t.Fatal(err)
	}
	var detail struct {
		Task struct {
			TraceId    string            `json:"traceId"`
			SpanId     string            `json:"spanId"`
			Attributes map[string]string `json:"attributes"`
		} `json:"task"`
		Spans []struct {
			TraceId      string `json:"traceId"`
			SpanId       string `json:"spanId"`
			ParentSpanId string `json:"parentSpanId"`
		} `json:"spans"`
	}
	if err := json.Unmarshal(rt.body, &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Spans) != 2 || detail.Task.TraceId != traceHex || detail.Task.SpanId != sourceRoot {
		t.Fatalf("task detail lost the ids its span arrived with: %s", rt.body)
	}
	for key := range detail.Task.Attributes {
		if strings.HasPrefix(key, "traceway.otel.") {
			t.Fatalf("identity belongs in traceId and spanId, not in attributes: %s", rt.body)
		}
	}
	for _, span := range detail.Spans {
		want := sourceRoot
		if span.SpanId == "2122232425262728" {
			want = "1112131415161718"
		}
		if span.TraceId != traceHex || span.ParentSpanId != want {
			t.Fatalf("span lost its source ids: %+v", span)
		}
	}
	// The dashed spelling of the same trace id is accepted and answered in hex.
	for _, id := range []string{traceHex, trace.String()} {
		resp, err := c.GetDistributedTrace(context.Background(), id, now)
		if err != nil {
			t.Fatal(err)
		}
		if resp.TraceId != traceHex || len(resp.Nodes) != 1 || len(resp.Nodes[0].Spans) != 2 || resp.Nodes[0].SpanId != sourceRoot {
			t.Fatalf("distributed graph contract for %s: %s", id, rt.body)
		}
	}
}

func TestContract_otelSpanPayloadAuthorization(t *testing.T) {
	trace := uuid.New()
	traceHex := hex.EncodeToString(trace[:])
	spanID := "f123456789abcdef"
	body := fmt.Sprintf(`{"resourceSpans":[{"schemaUrl":"resource-schema","scopeSpans":[{"scope":{"name":"test","version":"1.0"},"spans":[{"traceId":%q,"spanId":%q,"name":"full payload","events":[{"name":"event","attributes":[{"key":"value","value":{"intValue":"9223372036854775807"}}]}]}]}]}]}`, traceHex, spanID)
	request, _ := http.NewRequest(http.MethodPost, baseURL+"/api/otel/v1/traces", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+seedToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("ingest: %d", response.StatusCode)
	}
	// Every span read is anchored on a time and held to 24 hours either side of it.
	endpoint := baseURL + "/api/otel/spans/" + traceHex + "/" + spanID + "?projectId=" + projectID + "&at=" + url.QueryEscape(time.Now().UTC().Format(time.RFC3339Nano))
	for _, authorized := range []bool{false, true} {
		request, _ := http.NewRequest(http.MethodGet, endpoint, nil)
		if authorized {
			request.Header.Set("Authorization", "Bearer "+jwtToken)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		payload, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !authorized {
			if response.StatusCode != 401 {
				t.Fatalf("anonymous payload access: %d", response.StatusCode)
			}
			continue
		}
		if response.StatusCode != 200 {
			t.Fatalf("payload: %d %s", response.StatusCode, payload)
		}
		var decoded coltracepb.ExportTraceServiceRequest
		if err := proto.Unmarshal(payload, &decoded); err != nil {
			t.Fatal(err)
		}
		resource := decoded.ResourceSpans[0]
		span := resource.ScopeSpans[0].Spans[0]
		if resource.SchemaUrl != "resource-schema" || resource.ScopeSpans[0].Scope.Version != "1.0" || span.Events[0].Attributes[0].Value.GetIntValue() != 9223372036854775807 {
			t.Fatalf("lost OTLP metadata: %v", &decoded)
		}
	}
}
