package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/v2/internal/exitcode"
)

const sessionsFixture = `{"data":[
	{"id":"00000000-0000-0000-0000-000000000005","startedAt":"2026-06-23T14:30:00Z","duration":12500000000,"appVersion":"3.2.0","attributes":{"userId":"u_42"},"hasRecording":true,
	 "recordingReasons":[{"triggerId":"journey","name":"User journey","sequence":"click(#open)","revision":"v1","firedAt":"2026-06-23T14:30:05Z"}]},
	{"id":"00000000-0000-0000-0000-000000000006","startedAt":"2026-06-23T14:00:00Z","duration":0,"attributes":{}}
],"pagination":{"page":1,"pageSize":50,"total":2,"totalPages":1}}`

func TestSessionsList_sendsFiltersAndRendersTable(t *testing.T) {
	var gotBody map[string]any
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/sessions" || r.URL.Query().Get("projectId") != "proj-1" {
			t.Errorf("request = %s %s", r.Method, r.URL)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(sessionsFixture))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, stderr, err := runCmd(t, "", "sessions", "list", "--since", "24h", "--search", "u_42", "--order-by", "duration", "--sort-direction", "asc", "--output", "table")
	if err != nil {
		t.Fatalf("sessions list: %v\n%s", err, stderr.String())
	}
	if gotBody["search"] != "u_42" || gotBody["orderBy"] != "duration" || gotBody["sortDirection"] != "asc" || gotBody["fromDate"] == nil {
		t.Errorf("body = %v", gotBody)
	}
	out := stdout.String()
	for _, want := range []string{"ID", "STARTED AT", "REPLAY", "00000000-0000-0000-0000-000000000005", "2026-06-23T14:30:00Z", "3.2.0", "yes", "u_42"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
}

func TestSessionsList_jsonKeepsRecordingReasons(t *testing.T) {
	var gotBody map[string]any
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(sessionsFixture))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "sessions", "list", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	if gotBody["orderBy"] != "started_at" || gotBody["sortDirection"] != "desc" {
		t.Errorf("default order = %v", gotBody)
	}
	if !strings.Contains(stdout.String(), `"recordingReasons"`) || !strings.Contains(stdout.String(), `"startedAt"`) {
		t.Errorf("json = %s", stdout.String())
	}
}

func TestSessionsList_rejectsUnknownOrder(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("the server must not be hit")
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, _, err := runCmd(t, "", "sessions", "list", "--order-by", "user", "--output", "json")
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != exitcode.Usage {
		t.Fatalf("want a usage error, got %v", err)
	}
}

func TestSessionsShow_looksUpStartedAtWhenOmitted(t *testing.T) {
	var listBody, detailBody map[string]any
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/sessions":
			_ = json.NewDecoder(r.Body).Decode(&listBody)
			_, _ = w.Write([]byte(sessionsFixture))
		case "/api/sessions/00000000-0000-0000-0000-000000000005":
			_ = json.NewDecoder(r.Body).Decode(&detailBody)
			_, _ = w.Write([]byte(`{"session":{"id":"00000000-0000-0000-0000-000000000005","startedAt":"2026-06-23T14:30:00Z"},"exceptions":[{"id":"00000000-0000-0000-0000-0000000000e1","exceptionHash":"8b25da145460880a","stackTrace":"Error: boom","recordedAt":"2026-06-23T14:30:09Z"}]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, stderr, err := runCmd(t, "", "sessions", "show", "00000000-0000-0000-0000-000000000005", "--output", "table")
	if err != nil {
		t.Fatalf("sessions show: %v\n%s", err, stderr.String())
	}
	if listBody["search"] != "00000000-0000-0000-0000-000000000005" {
		t.Errorf("lookup body = %v", listBody)
	}
	if detailBody["startedAt"] != "2026-06-23T14:30:00Z" {
		t.Errorf("detail startedAt = %v, want the looked-up start", detailBody["startedAt"])
	}
	if !strings.Contains(stdout.String(), "8b25da145460880a") {
		t.Errorf("the exceptions table prints the full hash: %s", stdout.String())
	}
}

func TestSessionsShow_unknownSessionWithoutStartedAt(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/sessions" {
			t.Errorf("only the lookup may run, got %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[],"pagination":{"page":1,"pageSize":10,"total":0,"totalPages":0}}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "sessions", "show", "00000000-0000-0000-0000-000000000009", "--output", "json")
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != exitcode.NotFound {
		t.Fatalf("want not_found, got %v", err)
	}
	if !strings.Contains(stderr.String(), "--started-at") {
		t.Errorf("the hint should name --started-at: %s", stderr.String())
	}
}

func TestSessionsShow_badStartedAtIsStillAUsageError(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("the server must not be hit")
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, _, err := runCmd(t, "", "sessions", "show", "00000000-0000-0000-0000-000000000005", "--started-at", "yesterday", "--output", "json")
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != exitcode.Usage {
		t.Fatalf("want a usage error, got %v", err)
	}
}
