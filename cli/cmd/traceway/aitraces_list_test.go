package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAiTracesList_table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ai-traces/grouped" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"data":[
				{"traceName":"summarize-ticket","count":18,"p50Duration":900000000,"p95Duration":2500000000,"avgDuration":1100000000,"totalTokens":48213,"totalCost":0.7421,"lastSeen":"2026-07-01T12:00:00Z"}
			],
			"pagination":{"total":1}
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "ai-traces", "list", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "TRACE") || !strings.Contains(out, "COST") {
		t.Errorf("table missing headers: %s", out)
	}
	if !strings.Contains(out, "summarize-ticket") || !strings.Contains(out, "48213") || !strings.Contains(out, "0.7421") {
		t.Errorf("table missing row data: %s", out)
	}
}

func TestAiTracesList_mapsOrderBy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		body := string(buf[:n])
		if !strings.Contains(body, `"orderBy":"total_cost"`) {
			t.Errorf("expected snake_case orderBy in body, got: %s", body)
		}
		_, _ = w.Write([]byte(`{"data":[],"pagination":{}}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	if _, _, err := runCmd(t, "", "ai-traces", "list", "--order-by", "totalCost"); err != nil {
		t.Fatal(err)
	}
}
