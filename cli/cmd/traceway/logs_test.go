package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogsQuery_basic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/logs" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"data":[
				{"id":"00000000-0000-0000-0000-000000000001","timestamp":"2026-05-13T12:00:00Z","severityText":"ERROR","severityNumber":17,"serviceName":"api","body":"failed to connect"}
			],
			"pagination":{"total":1}
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "logs", "query", "--output", "json")
	if err != nil {
		t.Fatalf("logs query: %v", err)
	}
	if !strings.Contains(stdout.String(), "failed to connect") {
		t.Errorf("expected log body in output: %s", stdout.String())
	}
}

func TestLogsQuery_table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"data":[
				{"id":"00000000-0000-0000-0000-000000000001","timestamp":"2026-05-13T12:00:00Z","severityText":"ERROR","severityNumber":17,"serviceName":"api","body":"failed to connect"}
			],
			"pagination":{"total":1}
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "logs", "query", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "TIMESTAMP") || !strings.Contains(out, "SEVERITY") || !strings.Contains(out, "SERVICE") {
		t.Errorf("table missing headers: %s", out)
	}
	if !strings.Contains(out, "ERROR") || !strings.Contains(out, "api") || !strings.Contains(out, "failed") {
		t.Errorf("table missing row data: %s", out)
	}
}

func TestLogsQuery_passesServiceFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decode body and assert serviceName was passed through
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		body := string(buf[:n])
		if !strings.Contains(body, `"serviceName":"api"`) {
			t.Errorf("expected serviceName=api in body, got: %s", body)
		}
		_, _ = w.Write([]byte(`{"data":[],"pagination":{}}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	if _, _, err := runCmd(t, "", "logs", "query", "--service", "api"); err != nil {
		t.Fatal(err)
	}
}

func TestLogsQuery_severityNameMapsToNumber(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		body := string(buf[:n])
		if !strings.Contains(body, `"minSeverity":17`) {
			t.Errorf("expected minSeverity=17 in body, got: %s", body)
		}
		_, _ = w.Write([]byte(`{"data":[],"pagination":{}}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	if _, _, err := runCmd(t, "", "logs", "query", "--min-severity", "error"); err != nil {
		t.Fatal(err)
	}
}

func TestLogsQuery_severityNumberStillWorks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		body := string(buf[:n])
		if !strings.Contains(body, `"minSeverity":13`) {
			t.Errorf("expected minSeverity=13 in body, got: %s", body)
		}
		_, _ = w.Write([]byte(`{"data":[],"pagination":{}}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	if _, _, err := runCmd(t, "", "logs", "query", "--min-severity", "13"); err != nil {
		t.Fatal(err)
	}
}

func TestLogsQuery_severityBogusIsUsageError(t *testing.T) {
	seedSessionFor(t, "http://127.0.0.1:0")

	_, stderr, err := runCmd(t, "", "logs", "query", "--min-severity", "severe", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"usage_error"`) {
		t.Errorf("expected usage_error envelope, got: %s", stderr.String())
	}
}

func TestLogsQuery_attrFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 8192)
		n, _ := r.Body.Read(buf)
		body := string(buf[:n])
		// default scope is log; explicit resource: prefix is honored
		if !strings.Contains(body, `{"scope":"log","key":"user.id","value":"42"}`) {
			t.Errorf("expected log-scoped filter in body, got: %s", body)
		}
		if !strings.Contains(body, `{"scope":"resource","key":"host.name","value":"web-1"}`) {
			t.Errorf("expected resource-scoped filter in body, got: %s", body)
		}
		_, _ = w.Write([]byte(`{"data":[],"pagination":{}}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	if _, _, err := runCmd(t, "", "logs", "query",
		"--attr", "user.id=42",
		"--attr", "resource:host.name=web-1"); err != nil {
		t.Fatal(err)
	}
}

func TestLogsQuery_attrBadScopeIsUsageError(t *testing.T) {
	seedSessionFor(t, "http://127.0.0.1:0")

	_, stderr, err := runCmd(t, "", "logs", "query", "--attr", "span:key=v", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"usage_error"`) {
		t.Errorf("expected usage_error envelope, got: %s", stderr.String())
	}
}

func TestLogsQuery_distributedTraceId(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		body := string(buf[:n])
		if !strings.Contains(body, `"distributedTraceId":"11111111-1111-1111-1111-111111111111"`) {
			t.Errorf("expected distributedTraceId in body, got: %s", body)
		}
		if !strings.Contains(body, `"excludeTraceId":"abc123"`) {
			t.Errorf("expected excludeTraceId in body, got: %s", body)
		}
		_, _ = w.Write([]byte(`{"data":[],"pagination":{}}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	if _, _, err := runCmd(t, "", "logs", "query",
		"--distributed-trace-id", "11111111-1111-1111-1111-111111111111",
		"--exclude-trace-id", "abc123"); err != nil {
		t.Fatal(err)
	}
}

func TestLogsQuery_excludeWithoutDistributedIsUsageError(t *testing.T) {
	seedSessionFor(t, "http://127.0.0.1:0")

	_, stderr, err := runCmd(t, "", "logs", "query", "--exclude-trace-id", "abc123", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"usage_error"`) {
		t.Errorf("expected usage_error envelope, got: %s", stderr.String())
	}
}

func TestLogsQuery_invalidDistributedTraceIdIsUsageError(t *testing.T) {
	seedSessionFor(t, "http://127.0.0.1:0")

	_, stderr, err := runCmd(t, "", "logs", "query", "--distributed-trace-id", "not-a-uuid", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"usage_error"`) {
		t.Errorf("expected usage_error envelope, got: %s", stderr.String())
	}
}
