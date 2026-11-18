package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestQueryLogs_basic(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/logs" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("projectId") != "proj-1" {
			t.Errorf("projectId = %q", r.URL.Query().Get("projectId"))
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["fromDate"] == nil || body["toDate"] == nil {
			t.Errorf("body missing fromDate/toDate: %v", body)
		}
		_, _ = w.Write([]byte(`{
			"data":[
				{"id":"00000000-0000-0000-0000-000000000001","timestamp":"2026-05-13T12:00:00Z","severityText":"ERROR","severityNumber":17,"serviceName":"api","body":"boom"}
			],
			"pagination":{"page":1,"pageSize":50,"total":1,"totalPages":1}
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	resp, err := c.QueryLogs(context.Background(), "proj-1", QueryLogsRequest{
		TimeRange:   TimeRange{From: time.Now().Add(-time.Hour), To: time.Now()},
		Pagination:  PaginationParams{Page: 1, PageSize: 50},
		ServiceName: "api",
	})
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("got %d logs", len(resp.Data))
	}
	if resp.Data[0].ServiceName != "api" {
		t.Errorf("ServiceName = %q", resp.Data[0].ServiceName)
	}
	if resp.Data[0].Body != "boom" {
		t.Errorf("Body = %q", resp.Data[0].Body)
	}
}

func TestQueryLogs_passesFilters(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["serviceName"] != "api" {
			t.Errorf("serviceName = %v", body["serviceName"])
		}
		if int(body["minSeverity"].(float64)) != 13 { // WARN per OTel severity numbers
			t.Errorf("minSeverity = %v", body["minSeverity"])
		}
		if body["traceId"] != "abc123" {
			t.Errorf("traceId = %v", body["traceId"])
		}
		_, _ = w.Write([]byte(`{"data":[],"pagination":{}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	_, err := c.QueryLogs(context.Background(), "proj-1", QueryLogsRequest{
		ServiceName: "api",
		MinSeverity: 13,
		TraceId:     "abc123",
	})
	if err != nil {
		t.Fatal(err)
	}
}
