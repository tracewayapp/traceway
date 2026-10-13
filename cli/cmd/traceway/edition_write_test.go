//go:build write

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/appdir"
)

func TestWriteEdition_identity(t *testing.T) {
	if appdir.Name != "traceway-write" || appdir.Binary != "traceway-write" || newRootCmd().Use != "traceway-write" {
		t.Errorf("write build identity = dir %q, binary %q, use %q", appdir.Name, appdir.Binary, newRootCmd().Use)
	}
	if deviceClientID != "traceway-cli-write" {
		t.Errorf("device client = %q", deviceClientID)
	}
}

func TestWriteEdition_passwordLoginAsksForWriteScope(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	}))
	defer srv.Close()

	if _, stderr, err := runCmd(t, "pw\n", "login", "--url", srv.URL, "--username", "a@example.com", "--password-stdin"); err != nil {
		t.Fatalf("login: %v %s", err, stderr.String())
	}
	if body["scope"] != "traceway:write" {
		t.Errorf("write login must ask for a write-scoped token, body = %v", body)
	}
}

type recorded struct {
	mu    sync.Mutex
	calls []string
	body  map[string]any
}

func (r *recorded) add(req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, req.Method+" "+req.URL.Path)
	if req.Method != http.MethodGet {
		r.body = nil
		_ = json.NewDecoder(req.Body).Decode(&r.body)
	}
}

func TestPagesAck_requiresConfirmationThenAcknowledges(t *testing.T) {
	rec := &recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		_, _ = w.Write([]byte(`{"message":"Page acknowledged"}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	t.Cleanup(func() { flagYes = false })

	if _, _, err := runCmd(t, "", "pages", "ack", "7", "--output", "json"); err == nil || len(rec.calls) != 0 {
		t.Fatalf("a non-TTY write without --yes must be refused before any request, calls = %v", rec.calls)
	}
	stdout, stderr, err := runCmd(t, "", "pages", "ack", "7", "--yes", "--output", "json")
	if err != nil {
		t.Fatalf("ack: %v %s", err, stderr.String())
	}
	if len(rec.calls) != 1 || rec.calls[0] != "POST /api/pages/7/acknowledge" {
		t.Errorf("calls = %v", rec.calls)
	}
	if !strings.Contains(stdout.String(), "acknowledge") {
		t.Errorf("stdout = %s", stdout.String())
	}
}

func TestPagesResolve_archiveIssueFlag(t *testing.T) {
	rec := &recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	t.Cleanup(func() { flagYes = false })

	if _, stderr, err := runCmd(t, "", "pages", "resolve", "9", "--archive-issue", "--yes", "--output", "json"); err != nil {
		t.Fatalf("resolve: %v %s", err, stderr.String())
	}
	if rec.calls[0] != "POST /api/pages/9/resolve" || rec.body["archiveIssue"] != true {
		t.Errorf("calls = %v body = %v", rec.calls, rec.body)
	}
}

func TestAlertsEnableDisable_isIdempotent(t *testing.T) {
	rec := &recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"rules":[{"id":4,"name":"P95","ruleType":"endpoint_p95_threshold","enabled":true}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":4,"enabled":false}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	t.Cleanup(func() { flagYes = false })

	if _, stderr, err := runCmd(t, "", "alerts", "enable", "4", "--yes", "--output", "json"); err != nil {
		t.Fatalf("enable: %v %s", err, stderr.String())
	}
	if len(rec.calls) != 1 || rec.calls[0] != "GET /api/notification-rules" {
		t.Errorf("enabling an enabled rule must not toggle it, calls = %v", rec.calls)
	}

	rec.calls = nil
	if _, stderr, err := runCmd(t, "", "alerts", "disable", "4", "--yes", "--output", "json"); err != nil {
		t.Fatalf("disable: %v %s", err, stderr.String())
	}
	if len(rec.calls) != 2 || rec.calls[1] != "POST /api/notification-rules/4/toggle" {
		t.Errorf("disable calls = %v", rec.calls)
	}

	rec.calls = nil
	if _, _, err := runCmd(t, "", "alerts", "disable", "99", "--yes", "--output", "json"); err == nil {
		t.Error("an unknown rule id should fail")
	}
}

func TestAlertsSnooze_sendsMinutes(t *testing.T) {
	rec := &recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		_, _ = w.Write([]byte(`{"id":4}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	t.Cleanup(func() { flagYes = false })

	if _, stderr, err := runCmd(t, "", "alerts", "snooze", "4", "--minutes", "30", "--yes", "--output", "json"); err != nil {
		t.Fatalf("snooze: %v %s", err, stderr.String())
	}
	if rec.calls[0] != "POST /api/notification-rules/4/snooze" || rec.body["durationMinutes"] != float64(30) {
		t.Errorf("calls = %v body = %v", rec.calls, rec.body)
	}
}

func TestPostMortemsUpdate_keepsFieldsItWasNotGiven(t *testing.T) {
	rec := &recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":5,"title":"Redis outage","contentMd":"old","tags":["redis"],"incidentId":11}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":5}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	t.Cleanup(func() { flagYes = false })

	if _, stderr, err := runCmd(t, "new body", "post-mortems", "update", "5", "--content-file", "-", "--yes", "--output", "json"); err != nil {
		t.Fatalf("update: %v %s", err, stderr.String())
	}
	if rec.calls[1] != "PUT /api/post-mortems/5" {
		t.Fatalf("calls = %v", rec.calls)
	}
	if rec.body["title"] != "Redis outage" || rec.body["contentMd"] != "new body" || rec.body["incidentId"] != float64(11) {
		t.Errorf("update body = %v", rec.body)
	}
	tags, _ := rec.body["tags"].([]any)
	if len(tags) != 1 || tags[0] != "redis" {
		t.Errorf("tags = %v", rec.body["tags"])
	}
}

func TestPostMortemsUpdate_nothingToUpdate(t *testing.T) {
	seedSessionFor(t, "http://127.0.0.1:1")
	_, stderr, err := runCmd(t, "", "post-mortems", "update", "5", "--yes", "--output", "json")
	if err == nil || !strings.Contains(stderr.String(), "nothing to update") {
		t.Fatalf("want nothing to update, got %v %s", err, stderr.String())
	}
}

func TestEndpointsMarkSlow(t *testing.T) {
	rec := &recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	t.Cleanup(func() { flagYes = false })

	if _, stderr, err := runCmd(t, "", "endpoints", "mark-slow", "GET /export", "--offset-ms", "2000", "--reason", "batch", "--yes", "--output", "json"); err != nil {
		t.Fatalf("mark-slow: %v %s", err, stderr.String())
	}
	if rec.calls[0] != "POST /api/endpoints/slow" || rec.body["endpoint"] != "GET /export" || rec.body["offsetMs"] != float64(2000) {
		t.Errorf("calls = %v body = %v", rec.calls, rec.body)
	}
}
