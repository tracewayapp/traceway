package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestQueryMetrics_singleQuery(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/metrics/query" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("projectId") != "proj-1" {
			t.Errorf("projectId = %q", r.URL.Query().Get("projectId"))
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		// metrics uses 'from'/'to', not fromDate/toDate
		if body["from"] == nil || body["to"] == nil {
			t.Errorf("body missing from/to: %v", body)
		}
		queries, _ := body["queries"].([]any)
		if len(queries) != 1 {
			t.Fatalf("expected 1 query, got %d", len(queries))
		}
		q := queries[0].(map[string]any)
		if q["name"] != "http.request.duration" {
			t.Errorf("query name = %v", q["name"])
		}
		if q["aggregation"] != "p95" {
			t.Errorf("aggregation = %v", q["aggregation"])
		}
		_, _ = w.Write([]byte(`{
			"results":[
				{
					"name":"http.request.duration",
					"unit":"ms",
					"series":{
						"all":[
							{"timestamp":"2026-05-13T12:00:00Z","value":42.5},
							{"timestamp":"2026-05-13T12:05:00Z","value":47.0}
						]
					}
				}
			]
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	resp, err := c.QueryMetrics(context.Background(), "proj-1", QueryMetricsRequest{
		TimeRange:       TimeRange{From: time.Now().Add(-time.Hour), To: time.Now()},
		IntervalMinutes: 5,
		Queries: []MetricQueryItem{
			{Name: "http.request.duration", Aggregation: "p95"},
		},
	})
	if err != nil {
		t.Fatalf("QueryMetrics: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("got %d results", len(resp.Results))
	}
	r := resp.Results[0]
	if r.Name != "http.request.duration" {
		t.Errorf("Name = %q", r.Name)
	}
	if r.Unit != "ms" {
		t.Errorf("Unit = %q", r.Unit)
	}
	series, ok := r.Series["all"]
	if !ok {
		t.Fatal(`series["all"] missing`)
	}
	if len(series) != 2 {
		t.Errorf("got %d points, want 2", len(series))
	}
	if series[0].Value != 42.5 {
		t.Errorf("series[0].Value = %v", series[0].Value)
	}
}

func TestQueryMetrics_multiQuery(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		queries, _ := body["queries"].([]any)
		if len(queries) != 2 {
			t.Errorf("expected 2 queries, got %d", len(queries))
		}
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	_, err := c.QueryMetrics(context.Background(), "proj-1", QueryMetricsRequest{
		Queries: []MetricQueryItem{
			{Name: "metric.a", Aggregation: "avg"},
			{Name: "metric.b", Aggregation: "sum"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}
