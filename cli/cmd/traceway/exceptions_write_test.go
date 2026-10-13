//go:build write

package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
)

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
