package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/v2/internal/exitcode"
)

func TestMetricsQuery_jsonOutput(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	for _, agg := range []string{"avg", "sum", "count", "min", "max", "rate", "last", "p50", "p75", "p90", "p95", "p99", "good_ratio"} {
		t.Run(agg, func(t *testing.T) {
			received := make(chan string, 1)
			srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Queries []struct {
						Aggregation string `json:"aggregation"`
					} `json:"queries"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Queries) != 1 {
					http.Error(w, "expected one metric query", http.StatusBadRequest)
					return
				}
				received <- req.Queries[0].Aggregation
				_, _ = w.Write([]byte(`{"results":[]}`))
			}))
			defer srv.Close()
			seedSessionFor(t, srv.URL)

			name := "browser.web_vital.lcp"
			if agg == "last" {
				name = "system.cpu.utilization"
			}
			_, stderr, err := runCmd(t, "", "metrics", "query", "--name", name, "--aggregation", agg, "--output", "json")
			if err != nil {
				t.Fatalf("aggregation %q rejected: %v\nstderr: %s", agg, err, stderr.String())
			}
			select {
			case got := <-received:
				if got != agg {
					t.Errorf("sent aggregation %q, want %q", got, agg)
				}
			default:
				t.Fatal("server did not receive a metric query")
			}
		})
	}
}

func TestMetricsQuery_aggregationRejectsBogus(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
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

// "INP p75 over 24 hours" is one value for the whole window, so reading what an
// alert saw takes --whole-range; good_ratio carries its threshold.
func TestMetricsQuery_sendsWholeRangeAndThreshold(t *testing.T) {
	var body map[string]any
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	if _, stderr, err := runCmd(t, "", "metrics", "query", "--name", "browser.web_vital.inp", "--aggregation", "good_ratio",
		"--threshold", "200", "--whole-range", "--since", "24h", "--output", "json"); err != nil {
		t.Fatalf("metrics query: %v\n%s", err, stderr.String())
	}
	if body["wholeRange"] != true {
		t.Errorf("wholeRange not sent: %v", body)
	}
	queries, _ := body["queries"].([]any)
	query, _ := queries[0].(map[string]any)
	if query["aggregation"] != "good_ratio" || query["threshold"] != float64(200) {
		t.Errorf("query sent as %v", query)
	}
}

func TestMetricsQuery_thresholdNeedsGoodRatio(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("server should not be called for a stray --threshold")
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "metrics", "query", "--name", "browser.web_vital.inp", "--aggregation", "p75", "--threshold", "200", "--output", "json")
	if err == nil || !strings.Contains(stderr.String(), "good_ratio") {
		t.Fatalf("expected a usage error naming good_ratio, got err=%v stderr=%s", err, stderr.String())
	}
}
