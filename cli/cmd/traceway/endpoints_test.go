package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEndpointsList_table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"data":[
				{"endpoint":"GET /api/projects","count":120,"p50Duration":50000000,"p95Duration":150000000,"p99Duration":300000000,"avgDuration":80000000,"lastSeen":"2026-05-13T12:00:00Z","impact":0.42,"impactReason":"high p95"}
			],
			"pagination":{"total":1}
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "endpoints", "list", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "ENDPOINT") || !strings.Contains(out, "P50") || !strings.Contains(out, "P95") || !strings.Contains(out, "P99") {
		t.Errorf("table missing headers: %s", out)
	}
	if !strings.Contains(out, "/api/projects") || !strings.Contains(out, "120") {
		t.Errorf("table missing row data: %s", out)
	}
	// Latency should be human-formatted (50ms, not 50000000)
	if strings.Contains(out, "50000000") {
		t.Errorf("table should format ns as human duration: %s", out)
	}
}

func TestEndpointsList_jsonShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"endpoint":"GET /","count":1}],"pagination":{}}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "endpoints", "list", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"GET /"`) {
		t.Errorf("missing endpoint in JSON: %s", stdout.String())
	}
}

const chartResponseJSON = `{
	"endpoints":["GET /api/checkout","GET /api/projects","Other"],
	"series":[
		{"timestamp":"2026-06-25T12:00:00Z","endpoint":"GET /api/checkout","value":120.5},
		{"timestamp":"2026-06-25T13:00:00Z","endpoint":"GET /api/checkout","value":480.0},
		{"timestamp":"2026-06-25T12:00:00Z","endpoint":"GET /api/projects","value":30.0},
		{"timestamp":"2026-06-25T13:00:00Z","endpoint":"GET /api/projects","value":35.0}
	]
}`

func TestEndpointsChart_jsonShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(chartResponseJSON))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "endpoints", "chart", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"GET /api/checkout"`) || !strings.Contains(out, "480") {
		t.Errorf("json missing series data: %s", out)
	}
}

func TestEndpointsChart_table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(chartResponseJSON))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "endpoints", "chart", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "ENDPOINT") || !strings.Contains(out, "MIN(ms)") || !strings.Contains(out, "MAX(ms)") || !strings.Contains(out, "LATEST(ms)") {
		t.Errorf("table missing headers: %s", out)
	}
	// The slow endpoint's peak should surface as its MAX and its later bucket as LATEST.
	if !strings.Contains(out, "GET /api/checkout") || !strings.Contains(out, "480.0") {
		t.Errorf("table missing endpoint summary: %s", out)
	}
	// Endpoints with no points (the "Other" bucket here) still render a row.
	if !strings.Contains(out, "Other") {
		t.Errorf("table missing Other row: %s", out)
	}
}

func TestEndpointsChart_sendsRequestBody(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		if r.URL.Query().Get("projectId") != "proj-1" {
			t.Errorf("projectId = %q", r.URL.Query().Get("projectId"))
		}
		_, _ = w.Write([]byte(chartResponseJSON))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, _, err := runCmd(t, "", "endpoints", "chart", "--metric-type", "p99", "--interval-minutes", "60", "--since", "24h", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/endpoints/chart" {
		t.Errorf("path = %q, want /api/endpoints/chart", gotPath)
	}
	if gotBody["metricType"] != "p99" {
		t.Errorf("metricType = %v, want p99", gotBody["metricType"])
	}
	if gotBody["intervalMinutes"] != float64(60) {
		t.Errorf("intervalMinutes = %v, want 60", gotBody["intervalMinutes"])
	}
	// Endpoints routes use fromDate/toDate (not from/to like metrics).
	if gotBody["fromDate"] == nil || gotBody["toDate"] == nil {
		t.Errorf("body missing fromDate/toDate: %v", gotBody)
	}
	if gotBody["from"] != nil || gotBody["to"] != nil {
		t.Errorf("body should not carry from/to: %v", gotBody)
	}
}

func TestEndpointsChart_metricTypeValidation(t *testing.T) {
	// Bogus --metric-type is rejected client-side, before any request is sent.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be called for invalid --metric-type")
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "endpoints", "chart", "--metric-type", "nope", "--output", "json")
	if err == nil {
		t.Fatal("expected error for bogus --metric-type")
	}
	if !strings.Contains(stderr.String(), "--metric-type") {
		t.Errorf("expected --metric-type in error, got: %s", stderr.String())
	}

	// Every documented metric type is accepted.
	for _, mt := range endpointChartMetricTypes {
		okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(chartResponseJSON))
		}))
		seedSessionFor(t, okSrv.URL)
		if _, stderr, err := runCmd(t, "", "endpoints", "chart", "--metric-type", mt, "--output", "json"); err != nil {
			okSrv.Close()
			t.Fatalf("metric-type %q rejected: %v\nstderr: %s", mt, err, stderr.String())
		}
		okSrv.Close()
	}
}

func TestEndpointsSlow_marked(t *testing.T) {
	var gotPath, gotMethod, gotEndpoint string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotEndpoint = r.URL.Query().Get("endpoint")
		_, _ = w.Write([]byte(`{"offsetMs":2000,"reason":"third-party export, accepted"}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "endpoints", "slow", "GET /api/checkout", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "MARKED:") || !strings.Contains(out, "yes") || !strings.Contains(out, "2000ms") {
		t.Errorf("table missing marked summary: %s", out)
	}
	if !strings.Contains(out, "third-party export") {
		t.Errorf("table missing reason: %s", out)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/endpoints/slow" {
		t.Errorf("path = %q, want /api/endpoints/slow", gotPath)
	}
	if gotEndpoint != "GET /api/checkout" {
		t.Errorf("endpoint query = %q, want %q", gotEndpoint, "GET /api/checkout")
	}
}

func TestEndpointsSlow_unmarked(t *testing.T) {
	// The server returns offsetMs 0 / empty reason for an endpoint never marked.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"offsetMs":0,"reason":""}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "endpoints", "slow", "GET /api/healthz", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "no") || !strings.Contains(out, "default thresholds") {
		t.Errorf("unmarked endpoint should render 'no': %s", out)
	}
}

func TestEndpointsSlow_jsonShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"offsetMs":2000,"reason":"batch job"}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "endpoints", "slow", "GET /api/x", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout.String())
	}
	if got["offsetMs"] != float64(2000) || got["reason"] != "batch job" {
		t.Errorf("json = %v", got)
	}
}

func TestEndpointsSlow_requiresEndpointArg(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be called without an endpoint arg")
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	if _, _, err := runCmd(t, "", "endpoints", "slow", "--output", "json"); err == nil {
		t.Fatal("expected error when endpoint arg is missing")
	}
}
