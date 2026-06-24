package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEndpointsShow_postsRecordedAtAndRendersJSON(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"endpoint":{"id":"00000000-0000-0000-0000-000000000002","endpoint":"GET /x","recordedAt":"2026-06-23T14:30:00Z"},"spans":[],"hasSpans":false,"messages":[]}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "endpoints", "show",
		"00000000-0000-0000-0000-000000000002",
		"--recorded-at", "2026-06-23T14:30:00Z", "--output", "json")
	if err != nil {
		t.Fatalf("endpoints show: %v", err)
	}
	if gotPath != "/api/endpoints/00000000-0000-0000-0000-000000000002" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["recordedAt"] != "2026-06-23T14:30:00Z" {
		t.Errorf("recordedAt body = %v", gotBody["recordedAt"])
	}
	if !strings.Contains(stdout.String(), `"GET /x"`) {
		t.Errorf("expected endpoint in output: %s", stdout.String())
	}
}

func TestEndpointsShow_missingRecordedAt_writesUsageEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("server should not be called when --recorded-at is missing")
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "endpoints", "show",
		"00000000-0000-0000-0000-000000000002", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"usage_error"`) {
		t.Errorf("expected usage_error envelope, got: %s", stderr.String())
	}
}

func TestEndpointsShow_badTimestamp_writesInvalidTimestampEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("server should not be called when --recorded-at is malformed")
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "endpoints", "show",
		"00000000-0000-0000-0000-000000000002", "--recorded-at", "yesterday", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"invalid_timestamp"`) {
		t.Errorf("expected invalid_timestamp envelope, got: %s", stderr.String())
	}
}

func TestEndpointsShow_badUUID_writesUsageEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("server should not be called for a bad UUID")
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "endpoints", "show", "not-a-uuid",
		"--recorded-at", "2026-06-23T14:30:00Z", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"usage_error"`) {
		t.Errorf("expected usage_error envelope, got: %s", stderr.String())
	}
}

func TestExceptionsOccurrence_postsRecordedAt(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"exception":{"id":"00000000-0000-0000-0000-000000000001","exceptionHash":"abc","stackTrace":"boom","recordedAt":"2026-06-23T14:30:00Z"},"sessionId":"00000000-0000-0000-0000-0000000000aa"}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "exceptions", "occurrence",
		"00000000-0000-0000-0000-000000000001",
		"--recorded-at", "2026-06-23T14:30:00Z", "--output", "json")
	if err != nil {
		t.Fatalf("exceptions occurrence: %v", err)
	}
	if gotPath != "/api/exception-stack-traces/by-id/00000000-0000-0000-0000-000000000001" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["recordedAt"] != "2026-06-23T14:30:00Z" {
		t.Errorf("recordedAt body = %v", gotBody["recordedAt"])
	}
	if !strings.Contains(stdout.String(), `"abc"`) {
		t.Errorf("expected hash in output: %s", stdout.String())
	}
}

func TestTracesShow_rendersNodesAndNoProjectId(t *testing.T) {
	var gotRawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"distributedTraceId":"00000000-0000-0000-0000-000000000006","nodes":[{"projectName":"api","traceType":"endpoint","endpoint":{"id":"00000000-0000-0000-0000-0000000000d1","endpoint":"GET /x"},"spans":[]}]}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "traces", "show",
		"00000000-0000-0000-0000-000000000006",
		"--recorded-at", "2026-06-23T14:30:00Z", "--output", "table")
	if err != nil {
		t.Fatalf("traces show: %v", err)
	}
	if gotRawQuery != "" {
		t.Errorf("traces show must not send a query string, got %q", gotRawQuery)
	}
	out := stdout.String()
	if !strings.Contains(out, "GET /x") || !strings.Contains(out, "endpoint") {
		t.Errorf("expected node row in table: %s", out)
	}
}

func TestSessionsShow_postsStartedAt(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"session":{"id":"00000000-0000-0000-0000-000000000005","startedAt":"2026-06-23T14:30:00Z"},"exceptions":[]}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, _, err := runCmd(t, "", "sessions", "show",
		"00000000-0000-0000-0000-000000000005",
		"--started-at", "2026-06-23T14:30:00Z", "--output", "json")
	if err != nil {
		t.Fatalf("sessions show: %v", err)
	}
	if gotBody["startedAt"] != "2026-06-23T14:30:00Z" {
		t.Errorf("startedAt body = %v", gotBody["startedAt"])
	}
}

func TestTasksShow_postsRecordedAtAndRendersTable(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"task":{"id":"00000000-0000-0000-0000-000000000003","taskName":"nightly","recordedAt":"2026-06-23T14:30:00Z","duration":1500000000},"spans":[{"id":"00000000-0000-0000-0000-0000000000b1","name":"db.query","startTime":"2026-06-23T14:30:00Z","duration":50000000}],"hasSpans":true,"exception":{"exceptionHash":"deadbeef","stackTrace":"boom\nat foo"},"messages":[]}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "tasks", "show",
		"00000000-0000-0000-0000-000000000003",
		"--recorded-at", "2026-06-23T14:30:00Z", "--output", "table")
	if err != nil {
		t.Fatalf("tasks show: %v", err)
	}
	if gotPath != "/api/tasks/00000000-0000-0000-0000-000000000003" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["recordedAt"] != "2026-06-23T14:30:00Z" {
		t.Errorf("recordedAt body = %v", gotBody["recordedAt"])
	}
	out := stdout.String()
	for _, want := range []string{"nightly", "db.query", "deadbeef"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in table output: %s", want, out)
		}
	}
}

func TestTasksShow_missingRecordedAt_writesUsageEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("server should not be called when --recorded-at is missing")
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "tasks", "show",
		"00000000-0000-0000-0000-000000000003", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"usage_error"`) {
		t.Errorf("expected usage_error envelope, got: %s", stderr.String())
	}
}

func TestAiTracesShow_postsRecordedAtAndRendersTable(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"aiTrace":{"id":"00000000-0000-0000-0000-000000000004","traceName":"chat","model":"claude","provider":"anthropic","operation":"messages","inputTokens":100,"outputTokens":50,"totalTokens":150,"totalCost":0.0123,"recordedAt":"2026-06-23T14:30:00Z","duration":2000000000},"conversation":[{"role":"user"}]}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "ai-traces", "show",
		"00000000-0000-0000-0000-000000000004",
		"--recorded-at", "2026-06-23T14:30:00Z", "--output", "table")
	if err != nil {
		t.Fatalf("ai-traces show: %v", err)
	}
	if gotPath != "/api/ai-traces/00000000-0000-0000-0000-000000000004" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["recordedAt"] != "2026-06-23T14:30:00Z" {
		t.Errorf("recordedAt body = %v", gotBody["recordedAt"])
	}
	out := stdout.String()
	for _, want := range []string{"claude", "100 in / 50 out / 150 total", "CONVERSATION available"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in table output: %s", want, out)
		}
	}
}

func TestAiTracesShow_missingRecordedAt_writesUsageEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("server should not be called when --recorded-at is missing")
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "ai-traces", "show",
		"00000000-0000-0000-0000-000000000004", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"usage_error"`) {
		t.Errorf("expected usage_error envelope, got: %s", stderr.String())
	}
}
