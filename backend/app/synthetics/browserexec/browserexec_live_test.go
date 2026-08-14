package browserexec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestRunLive executes real Playwright specs against a local server. Skipped
// unless TRACEWAY_BROWSEREXEC_LIVE_HARNESS points at a harness directory with
// @playwright/test and Chromium installed (CI does not run browsers).
func TestRunLive(t *testing.T) {
	harness := os.Getenv("TRACEWAY_BROWSEREXEC_LIVE_HARNESS")
	if harness == "" {
		t.Skip("TRACEWAY_BROWSEREXEC_LIVE_HARNESS not set")
	}
	if err := Validate(harness); err != nil {
		t.Fatalf("harness invalid: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Traceway Test</title></head><body><h1 id="msg">hello from ` + os.Getenv("USER") + `</h1></body></html>`))
	}))
	defer server.Close()

	t.Run("passing spec", func(t *testing.T) {
		script := `import { test, expect } from '@playwright/test';

test('page loads', async ({ page }) => {
  await page.goto(process.env.TARGET_URL!);
  await expect(page).toHaveTitle(/Traceway Test/);
  await expect(page.locator('#msg')).toContainText('hello');
});
`
		result, err := Run(context.Background(), Options{
			HarnessDir: harness,
			Script:     script,
			Env:        map[string]string{"TARGET_URL": server.URL},
			Timeout:    60 * time.Second,
		})
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		if !result.Passed {
			t.Fatalf("expected pass, got error %q output %q", result.ErrorMsg, result.Output)
		}
		if result.DurationMs <= 0 {
			t.Errorf("duration = %f, want > 0", result.DurationMs)
		}
	})

	t.Run("failing spec with screenshot", func(t *testing.T) {
		script := `import { test, expect } from '@playwright/test';

test('wrong title', async ({ page }) => {
  await page.goto(process.env.TARGET_URL!);
  await expect(page.locator('#msg')).toContainText('definitely not there', { timeout: 2000 });
});
`
		result, err := Run(context.Background(), Options{
			HarnessDir: harness,
			Script:     script,
			Env:        map[string]string{"TARGET_URL": server.URL},
			Timeout:    60 * time.Second,
		})
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		if result.Passed {
			t.Fatal("expected failure")
		}
		if !strings.Contains(result.ErrorMsg, "definitely not there") && !strings.Contains(result.ErrorMsg, "toContainText") {
			t.Errorf("unexpected error message: %q", result.ErrorMsg)
		}
		if len(result.Screenshot) == 0 {
			t.Error("expected a failure screenshot")
		}
	})

	t.Run("secrets are not inherited", func(t *testing.T) {
		script := `import { test, expect } from '@playwright/test';

test('env allowlist', async () => {
  expect(process.env.TRACEWAY_FAKE_SECRET).toBeUndefined();
  expect(process.env.CHECK_VAR).toBe('visible');
});
`
		t.Setenv("TRACEWAY_FAKE_SECRET", "must-not-leak")
		result, err := Run(context.Background(), Options{
			HarnessDir: harness,
			Script:     script,
			Env:        map[string]string{"CHECK_VAR": "visible"},
			Timeout:    60 * time.Second,
		})
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		if !result.Passed {
			t.Fatalf("env allowlist violated: %q", result.ErrorMsg)
		}
	})
}
