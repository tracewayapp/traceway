//go:build write

package main

import (
	"encoding/json"
	"fmt"
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

func TestAlertsEnableDisable_setsAnExplicitState(t *testing.T) {
	rec := &recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		if strings.Contains(r.URL.Path, "/99/") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"rule not found"}`))
			return
		}
		rec.mu.Lock()
		enabled, _ := rec.body["enabled"].(bool)
		rec.mu.Unlock()
		_, _ = fmt.Fprintf(w, `{"id":4,"enabled":%t}`, enabled)
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	t.Cleanup(func() { flagYes = false })

	for _, tc := range []struct {
		command string
		want    bool
	}{{"enable", true}, {"disable", false}} {
		rec.calls = nil
		if _, stderr, err := runCmd(t, "", "alerts", tc.command, "4", "--yes", "--output", "json"); err != nil {
			t.Fatalf("%s: %v %s", tc.command, err, stderr.String())
		}
		if len(rec.calls) != 1 || rec.calls[0] != "POST /api/notification-rules/4/toggle" || rec.body["enabled"] != tc.want {
			t.Errorf("%s must send one toggle with enabled=%v, calls = %v body = %v", tc.command, tc.want, rec.calls, rec.body)
		}
	}

	if _, _, err := runCmd(t, "", "alerts", "disable", "99", "--yes", "--output", "json"); err == nil {
		t.Error("an unknown rule id should fail")
	}
}

// A server older than the toggle route's enabled field ignores it and flips the
// rule. The CLI must notice from the response and undo the flip, so disabling
// an already-disabled rule leaves it disabled instead of turning it on.
func TestAlertsEnableDisable_undoesAnOlderServersFlip(t *testing.T) {
	var mu sync.Mutex
	ruleEnabled := false
	rec := &recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		mu.Lock()
		defer mu.Unlock()
		ruleEnabled = !ruleEnabled
		_, _ = fmt.Fprintf(w, `{"id":4,"enabled":%t}`, ruleEnabled)
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	t.Cleanup(func() { flagYes = false })

	for _, tc := range []struct {
		command   string
		startedOn bool
		wantCalls int
	}{
		{"disable", false, 2}, // already off: flipped on, then back off
		{"disable", true, 1},  // the flip happens to match
		{"enable", true, 2},
		{"enable", false, 1},
	} {
		mu.Lock()
		ruleEnabled = tc.startedOn
		mu.Unlock()
		rec.calls = nil
		if _, stderr, err := runCmd(t, "", "alerts", tc.command, "4", "--yes", "--output", "json"); err != nil {
			t.Fatalf("%s from enabled=%v: %v %s", tc.command, tc.startedOn, err, stderr.String())
		}
		mu.Lock()
		if want := tc.command == "enable"; ruleEnabled != want || len(rec.calls) != tc.wantCalls {
			t.Errorf("%s from enabled=%v: rule enabled=%v after %d calls, want %v after %d", tc.command, tc.startedOn, ruleEnabled, len(rec.calls), want, tc.wantCalls)
		}
		mu.Unlock()
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
			_, _ = w.Write([]byte(`{"id":5,"title":"Redis outage","contentMd":"old","tags":["redis"],"incidentId":11,"updatedAt":"2026-10-15T10:05:00Z"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":5}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	t.Cleanup(func() { flagYes = false })

	if _, stderr, err := runCmd(t, "new body", "post-mortems", "update", "5", "--content-file", "-", "--expected-updated-at", "2026-10-15T10:00:00Z", "--yes", "--output", "json"); err != nil {
		t.Fatalf("update: %v %s", err, stderr.String())
	}
	if rec.calls[1] != "PUT /api/post-mortems/5" {
		t.Fatalf("calls = %v", rec.calls)
	}
	if rec.body["title"] != "Redis outage" || rec.body["contentMd"] != "new body" || rec.body["incidentId"] != float64(11) {
		t.Errorf("update body = %v", rec.body)
	}
	if rec.body["expectedUpdatedAt"] != "2026-10-15T10:00:00Z" {
		t.Errorf("the revision the edit is based on must be sent, not the one re-read before the PUT: expectedUpdatedAt = %v", rec.body["expectedUpdatedAt"])
	}
	tags, _ := rec.body["tags"].([]any)
	if len(tags) != 1 || tags[0] != "redis" {
		t.Errorf("tags = %v", rec.body["tags"])
	}
}

func TestPostMortemsUpdate_contentNeedsExpectedUpdatedAt(t *testing.T) {
	seedSessionFor(t, "http://127.0.0.1:1")
	_, stderr, err := runCmd(t, "new body", "post-mortems", "update", "5", "--content-file", "-", "--yes", "--output", "json")
	if err == nil || !strings.Contains(stderr.String(), "--expected-updated-at is required") {
		t.Fatalf("want a usage error, got %v %s", err, stderr.String())
	}
}

func TestPostMortemsUpdate_unlinkIncident(t *testing.T) {
	rec := &recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":5,"title":"Redis outage","contentMd":"old","tags":[],"incidentId":11,"updatedAt":"2026-10-15T10:05:00Z"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":5}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	t.Cleanup(func() { flagYes = false })

	if _, stderr, err := runCmd(t, "", "post-mortems", "update", "5", "--unlink-incident", "--yes", "--output", "json"); err != nil {
		t.Fatalf("update: %v %s", err, stderr.String())
	}
	if v, ok := rec.body["incidentId"]; !ok || v != nil {
		t.Errorf("incidentId = %v (present %v), want null", v, ok)
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

	rec.calls = nil
	if _, _, err := runCmd(t, "", "endpoints", "mark-slow", "GET /export", "--yes", "--output", "json"); err == nil || len(rec.calls) != 0 {
		t.Errorf("mark-slow without --offset-ms must fail before any request (0 would remove the mark), calls = %v", rec.calls)
	}
}
