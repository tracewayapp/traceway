//go:build smoke

package smoke

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSmokeListCommands covers the list/discovery query surface added for
// tasks, sessions, ai-traces, and metrics, plus the new logs filter flags.
// Assertions are envelope-shaped, not row-counted, so the suite passes on a
// quiet instance with no tasks/sessions/AI traces. One login + one
// projects-use is shared to keep wall-clock cost down.
func TestSmokeListCommands(t *testing.T) {
	url, user, pass, proj := requireEnv(t)
	freshXDG(t)
	if _, _, code := runCLI(t, pass+"\n", "login", "--url", url, "--username", user, "--password-stdin"); code != 0 {
		t.Fatal("login failed")
	}
	if _, _, code := runCLI(t, "", "projects", "use", proj); code != 0 {
		t.Fatalf("projects use %s failed", proj)
	}

	// assertEnvelope asserts exit 0 and a {data, pagination} JSON envelope.
	assertEnvelope := func(t *testing.T, name string, args ...string) {
		t.Helper()
		stdout, stderr, code := runCLI(t, "", args...)
		if code != 0 {
			t.Fatalf("%s exit %d\nstderr: %s", name, code, stderr)
		}
		var resp struct {
			Pagination map[string]any `json:"pagination"`
		}
		if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
			t.Fatalf("%s stdout not JSON: %v\n%s", name, err, stdout)
		}
		if resp.Pagination == nil {
			t.Errorf("%s response missing pagination wrapper: %s", name, stdout)
		}
	}

	// assertUsageError asserts exit 2 with a usage_error envelope.
	assertUsageError := func(t *testing.T, name string, args ...string) {
		t.Helper()
		_, stderr, code := runCLI(t, "", args...)
		if code != 2 {
			t.Fatalf("%s exit = %d, want 2\nstderr: %s", name, code, stderr)
		}
		if !strings.Contains(stderr, "usage_error") {
			t.Errorf("%s: expected usage_error envelope, got: %s", name, stderr)
		}
	}

	t.Run("tasks-list", func(t *testing.T) {
		assertEnvelope(t, "tasks list",
			"tasks", "list", "--since", "24h", "--page-size", "5", "--output", "json")
	})

	t.Run("tasks-list-order-by-choices", func(t *testing.T) {
		for _, choice := range []string{"impact", "count", "p50", "p95", "avg", "lastSeen"} {
			_, stderr, code := runCLI(t, "",
				"tasks", "list", "--since", "24h", "--order-by", choice,
				"--page-size", "1", "--output", "json")
			if code != 0 {
				t.Errorf("tasks list --order-by %s exit %d\nstderr: %s", choice, code, stderr)
			}
		}
	})

	t.Run("tasks-list-rejects-wire-order-by", func(t *testing.T) {
		// The CLI takes camelCase sort fields and maps them to the wire's
		// snake_case; passing the wire name directly must be a usage error.
		assertUsageError(t, "tasks list --order-by p95_duration",
			"tasks", "list", "--since", "24h", "--order-by", "p95_duration", "--output", "json")
	})

	t.Run("tasks-list-bad-root-filter", func(t *testing.T) {
		assertUsageError(t, "tasks list --root-filter bogus",
			"tasks", "list", "--since", "24h", "--root-filter", "bogus", "--output", "json")
	})

	t.Run("tasks-runs", func(t *testing.T) {
		assertEnvelope(t, "tasks runs",
			"tasks", "runs", "--since", "24h", "--page-size", "5", "--output", "json")
	})

	t.Run("tasks-runs-scoped-no-such-task", func(t *testing.T) {
		// Scoping to a task name that doesn't exist is empty data, not an error.
		assertEnvelope(t, "tasks runs --task <none>",
			"tasks", "runs", "--since", "24h", "--task", "no-such-task-zzz",
			"--page-size", "1", "--output", "json")
	})

	t.Run("sessions-list", func(t *testing.T) {
		assertEnvelope(t, "sessions list",
			"sessions", "list", "--since", "24h", "--page-size", "5", "--output", "json")
	})

	t.Run("sessions-list-bad-order-by", func(t *testing.T) {
		assertUsageError(t, "sessions list --order-by started_at",
			"sessions", "list", "--since", "24h", "--order-by", "started_at", "--output", "json")
	})

	t.Run("ai-traces-list", func(t *testing.T) {
		assertEnvelope(t, "ai-traces list",
			"ai-traces", "list", "--since", "24h", "--page-size", "5", "--output", "json")
	})

	t.Run("ai-traces-list-order-by-choices", func(t *testing.T) {
		for _, choice := range []string{"count", "p50", "p95", "avg", "totalTokens", "totalCost", "lastSeen"} {
			_, stderr, code := runCLI(t, "",
				"ai-traces", "list", "--since", "24h", "--order-by", choice,
				"--page-size", "1", "--output", "json")
			if code != 0 {
				t.Errorf("ai-traces list --order-by %s exit %d\nstderr: %s", choice, code, stderr)
			}
		}
	})

	t.Run("metrics-list", func(t *testing.T) {
		stdout, stderr, code := runCLI(t, "", "metrics", "list", "--output", "json")
		if code != 0 {
			t.Fatalf("metrics list exit %d\nstderr: %s", code, stderr)
		}
		var resp struct {
			Metrics []map[string]any `json:"metrics"`
		}
		if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
			t.Fatalf("metrics list stdout not JSON: %v\n%s", err, stdout)
		}
	})

	t.Run("metrics-list-search-no-match", func(t *testing.T) {
		stdout, stderr, code := runCLI(t, "",
			"metrics", "list", "--search", "no.such.metric.zzz", "--output", "json")
		if code != 0 {
			t.Fatalf("metrics list --search exit %d\nstderr: %s", code, stderr)
		}
		var resp struct {
			Metrics []map[string]any `json:"metrics"`
		}
		if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
			t.Fatalf("stdout not JSON: %v\n%s", err, stdout)
		}
		if len(resp.Metrics) != 0 {
			t.Errorf("expected no matches, got %d", len(resp.Metrics))
		}
	})

	t.Run("metrics-tags-unknown-metric", func(t *testing.T) {
		_, stderr, code := runCLI(t, "",
			"metrics", "tags", "no.such.metric.zzz", "--output", "json")
		if code != 5 {
			t.Fatalf("metrics tags <unknown> exit = %d, want 5\nstderr: %s", code, stderr)
		}
		if !strings.Contains(stderr, "not_found") {
			t.Errorf("expected not_found envelope, got: %s", stderr)
		}
	})

	t.Run("logs-min-severity-name", func(t *testing.T) {
		assertEnvelope(t, "logs query --min-severity error",
			"logs", "query", "--since", "1h", "--min-severity", "error",
			"--page-size", "1", "--output", "json")
	})

	t.Run("logs-min-severity-bogus", func(t *testing.T) {
		assertUsageError(t, "logs query --min-severity severe",
			"logs", "query", "--since", "1h", "--min-severity", "severe", "--output", "json")
	})

	t.Run("logs-attr-bad-scope", func(t *testing.T) {
		assertUsageError(t, "logs query --attr span:k=v",
			"logs", "query", "--since", "1h", "--attr", "span:k=v", "--output", "json")
	})

	t.Run("logs-attr-no-match", func(t *testing.T) {
		assertEnvelope(t, "logs query --attr <none>",
			"logs", "query", "--since", "1h", "--attr", "no.such.key=zzz",
			"--page-size", "1", "--output", "json")
	})

	t.Run("logs-exclude-without-distributed", func(t *testing.T) {
		assertUsageError(t, "logs query --exclude-trace-id alone",
			"logs", "query", "--since", "1h", "--exclude-trace-id", "abc", "--output", "json")
	})

	t.Run("logs-bad-distributed-trace-id", func(t *testing.T) {
		assertUsageError(t, "logs query --distributed-trace-id not-a-uuid",
			"logs", "query", "--since", "1h", "--distributed-trace-id", "not-a-uuid", "--output", "json")
	})
}
