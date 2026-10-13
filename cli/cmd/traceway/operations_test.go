package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPagesList_sendsStatusAndPassesThroughJSON(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/pages" || r.URL.Query().Get("projectId") != "proj-1" {
			t.Errorf("request = %s %s", r.Method, r.URL)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"data":[{"id":7,"status":"open","subject":"DB down"}],"pagination":{"page":1,"pageSize":50,"total":1,"totalPages":1}}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, stderr, err := runCmd(t, "", "pages", "list", "--output", "json")
	if err != nil {
		t.Fatalf("runCmd: %v\nstderr: %s", err, stderr.String())
	}
	if body["status"] != "active" {
		t.Errorf("default status should be active, body = %v", body)
	}
	if !strings.Contains(stdout.String(), `"subject":"DB down"`) {
		t.Errorf("stdout = %s", stdout.String())
	}
}

func TestPagesList_rejectsUnknownStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("an invalid status must not reach the server")
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "pages", "list", "--status", "closed", "--output", "json")
	if err == nil || !strings.Contains(stderr.String(), "usage_error") {
		t.Fatalf("want a usage error, got err=%v stderr=%s", err, stderr.String())
	}
}

func TestAlertsList_tableUsesRulesWrapper(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/notification-rules" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"rules":[{"id":3,"name":"Slow search","ruleType":"endpoint_p95_threshold","enabled":true,"snoozedUntil":null}]}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, stderr, err := runCmd(t, "", "alerts", "list", "--output", "table")
	if err != nil {
		t.Fatalf("runCmd: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "Slow search") || !strings.Contains(out, "endpoint_p95_threshold") {
		t.Errorf("table = %s", out)
	}
}

func TestPostMortemsList_forwardsSearchAndTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/api/post-mortems" || q.Get("search") != "redis" || strings.Join(q["tag"], ",") != "cache,db" {
			t.Errorf("request = %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"data":[],"pagination":{"page":1,"pageSize":50,"total":0,"totalPages":0}}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	if _, stderr, err := runCmd(t, "", "post-mortems", "list", "--search", "redis", "--tag", "cache", "--tag", "db", "--output", "json"); err != nil {
		t.Fatalf("runCmd: %v\nstderr: %s", err, stderr.String())
	}
}

func TestMonitorsShow_rejectsNonNumericId(t *testing.T) {
	seedSessionFor(t, "http://127.0.0.1:1")
	_, stderr, err := runCmd(t, "", "monitors", "show", "abc", "--output", "json")
	if err == nil || !strings.Contains(stderr.String(), "positive integer") {
		t.Fatalf("want a usage error, got err=%v stderr=%s", err, stderr.String())
	}
}

func TestForbiddenCarriesServerMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"This endpoint is not available to personal access tokens, the CLI, or MCP clients."}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "alerts", "list", "--output", "json")
	if err == nil || !strings.Contains(stderr.String(), "not available to personal access tokens") {
		t.Fatalf("403 should surface the server's message, stderr = %s", stderr.String())
	}
}
