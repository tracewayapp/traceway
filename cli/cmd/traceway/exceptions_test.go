package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/config"
	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/state"
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	// Hash should be truncated (first 12 chars by convention)
	if !strings.Contains(out, "abcdef123456") {
		t.Errorf("table missing truncated hash prefix: %s", out)
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

func TestExceptionsArchive_yes_callsClientAndRendersJSON(t *testing.T) {
	var gotHashes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/exception-stack-traces/archive" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, h := range body["hashes"].([]any) {
			gotHashes = append(gotHashes, h.(string))
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	seedSessionFor(t, srv.URL)
	t.Cleanup(func() { flagYes = false })

	stdout, stderr, err := runCmd(t, "", "exceptions", "archive", "--yes", "--output", "json", "h1", "h2")
	if err != nil {
		t.Fatalf("runCmd: %v\nstderr: %s", err, stderr.String())
	}
	if len(gotHashes) != 2 || gotHashes[0] != "h1" || gotHashes[1] != "h2" {
		t.Errorf("server got hashes %v", gotHashes)
	}

	var resp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout.String())
	}
	if resp["action"] != "archive" {
		t.Errorf("action = %v", resp["action"])
	}
	if resp["count"].(float64) != 2 {
		t.Errorf("count = %v", resp["count"])
	}
}

func TestExceptionsArchive_nonTTYWithoutYes_failsWithUsageError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("server should not be called when confirmation refuses")
	}))
	defer srv.Close()

	seedSessionFor(t, srv.URL)

	stdout, stderr, err := runCmd(t, "", "exceptions", "archive", "--output", "json", "h1")
	if err == nil {
		t.Fatalf("expected error, got success: stdout=%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage_error") {
		t.Errorf("stderr should contain usage_error: %s", stderr.String())
	}
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != exitcode.Usage {
		t.Errorf("expected cliError(Usage), got %v", err)
	}
}

func TestExceptionsArchive_requiresAtLeastOneHash(t *testing.T) {
	seedSessionFor(t, "https://unused")
	t.Cleanup(func() { flagYes = false })

	_, _, err := runCmd(t, "", "exceptions", "archive", "--yes")
	if err == nil {
		t.Fatal("expected error from missing args")
	}
}

func TestExceptionsUnarchive_yes_callsClientAndRendersJSON(t *testing.T) {
	var gotPath string
	var gotHashes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, h := range body["hashes"].([]any) {
			gotHashes = append(gotHashes, h.(string))
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	seedSessionFor(t, srv.URL)
	t.Cleanup(func() { flagYes = false })

	stdout, stderr, err := runCmd(t, "", "exceptions", "unarchive", "--yes", "--output", "json", "h1")
	if err != nil {
		t.Fatalf("runCmd: %v\nstderr: %s", err, stderr.String())
	}
	if gotPath != "/api/exception-stack-traces/unarchive" {
		t.Errorf("path = %q", gotPath)
	}
	if len(gotHashes) != 1 || gotHashes[0] != "h1" {
		t.Errorf("server got hashes %v", gotHashes)
	}

	var resp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout.String())
	}
	if resp["action"] != "unarchive" {
		t.Errorf("action = %v", resp["action"])
	}
}
