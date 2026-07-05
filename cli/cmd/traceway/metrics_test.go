package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
)

func TestMetricsQuery_jsonOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"results":[
				{"name":"http.request.duration","unit":"ms","series":{"all":[{"timestamp":"2026-05-13T12:00:00Z","value":42.5}]}}
			]
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "metrics", "query", "--name", "http.request.duration", "--aggregation", "p95", "--output", "json")
	if err != nil {
		t.Fatalf("metrics query: %v", err)
	}
	if !strings.Contains(stdout.String(), "http.request.duration") {
		t.Errorf("expected metric name in output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "42.5") {
		t.Errorf("expected value in output: %s", stdout.String())
	}
}

func TestMetricsQuery_table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"results":[
				{"name":"http.request.duration","unit":"ms","series":{
					"all":[
						{"timestamp":"2026-05-13T12:00:00Z","value":42.5},
						{"timestamp":"2026-05-13T12:05:00Z","value":47.0}
					]
				}}
			]
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "metrics", "query", "--name", "http.request.duration", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "METRIC") || !strings.Contains(out, "GROUP") || !strings.Contains(out, "POINTS") {
		t.Errorf("table missing headers: %s", out)
	}
	if !strings.Contains(out, "http.request.duration") {
		t.Errorf("table missing metric name: %s", out)
	}
	if !strings.Contains(out, "47") {
		t.Errorf("table missing latest value: %s", out)
	}
}

func TestMetricsQuery_requiresName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "metrics", "query", "--output", "json")
	if err == nil {
		t.Fatal("expected --name to be required")
	}
	if !strings.Contains(stderr.String(), `"usage_error"`) {
		t.Errorf("expected usage_error envelope, got: %s", stderr.String())
	}
}

func TestMetricsQuery_aggregationAcceptsAllDocumentedValues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	for _, agg := range []string{"avg", "sum", "count", "min", "max", "p50", "p95", "p99"} {
		t.Run(agg, func(t *testing.T) {
			_, stderr, err := runCmd(t, "", "metrics", "query", "--name", "x", "--aggregation", agg, "--output", "json")
			if err != nil {
				t.Fatalf("aggregation %q rejected: %v\nstderr: %s", agg, err, stderr.String())
			}
		})
	}
}

func TestMetricsQuery_aggregationRejectsBogus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("server should not be called for invalid --aggregation")
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "metrics", "query", "--name", "x", "--aggregation", "nopenope", "--output", "json")
	if err == nil {
		t.Fatal("expected error for bogus --aggregation")
	}
	if !strings.Contains(stderr.String(), `"usage_error"`) {
		t.Errorf("expected usage_error envelope, got: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "--aggregation") {
		t.Errorf("error should name the flag, got: %s", stderr.String())
	}
	for _, want := range []string{"avg", "sum", "count", "min", "max", "p50", "p95", "p99"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("error should list allowed value %q, got: %s", want, stderr.String())
		}
	}
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != exitcode.Usage {
		t.Errorf("expected cliError(Usage), got %v", err)
	}
}

func TestMetricsList_table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/metrics/discover" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q", r.Method)
		}
		// No time flags -> no from/to params, server defaults to 7d
		if r.URL.Query().Get("from") != "" || r.URL.Query().Get("to") != "" {
			t.Errorf("expected no from/to params, got: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{
			"metrics":[
				{"name":"system.cpu.utilization","tagKeys":["host","core"],"metricType":"gauge","unit":"%"},
				{"name":"orders.created","tagKeys":[]}
			]
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "metrics", "list", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "TAG KEYS") {
		t.Errorf("table missing headers: %s", out)
	}
	if !strings.Contains(out, "system.cpu.utilization") || !strings.Contains(out, "host,core") {
		t.Errorf("table missing row data: %s", out)
	}
}

func TestMetricsList_sincePassesWindow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("from") == "" || r.URL.Query().Get("to") == "" {
			t.Errorf("expected from/to params with --since, got: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"metrics":[]}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	if _, _, err := runCmd(t, "", "metrics", "list", "--since", "24h"); err != nil {
		t.Fatal(err)
	}
}

func TestMetricsList_searchFiltersClientSide(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"metrics":[
				{"name":"system.cpu.utilization","tagKeys":[]},
				{"name":"orders.created","tagKeys":[]}
			]
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "metrics", "list", "--search", "cpu", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "system.cpu.utilization") {
		t.Errorf("expected matching metric: %s", out)
	}
	if strings.Contains(out, "orders.created") {
		t.Errorf("expected non-matching metric filtered out: %s", out)
	}
}

func TestMetricsTags_keysForm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/metrics/discover" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"metrics":[{"name":"system.cpu.utilization","tagKeys":["host","core"]}]}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "metrics", "tags", "system.cpu.utilization", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "host") || !strings.Contains(out, "core") {
		t.Errorf("expected tag keys listed: %s", out)
	}
}

func TestMetricsTags_valuesForm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/metrics/discover/tags" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("name") != "system.cpu.utilization" || r.URL.Query().Get("key") != "host" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"values":["web-1","web-2"]}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "metrics", "tags", "system.cpu.utilization", "host", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "web-1") || !strings.Contains(out, "web-2") {
		t.Errorf("expected tag values listed: %s", out)
	}
}

func TestMetricsTags_unknownMetricExitsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"metrics":[]}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "metrics", "tags", "nope", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	var cliErr *cliError
	if !errors.As(err, &cliErr) || cliErr.code != exitcode.NotFound {
		t.Errorf("expected exit %d, got: %v", exitcode.NotFound, err)
	}
	if !strings.Contains(stderr.String(), `"not_found"`) {
		t.Errorf("expected not_found envelope, got: %s", stderr.String())
	}
}
