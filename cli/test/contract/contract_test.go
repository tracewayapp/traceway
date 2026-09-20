package contract

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	client "github.com/tracewayapp/traceway/cli/pkg/client"
)

// timeWindow brackets the seeded instant so list/query endpoints return it.
func timeWindow() client.TimeRange {
	return client.TimeRangeFromExplicit(seedAt.Add(-time.Hour), seedAt.Add(time.Hour))
}

// Each endpoint is driven through the real CLI client (so the request encoding
// is the contract under test) while a capturing transport snapshots the raw
// response for the wire-shape golden.

func TestContract_login(t *testing.T) {
	c, rt := capturedClient()
	if _, err := c.Login(context.Background(), seedEmail, seedPassword); err != nil {
		t.Fatalf("login: %v", err)
	}
	goldenAssert(t, "login", rt.body)
}

func TestContract_projectsList(t *testing.T) {
	c, rt := capturedClient()
	projects, err := c.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("projects: %v", err)
	}
	if len(projects) == 0 {
		t.Fatal("expected at least one project")
	}
	goldenAssert(t, "projects-list", rt.body)
}

func TestContract_endpointsGrouped(t *testing.T) {
	c, rt := capturedClient()
	resp, err := c.ListEndpoints(context.Background(), projectID, client.ListEndpointsRequest{
		TimeRange:  timeWindow(),
		Pagination: client.PaginationParams{Page: 1, PageSize: 50},
		OrderBy:    "impact",
	})
	if err != nil {
		t.Fatalf("endpoints grouped: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("expected seeded endpoint in response")
	}
	goldenAssert(t, "endpoints-grouped", rt.body)
}

func TestContract_endpointDetail(t *testing.T) {
	c, rt := capturedClient()
	resp, err := c.GetEndpoint(context.Background(), projectID, seedEndpointID.String(), seedAt)
	if err != nil {
		t.Fatalf("endpoint detail: %v", err)
	}
	// The waterfall is everything under the endpoint's span, the task's spans included.
	if resp.Endpoint == nil || resp.Endpoint.TraceId != seedTraceID || resp.Endpoint.SpanId != seedEndpointSpanID || resp.Endpoint.LinkedTraceId != seedLinkedTraceID || len(resp.Spans) != 4 {
		t.Errorf("endpoint ids or waterfall wrong: %s", rt.body)
	}
	if resp.Exception == nil || resp.Exception.ExceptionHash != seedHash {
		t.Errorf("the exception recorded below the endpoint's span is missing: %s", rt.body)
	}
	goldenAssert(t, "endpoint-detail", rt.body)
}

func TestContract_endpointsChart(t *testing.T) {
	c, rt := capturedClient()
	resp, err := c.GetEndpointChart(context.Background(), projectID, client.EndpointChartRequest{
		TimeRange:       timeWindow(),
		MetricType:      "p95",
		IntervalMinutes: 5,
	})
	if err != nil {
		t.Fatalf("endpoints chart: %v", err)
	}
	if len(resp.Series) == 0 {
		t.Fatal("expected seeded endpoint to produce a chart series point")
	}
	goldenAssert(t, "endpoints-chart", rt.body)
}

func TestContract_endpointsSlow(t *testing.T) {
	c, rt := capturedClient()
	// The seeded endpoint is not marked slow; the server still returns the
	// {offsetMs, reason} shape (offsetMs 0), which is what the golden locks.
	if _, err := c.GetSlowEndpoint(context.Background(), projectID, "GET /api/contract"); err != nil {
		t.Fatalf("endpoints slow: %v", err)
	}
	goldenAssert(t, "endpoints-slow", rt.body)
}

func TestContract_exceptionsGrouped(t *testing.T) {
	c, rt := capturedClient()
	resp, err := c.ListExceptions(context.Background(), projectID, client.ListExceptionsRequest{
		TimeRange:  timeWindow(),
		Pagination: client.PaginationParams{Page: 1, PageSize: 50},
		OrderBy:    "lastSeen",
	})
	if err != nil {
		t.Fatalf("exceptions grouped: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("expected seeded exception group in response")
	}
	goldenAssert(t, "exceptions-grouped", rt.body)
}

func TestContract_exceptionByHash(t *testing.T) {
	c, rt := capturedClient()
	resp, err := c.GetException(context.Background(), projectID, seedHash, client.PaginationParams{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("exception by hash: %v", err)
	}
	assertRelatedEndpoint(t, resp.RelatedEntity, rt.body)
	goldenAssert(t, "exception-by-hash", rt.body)
}

func TestContract_exceptionById(t *testing.T) {
	c, rt := capturedClient()
	resp, err := c.GetExceptionById(context.Background(), projectID, seedExceptionID.String(), seedAt)
	if err != nil {
		t.Fatalf("exception by id: %v", err)
	}
	if resp.Exception == nil || resp.Exception.TraceId != seedTraceID || resp.Exception.SpanId != seedChildSpanID || resp.Exception.LinkedTraceId != seedLinkedTraceID {
		t.Errorf("occurrence lost its trace ids: %s", rt.body)
	}
	assertRelatedEndpoint(t, resp.RelatedEntity, rt.body)
	goldenAssert(t, "exception-by-id", rt.body)
}

// The seeded exception sits on a span under the endpoint's, so the server has
// to walk up the trace to name the endpoint.
func assertRelatedEndpoint(t *testing.T, related *client.RelatedEntity, body []byte) {
	t.Helper()
	if related == nil || related.TraceType != "endpoint" || related.Id != seedEndpointID || related.Name != "GET /api/contract" || related.TraceId != seedTraceID {
		t.Errorf("relatedEntity should name the seeded endpoint: %s", body)
	}
}

func TestContract_exceptionArchiveRoundTrip(t *testing.T) {
	c := client.New(baseURL, client.WithJWT(jwtToken))
	ctx := context.Background()
	if err := c.ArchiveExceptions(ctx, projectID, []string{seedHash}); err != nil {
		t.Fatalf("archive: %v", err)
	}
	// Restore state so other tests still see the (unarchived) exception.
	if err := c.UnarchiveExceptions(ctx, projectID, []string{seedHash}); err != nil {
		t.Fatalf("unarchive: %v", err)
	}
}

func TestContract_logsQuery(t *testing.T) {
	c, rt := capturedClient()
	resp, err := c.QueryLogs(context.Background(), projectID, client.QueryLogsRequest{
		TimeRange:  timeWindow(),
		Pagination: client.PaginationParams{Page: 1, PageSize: 50},
	})
	if err != nil {
		t.Fatalf("logs query: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("expected seeded log record in response")
	}
	goldenAssert(t, "logs-query", rt.body)
}

func TestContract_metricsQuery(t *testing.T) {
	c, rt := capturedClient()
	resp, err := c.QueryMetrics(context.Background(), projectID, client.QueryMetricsRequest{
		TimeRange:       timeWindow(),
		IntervalMinutes: 60,
		Queries: []client.MetricQueryItem{
			{Name: seedMetricName, Aggregation: "avg"},
		},
	})
	if err != nil {
		t.Fatalf("metrics query: %v", err)
	}
	if !hasMetricValue(resp, seedMetricVal) {
		t.Fatalf("seeded metric value %v not found in response: %+v", seedMetricVal, resp)
	}
	goldenAssert(t, "metrics-query", rt.body)
}

func TestContract_taskDetail(t *testing.T) {
	c, rt := capturedClient()
	resp, err := c.GetTask(context.Background(), projectID, seedTaskID.String(), seedAt)
	if err != nil {
		t.Fatalf("task detail: %v", err)
	}
	// The exception sits above the task's span, so it is not the task's.
	if resp.Task == nil || resp.Task.SpanId != seedTaskSpanID || resp.Task.ParentSpanId != seedChildSpanID || len(resp.Spans) != 1 || resp.Exception != nil {
		t.Errorf("task ids, waterfall or exception wrong: %s", rt.body)
	}
	goldenAssert(t, "task-detail", rt.body)
}

func TestContract_aiTraceDetail(t *testing.T) {
	c, rt := capturedClient()
	if _, err := c.GetAiTrace(context.Background(), projectID, seedAiTraceID.String(), seedAt); err != nil {
		t.Fatalf("ai trace detail: %v", err)
	}
	goldenAssert(t, "aitrace-detail", rt.body)
}

func TestContract_sessionDetail(t *testing.T) {
	c, rt := capturedClient()
	if _, err := c.GetSession(context.Background(), projectID, seedSessionID.String(), seedAt); err != nil {
		t.Fatalf("session detail: %v", err)
	}
	goldenAssert(t, "session-detail", rt.body)
}

func TestContract_distributedTrace(t *testing.T) {
	c, rt := capturedClient()
	resp, err := c.GetDistributedTrace(context.Background(), seedTraceID, seedAt)
	if err != nil {
		t.Fatalf("distributed trace: %v", err)
	}
	wire := rt.body
	// The exception was recorded under the endpoint, so it rides on that node.
	parents := map[string]string{}
	for _, node := range resp.Nodes {
		parents[node.TraceType] = node.ParentEntitySpanId
		if node.TraceType == "endpoint" && node.Exception == nil {
			t.Error("the endpoint node lost the exception recorded below its span")
		}
	}
	if len(resp.Nodes) != 3 || parents["task"] != seedEndpointSpanID || parents["ai_trace"] != seedEndpointSpanID || parents["endpoint"] != "" {
		t.Fatalf("expected the task and the AI trace nested under the endpoint: %s", wire)
	}
	// The linked id names the same trace.
	linked, err := c.GetDistributedTrace(context.Background(), seedLinkedTraceID, seedAt)
	if err != nil {
		t.Fatalf("distributed trace by linked id: %v", err)
	}
	if len(linked.Nodes) != len(resp.Nodes) {
		t.Errorf("the linked trace id returned %d nodes, the trace id %d", len(linked.Nodes), len(resp.Nodes))
	}
	goldenAssert(t, "distributed-trace", wire)
}

// TestContract_cliMetricsValueRoundTrip drives the actual binary end to end and
// asserts the seeded value surfaces. With the lowercase json tags on
// TimeSeriesPoint this decodes by exact tag; the wire-shape golden above is what
// pins the casing so a regression to PascalCase would be caught.
func TestContract_cliMetricsValueRoundTrip(t *testing.T) {
	stdout, stderr, code := runCLI(t, "",
		"metrics", "query", "--name", seedMetricName, "--aggregation", "avg",
		"--since", "24h", "--interval-minutes", "60", "--output", "json")
	if code != 0 {
		t.Fatalf("metrics query exit %d\nstderr: %s", code, stderr)
	}
	var resp client.QueryMetricsResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("metrics output not JSON: %v\n%s", err, stdout)
	}
	if !hasMetricValue(&resp, seedMetricVal) {
		t.Fatalf("CLI metrics output missing seeded value %v: %s", seedMetricVal, stdout)
	}
}

func hasMetricValue(resp *client.QueryMetricsResponse, want float64) bool {
	for _, r := range resp.Results {
		for _, points := range r.Series {
			for _, p := range points {
				if p.Value == want {
					return true
				}
			}
		}
	}
	return false
}
