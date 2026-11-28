//go:build smoke

package smoke

import (
	"encoding/json"
	"strings"
	"testing"
)

func logsLogin(t *testing.T) {
	t.Helper()
	url, user, pass, proj := requireEnv(t)
	freshXDG(t)
	if _, _, code := runCLI(t, pass+"\n", "login", "--url", url, "--username", user, "--password-stdin"); code != 0 {
		t.Fatal("login failed")
	}
	if _, _, code := runCLI(t, "", "projects", "use", proj); code != 0 {
		t.Fatalf("projects use %s failed", proj)
	}
}

func TestSmokeLogsQuery(t *testing.T) {
	logsLogin(t)

	stdout, stderr, code := runCLI(t, "",
		"logs", "query", "--since", "1h", "--page-size", "5", "--output", "json")
	if code != 0 {
		t.Fatalf("logs query exit %d\nstderr: %s", code, stderr)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("logs query stdout not JSON: %v\n%s", err, stdout)
	}
	if _, ok := resp["data"]; !ok {
		t.Errorf("logs query response missing 'data' field: %s", stdout)
	}
}

func TestSmokeLogsQueryBadSortDirection(t *testing.T) {
	logsLogin(t)
	_, stderr, code := runCLI(t, "",
		"logs", "query", "--since", "1h",
		"--sort-direction", "bogus", "--output", "json")
	if code != 2 {
		t.Fatalf("logs query --sort-direction bogus exit = %d, want 2\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "usage_error") {
		t.Errorf("expected usage_error envelope, got: %s", stderr)
	}
}

func TestSmokeLogsQueryTraceID(t *testing.T) {
	logsLogin(t)
	_, stderr, code := runCLI(t, "",
		"logs", "query", "--since", "1h",
		"--trace-id", "00000000000000000000000000000000",
		"--output", "json")
	if code != 1 || !strings.Contains(stderr, "invalid traceId") {
		t.Fatalf("all-zero trace ID: exit %d, stderr %s", code, stderr)
	}
	stdout, stderr, code := runCLI(t, "",
		"logs", "query", "--since", "1h",
		"--trace-id", "00000000000000000000000000000001",
		"--output", "json")
	if code != 0 {
		t.Fatalf("logs query with a valid trace ID: exit %d\nstderr: %s", code, stderr)
	}
	var resp struct {
		Data       []map[string]any `json:"data"`
		Pagination map[string]any   `json:"pagination"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("logs query stdout not JSON: %v\n%s", err, stdout)
	}
	if len(resp.Data) != 0 {
		t.Errorf("unmatched trace ID returned %d rows, expected 0: %s", len(resp.Data), stdout)
	}
}

func TestSmokeLogsQueryFarFuture(t *testing.T) {
	logsLogin(t)
	stdout, stderr, code := runCLI(t, "",
		"logs", "query",
		"--from", "2099-01-01T00:00:00Z",
		"--to", "2099-01-02T00:00:00Z",
		"--output", "json")
	if code != 0 {
		t.Fatalf("logs query (far future) exit %d\nstderr: %s", code, stderr)
	}
	var resp struct {
		Data       []map[string]any `json:"data"`
		Pagination map[string]any   `json:"pagination"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("logs query far-future stdout not JSON: %v\n%s", err, stdout)
	}
	if len(resp.Data) != 0 {
		t.Errorf("far-future window returned %d rows, expected 0: %s", len(resp.Data), stdout)
	}
}
