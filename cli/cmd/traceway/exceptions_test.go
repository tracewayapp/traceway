package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/v2/internal/config"
	"github.com/tracewayapp/traceway/cli/v2/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/v2/internal/state"
)

// seedSessionFor configures config + state for a default profile pointing at
// baseURL with a stored JWT and a current_project_id.
func seedSessionFor(t *testing.T, baseURL string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cfg := &config.Config{Profiles: map[string]config.Profile{
		"default": {URL: baseURL, Username: "fred@example.com"},
	}}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	st := &state.State{
		CurrentProfile: "default",
		Profiles:       map[string]state.ProfileState{"default": {JWT: "tok", CurrentProjectID: "proj-1"}},
	}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestExceptionsList_jsonShape(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/exception-stack-traces" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("projectId") != "proj-1" {
			t.Errorf("projectId = %q", r.URL.Query().Get("projectId"))
		}
		_, _ = w.Write([]byte(`{
			"data":[
				{"exceptionHash":"h1","stackTrace":"t1","count":7,"firstSeen":"2026-05-13T00:00:00Z","lastSeen":"2026-05-13T01:00:00Z"}
			],
			"pagination":{"page":1,"pageSize":50,"total":1,"totalPages":1}
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	t.Cleanup(func() { flagProject = "" })

	stdout, _, err := runCmd(t, "", "exceptions", "list", "--output", "json")
	if err != nil {
		t.Fatalf("exceptions list: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"h1"`) {
		t.Errorf("expected hash in output, got: %s", out)
	}
	if !strings.Contains(out, `"pagination"`) {
		t.Errorf("expected pagination passthrough, got: %s", out)
	}
}

func TestExceptionsList_table(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"data":[
				{"exceptionHash":"abcdef1234567890","stackTrace":"NullPointerException at line 42","count":7,"firstSeen":"2026-05-13T00:00:00Z","lastSeen":"2026-05-13T01:00:00Z"}
			],
			"pagination":{"total":1}
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "exceptions", "list", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	// First line is header
	if !strings.Contains(out, "HASH") || !strings.Contains(out, "COUNT") {
		t.Errorf("table missing headers: %s", out)
	}
	if !strings.Contains(out, "7") {
		t.Errorf("table missing count: %s", out)
	}
	// The full hash, which is what exceptions show takes.
	if !strings.Contains(out, "abcdef1234567890") {
		t.Errorf("table missing the full hash: %s", out)
	}
}

func TestExceptionsList_noSession_writesEnvelope(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	_, stderr, err := runCmd(t, "", "exceptions", "list", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"not_authenticated"`) {
		t.Errorf("expected not_authenticated envelope, got: %s", stderr.String())
	}
}

func TestExceptionsList_invalidTimeRange_writesEnvelope(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[],"pagination":{}}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "exceptions", "list", "--output", "json", "--since", "1h", "--from", "2026-05-13T00:00:00Z", "--to", "2026-05-13T23:59:59Z")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"invalid_time_range"`) {
		t.Errorf("expected invalid_time_range envelope, got: %s", stderr.String())
	}
}

func TestExceptionsList_outOfRangePageSize_writesUsageError(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("server must not be hit when pagination is rejected locally")
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "exceptions", "list", "--output", "json", "--page-size", "200")
	if err == nil {
		t.Fatal("expected error")
	}
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != exitcode.Usage {
		t.Fatalf("expected *cliError with exitcode.Usage, got %v", err)
	}
	if !strings.Contains(stderr.String(), "usage_error") {
		t.Errorf("expected usage_error envelope, got: %s", stderr.String())
	}
}

func TestExceptionsShow_passesHashInPath(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/exception-stack-traces/abc" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"group":{"exceptionHash":"abc","stackTrace":"trace","count":3,"firstSeen":"2026-05-13T00:00:00Z","lastSeen":"2026-05-13T01:00:00Z"},
			"occurrences":[],
			"pagination":{"page":1,"pageSize":20,"total":0,"totalPages":0}
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "exceptions", "show", "abc", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"abc"`) {
		t.Errorf("expected hash in output: %s", out)
	}
}

func TestExceptionsShow_404_writesNotFoundEnvelope(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "exceptions", "show", "missing", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"not_found"`) {
		t.Errorf("expected not_found envelope, got: %s", stderr.String())
	}
}

func prefixTestServer(t *testing.T, hashes ...string) (*httptest.Server, *[]string) {
	t.Helper()
	var shown []string
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/exception-stack-traces":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["includeArchived"] != true || body["orderBy"] != "last_seen" {
				t.Errorf("prefix lookup body = %v", body)
			}
			data := []map[string]any{}
			for _, h := range hashes {
				data = append(data, map[string]any{"exceptionHash": h, "stackTrace": "Error: " + h, "count": 1,
					"firstSeen": "2026-05-13T00:00:00Z", "lastSeen": "2026-05-13T01:00:00Z"})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "pagination": map[string]any{"page": 1, "pageSize": 100, "total": len(data), "totalPages": 1}})
		case strings.HasPrefix(r.URL.Path, "/api/exception-stack-traces/"):
			hash := strings.TrimPrefix(r.URL.Path, "/api/exception-stack-traces/")
			shown = append(shown, hash)
			for _, h := range hashes {
				if h == hash {
					_, _ = fmt.Fprintf(w, `{"group":{"exceptionHash":%q,"stackTrace":"Error: boom","count":2,"firstSeen":"2026-05-13T00:00:00Z","lastSeen":"2026-05-13T01:00:00Z"},"occurrences":[],"pagination":{"page":1,"pageSize":20}}`, h)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &shown
}

func TestExceptionsShow_resolvesAUniqueHashPrefix(t *testing.T) {
	srv, shown := prefixTestServer(t, "8b25da145460880a", "1234567890abcdef")
	seedSessionFor(t, srv.URL)

	stdout, stderr, err := runCmd(t, "", "exceptions", "show", "8b25da145460", "--output", "table")
	if err != nil {
		t.Fatalf("exceptions show <prefix>: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "HASH:        8b25da145460880a") {
		t.Errorf("want the full group, got: %s", stdout.String())
	}
	if want := []string{"8b25da145460", "8b25da145460880a"}; strings.Join(*shown, ",") != strings.Join(want, ",") {
		t.Errorf("detail lookups = %v, want %v (exact first, then the resolved hash)", *shown, want)
	}
}

func TestExceptionsShow_ambiguousPrefixIsAUsageError(t *testing.T) {
	srv, _ := prefixTestServer(t, "8b25da145460880a", "8b25da1454609999")
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "exceptions", "show", "8b25da14", "--output", "json")
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != exitcode.Usage {
		t.Fatalf("want a usage error, got %v", err)
	}
	if !strings.Contains(stderr.String(), "8b25da145460880a") || !strings.Contains(stderr.String(), "8b25da1454609999") {
		t.Errorf("the error should list the candidates: %s", stderr.String())
	}
}

func TestExceptionsShow_unknownPrefixStaysNotFound(t *testing.T) {
	srv, _ := prefixTestServer(t, "8b25da145460880a")
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "exceptions", "show", "ffff0000", "--output", "json")
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != exitcode.NotFound {
		t.Fatalf("want not_found, got %v", err)
	}
	if !strings.Contains(stderr.String(), `"not_found"`) || !strings.Contains(stderr.String(), "exceptions list --since 30d") {
		t.Errorf("want a not_found envelope with a hint: %s", stderr.String())
	}
}
