package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestLogsQuery_basic(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
