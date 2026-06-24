package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ts is a fixed RFC3339 timestamp used across the detail tests.
var ts = time.Date(2026, 6, 23, 14, 30, 0, 0, time.UTC)

func TestGetExceptionById_postsRecordedAtAndProjectId(t *testing.T) {
	var gotPath, gotProject string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		gotPath = r.URL.Path
		gotProject = r.URL.Query().Get("projectId")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"exception":{"id":"00000000-0000-0000-0000-000000000001","exceptionHash":"h","stackTrace":"x","recordedAt":"2026-06-23T14:30:00Z"},"sessionId":"00000000-0000-0000-0000-0000000000aa"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	resp, err := c.GetExceptionById(context.Background(), "proj-1", "00000000-0000-0000-0000-000000000001", ts)
	if err != nil {
		t.Fatalf("GetExceptionById: %v", err)
	}
	if gotPath != "/api/exception-stack-traces/by-id/00000000-0000-0000-0000-000000000001" {
		t.Errorf("path = %q", gotPath)
	}
	if gotProject != "proj-1" {
		t.Errorf("projectId = %q", gotProject)
	}
	if gotBody["recordedAt"] != "2026-06-23T14:30:00Z" {
		t.Errorf("recordedAt = %v", gotBody["recordedAt"])
	}
	if resp.Exception == nil || resp.SessionId == nil {
		t.Errorf("expected exception + sessionId, got %+v", resp)
	}
}

func TestGetEndpoint_postsRecordedAtAndProjectId(t *testing.T) {
	var gotPath, gotProject string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotProject = r.URL.Query().Get("projectId")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"endpoint":{"id":"00000000-0000-0000-0000-000000000002","endpoint":"GET /x","recordedAt":"2026-06-23T14:30:00Z"},"spans":[{"id":"00000000-0000-0000-0000-0000000000b1","name":"db"}],"hasSpans":true,"messages":[]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	resp, err := c.GetEndpoint(context.Background(), "proj-1", "00000000-0000-0000-0000-000000000002", ts)
	if err != nil {
		t.Fatalf("GetEndpoint: %v", err)
	}
	if gotPath != "/api/endpoints/00000000-0000-0000-0000-000000000002" {
		t.Errorf("path = %q", gotPath)
	}
	if gotProject != "proj-1" {
		t.Errorf("projectId = %q", gotProject)
	}
	if gotBody["recordedAt"] != "2026-06-23T14:30:00Z" {
		t.Errorf("recordedAt = %v", gotBody["recordedAt"])
	}
	if resp.Endpoint == nil || resp.Endpoint.Endpoint != "GET /x" {
		t.Errorf("endpoint missing/wrong: %+v", resp.Endpoint)
	}
	if len(resp.Spans) != 1 || !resp.HasSpans {
		t.Errorf("spans = %+v hasSpans=%v", resp.Spans, resp.HasSpans)
	}
}

func TestGetTask_postsRecordedAt(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"task":{"id":"00000000-0000-0000-0000-000000000003","taskName":"nightly","recordedAt":"2026-06-23T14:30:00Z"},"spans":[],"hasSpans":false,"messages":[]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	resp, err := c.GetTask(context.Background(), "proj-1", "00000000-0000-0000-0000-000000000003", ts)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if gotPath != "/api/tasks/00000000-0000-0000-0000-000000000003" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["recordedAt"] != "2026-06-23T14:30:00Z" {
		t.Errorf("recordedAt = %v", gotBody["recordedAt"])
	}
	if resp.Task == nil || resp.Task.TaskName != "nightly" {
		t.Errorf("task missing/wrong: %+v", resp.Task)
	}
}

func TestGetAiTrace_postsRecordedAt(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"aiTrace":{"id":"00000000-0000-0000-0000-000000000004","traceName":"chat","model":"claude","recordedAt":"2026-06-23T14:30:00Z"},"conversation":[{"role":"user"}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	resp, err := c.GetAiTrace(context.Background(), "proj-1", "00000000-0000-0000-0000-000000000004", ts)
	if err != nil {
		t.Fatalf("GetAiTrace: %v", err)
	}
	if gotPath != "/api/ai-traces/00000000-0000-0000-0000-000000000004" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["recordedAt"] != "2026-06-23T14:30:00Z" {
		t.Errorf("recordedAt = %v", gotBody["recordedAt"])
	}
	if resp.AiTrace == nil || resp.AiTrace.Model != "claude" {
		t.Errorf("aiTrace missing/wrong: %+v", resp.AiTrace)
	}
	if len(resp.Conversation) == 0 {
		t.Errorf("expected conversation passthrough")
	}
}

func TestGetSession_postsStartedAt(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"session":{"id":"00000000-0000-0000-0000-000000000005","startedAt":"2026-06-23T14:30:00Z"},"exceptions":[{"id":"00000000-0000-0000-0000-0000000000c1","exceptionHash":"h","stackTrace":"x","recordedAt":"2026-06-23T14:31:00Z","isMessage":false}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	resp, err := c.GetSession(context.Background(), "proj-1", "00000000-0000-0000-0000-000000000005", ts)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if gotPath != "/api/sessions/00000000-0000-0000-0000-000000000005" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["startedAt"] != "2026-06-23T14:30:00Z" {
		t.Errorf("startedAt = %v (want startedAt, not recordedAt)", gotBody["startedAt"])
	}
	if gotBody["recordedAt"] != nil {
		t.Errorf("session body should not carry recordedAt: %v", gotBody)
	}
	if resp.Session == nil || len(resp.Exceptions) != 1 {
		t.Errorf("session/exceptions wrong: %+v", resp)
	}
}

func TestGetDistributedTrace_noProjectIdQueryParam(t *testing.T) {
	var gotPath, gotRawQuery string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRawQuery = r.URL.RawQuery
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"distributedTraceId":"00000000-0000-0000-0000-000000000006","nodes":[{"projectName":"api","traceType":"endpoint","endpoint":{"id":"00000000-0000-0000-0000-0000000000d1","endpoint":"GET /x"},"spans":[]}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	resp, err := c.GetDistributedTrace(context.Background(), "00000000-0000-0000-0000-000000000006", ts)
	if err != nil {
		t.Fatalf("GetDistributedTrace: %v", err)
	}
	if gotPath != "/api/distributed-traces/00000000-0000-0000-0000-000000000006" {
		t.Errorf("path = %q", gotPath)
	}
	if gotRawQuery != "" {
		t.Errorf("distributed-traces must not send a query string, got %q", gotRawQuery)
	}
	if gotBody["recordedAt"] != "2026-06-23T14:30:00Z" {
		t.Errorf("recordedAt = %v", gotBody["recordedAt"])
	}
	if len(resp.Nodes) != 1 || resp.Nodes[0].TraceType != "endpoint" {
		t.Errorf("nodes wrong: %+v", resp.Nodes)
	}
}
