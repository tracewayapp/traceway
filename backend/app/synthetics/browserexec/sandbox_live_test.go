package browserexec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const (
	sentinelPrefix = "traceway-sentinel-"
	sentinelSuffix = "8f3a1c2e"
)

func TestSandboxLive(t *testing.T) {
	harness := os.Getenv("TRACEWAY_BROWSEREXEC_LIVE_HARNESS")
	if harness == "" {
		t.Skip("TRACEWAY_BROWSEREXEC_LIVE_HARNESS not set")
	}
	if err := Validate(harness); err != nil {
		t.Fatalf("harness invalid: %v", err)
	}
	sandbox, err := ResolveSandbox("bwrap", harness)
	if err != nil {
		t.Skipf("this host cannot sandbox: %v", err)
	}
	if sandbox != SandboxBwrap {
		t.Fatalf("expected bwrap, got %q", sandbox)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Traceway Sandbox</title></head><body><h1 id="msg">sandboxed</h1></body></html>`))
	}))
	defer server.Close()

	t.Run("a real spec runs inside the sandbox", func(t *testing.T) {
		script := `import { test, expect } from '@playwright/test';

test('page loads', async ({ page }) => {
  await page.goto(process.env.TARGET_URL!);
  await expect(page).toHaveTitle(/Traceway Sandbox/);
  await expect(page.locator('#msg')).toContainText('sandboxed');
});
`
		result, err := Run(context.Background(), Options{
			HarnessDir: harness,
			Script:     script,
			Env:        map[string]string{"TARGET_URL": server.URL},
			Timeout:    90 * time.Second,
			Sandbox:    SandboxBwrap,
		})
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		if !result.Passed {
			t.Fatalf("Chromium could not run under the sandbox, the bind set is probably incomplete: %q\noutput: %s", result.ErrorMsg, result.Output)
		}
	})

	const scanScript = `import { test, expect } from '@playwright/test';
import * as fs from 'fs';

test('parent environment', async () => {
  const sentinel = process.env.SENTINEL_A! + process.env.SENTINEL_B!;
  let found = false;
  for (const name of fs.readdirSync('/proc')) {
    if (!/^\d+$/.test(name)) continue;
    try {
      if (fs.readFileSync('/proc/' + name + '/environ', 'utf8').includes(sentinel)) {
        found = true;
        break;
      }
    } catch { /* not readable, keep going */ }
  }
  expect(found).toBe(process.env.EXPECT_FOUND === 'true');
});
`

	scanEnv := func(expectFound string) map[string]string {
		return map[string]string{
			"SENTINEL_A":   sentinelPrefix,
			"SENTINEL_B":   sentinelSuffix,
			"EXPECT_FOUND": expectFound,
		}
	}

	holder := exec.Command("sleep", "120")
	holder.Env = []string{"TRACEWAY_SENTINEL=" + sentinelPrefix + sentinelSuffix}
	if err := holder.Start(); err != nil {
		t.Fatalf("failed to start the sentinel holder: %v", err)
	}
	defer func() {
		holder.Process.Kill()
		holder.Wait()
	}()

	t.Run("the scan detects a leak when unconfined", func(t *testing.T) {
		result, err := Run(context.Background(), Options{
			HarnessDir: harness,
			Script:     scanScript,
			Env:        scanEnv("true"),
			Timeout:    90 * time.Second,
			Sandbox:    SandboxOff,
		})
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		if !result.Passed {
			t.Fatalf("the scan could not observe the sentinel even unconfined, so it cannot prove the sandbox works: %q", result.ErrorMsg)
		}
	})

	t.Run("the parent environment is unreachable when sandboxed", func(t *testing.T) {
		result, err := Run(context.Background(), Options{
			HarnessDir: harness,
			Script:     scanScript,
			Env:        scanEnv("false"),
			Timeout:    90 * time.Second,
			Sandbox:    SandboxBwrap,
		})
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		if !result.Passed {
			t.Fatalf("a sandboxed spec could still read the parent environment: %q", result.ErrorMsg)
		}
	})

	t.Run("the harness cannot be poisoned", func(t *testing.T) {
		probe := filepath.Join(harness, "node_modules", ".traceway-poison-probe")
		defer os.Remove(probe)

		script := `import { test, expect } from '@playwright/test';
import * as fs from 'fs';

test('harness is read-only', async () => {
  let blocked = false;
  try {
    fs.writeFileSync(process.env.PROBE_PATH!, 'poisoned');
  } catch {
    blocked = true;
  }
  expect(blocked).toBe(true);
});
`
		result, err := Run(context.Background(), Options{
			HarnessDir: harness,
			Script:     script,
			Env:        map[string]string{"PROBE_PATH": probe},
			Timeout:    90 * time.Second,
			Sandbox:    SandboxBwrap,
		})
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		if !result.Passed {
			t.Fatalf("a sandboxed spec could write into the shared harness: %q", result.ErrorMsg)
		}
		if _, err := os.Stat(probe); err == nil {
			t.Fatal("the poison probe reached the harness on disk")
		}
	})

	t.Run("a sibling run dir is unreadable", func(t *testing.T) {
		// Plant a sibling run dir with a secret next to where this run's dir
		// will be created, exactly as a concurrent check would.
		runsDir := filepath.Join(harness, ".runs")
		if err := os.MkdirAll(runsDir, 0o755); err != nil {
			t.Fatalf("failed to create the runs dir: %v", err)
		}
		sibling, err := os.MkdirTemp(runsDir, "sibling-*")
		if err != nil {
			t.Fatalf("failed to create the sibling run dir: %v", err)
		}
		defer os.RemoveAll(sibling)
		secret := sentinelPrefix + "sibling-" + sentinelSuffix
		if err := os.WriteFile(filepath.Join(sibling, "creds.txt"), []byte(secret), 0o600); err != nil {
			t.Fatalf("failed to write the sibling secret: %v", err)
		}

		script := `import { test, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

test('sibling run dirs are hidden', async () => {
  const runs = path.dirname(process.cwd());
  const own = path.basename(process.cwd());
  const siblings = fs.readdirSync(runs).filter((e) => e !== own);
  let leaked = '';
  for (const e of siblings) {
    try { leaked += fs.readFileSync(path.join(runs, e, 'creds.txt'), 'utf8'); } catch { /* expected: not present */ }
  }
  expect(siblings).toHaveLength(0);
  expect(leaked).toBe('');
});
`
		result, err := Run(context.Background(), Options{
			HarnessDir: harness,
			Script:     script,
			Timeout:    90 * time.Second,
			Sandbox:    SandboxBwrap,
		})
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		if !result.Passed {
			t.Fatalf("a sandboxed run could see or read a sibling run dir: %q\noutput: %s", result.ErrorMsg, result.Output)
		}
	})

	t.Run("the run dir stays writable", func(t *testing.T) {
		script := `import { test, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

test('scratch writes work', async () => {
  const file = path.join(process.cwd(), 'scratch.txt');
  fs.writeFileSync(file, 'ok');
  expect(fs.readFileSync(file, 'utf8')).toBe('ok');
});
`
		result, err := Run(context.Background(), Options{
			HarnessDir: harness,
			Script:     script,
			Timeout:    90 * time.Second,
			Sandbox:    SandboxBwrap,
		})
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		if !result.Passed {
			t.Fatalf("the run dir is not writable inside the sandbox: %q", result.ErrorMsg)
		}
	})
}
