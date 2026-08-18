// traceway-runner executes browser synthetic checks for a Traceway instance
// running with SYNTHETICS_BROWSER_MODE=remote. It long-polls the backend over
// HTTPS (outbound only, NAT-friendly), runs claimed @playwright/test specs
// via a local Node + Playwright harness, and posts results back.
//
// Runners are operator infrastructure: they authenticate with the same
// SYNTHETICS_RUNNER_SECRET the server was deployed with and self-register
// under their name on first poll. Nothing is created in the dashboard.
//
// Configuration (environment):
//
//	TRACEWAY_URL             base URL of the Traceway instance (required)
//	TRACEWAY_RUNNER_SECRET   the server's SYNTHETICS_RUNNER_SECRET (required)
//	TRACEWAY_RUNNER_NAME     name this runner registers under (default: hostname)
//	TRACEWAY_RUNNER_MONITORING  optional Traceway SDK connection string
//	                         (<project_token>@<url>/api/report). When set the
//	                         runner reports its own telemetry: server metrics
//	                         (CPU, memory, goroutines), one task trace per
//	                         executed run (tagged check_id/run_id/status), and
//	                         exceptions for infrastructure failures.
//	RUNNER_WORKERS           concurrent browser runs (default 2)
//	SYNTHETICS_PLAYWRIGHT_DIR  harness dir with @playwright/test (default .)
//	SYNTHETICS_BROWSER_SANDBOX  auto (default) | bwrap | off
//
// Harness setup (once): npm init -y && npm i @playwright/test && npx playwright install chromium
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/synthetics/browserexec"
	traceway "go.tracewayapp.com"
)

const version = "1.0.0"

type runnerConfig struct {
	baseURL         string
	secret          string
	name            string
	monitoring      string
	workers         int
	harnessDir      string
	sandbox         string
	resolvedSandbox browserexec.Sandbox
}

type job struct {
	RunId          int               `json:"runId"`
	CheckId        int               `json:"checkId"`
	TimeoutSeconds int               `json:"timeoutSeconds"`
	Script         string            `json:"script"`
	Env            map[string]string `json:"env"`
}

func main() {
	cfg := loadConfig()
	if err := browserexec.Validate(cfg.harnessDir); err != nil {
		log.Fatalf("Playwright harness unusable at %s: %v\nSet it up with:\n  npm init -y && npm i @playwright/test && npx playwright install chromium\nor point SYNTHETICS_PLAYWRIGHT_DIR at an existing harness.", cfg.harnessDir, err)
	}
	sandbox, err := browserexec.ResolveSandbox(cfg.sandbox, cfg.harnessDir)
	if err != nil {
		log.Fatalf("SYNTHETICS_BROWSER_SANDBOX=%s is unusable: %v\nInstall bubblewrap (apt install bubblewrap / dnf install bubblewrap), or set SYNTHETICS_BROWSER_SANDBOX=off to accept unconfined check scripts.", cfg.sandbox, err)
	}
	cfg.resolvedSandbox = sandbox
	if sandbox == browserexec.SandboxOff {
		log.Print("WARNING: check scripts run unconfined. They can read this host's filesystem and process table, including this runner's own secret, and they can see each other's runs. Install bubblewrap and set SYNTHETICS_BROWSER_SANDBOX=bwrap before pointing this runner at a shared instance.")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.monitoring != "" {
		if err := traceway.Init(cfg.monitoring,
			traceway.WithServerName(cfg.name),
			traceway.WithVersion(version),
		); err != nil {
			log.Printf("self-monitoring disabled: %v", err)
		} else {
			log.Printf("self-monitoring enabled (server name: %s)", cfg.name)
		}
	}

	log.Printf("traceway-runner %s (%s): polling %s (workers: %d, harness: %s, sandbox: %s)", version, cfg.name, cfg.baseURL, cfg.workers, cfg.harnessDir, cfg.resolvedSandbox)

	jobs := make(chan job)
	var wg sync.WaitGroup
	for i := 0; i < cfg.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				executeGuarded(ctx, cfg, j)
			}
		}()
	}

	pollLoop(ctx, cfg, jobs)
	close(jobs)
	wg.Wait()
	log.Println("traceway-runner stopped")
}

func loadConfig() runnerConfig {
	cfg := runnerConfig{
		baseURL:    strings.TrimRight(os.Getenv("TRACEWAY_URL"), "/"),
		secret:     os.Getenv("TRACEWAY_RUNNER_SECRET"),
		name:       strings.TrimSpace(os.Getenv("TRACEWAY_RUNNER_NAME")),
		monitoring: strings.TrimSpace(os.Getenv("TRACEWAY_RUNNER_MONITORING")),
		workers:    2,
		harnessDir: os.Getenv("SYNTHETICS_PLAYWRIGHT_DIR"),
		sandbox:    os.Getenv("SYNTHETICS_BROWSER_SANDBOX"),
	}
	if cfg.baseURL == "" || cfg.secret == "" {
		log.Fatal("TRACEWAY_URL and TRACEWAY_RUNNER_SECRET are required (the secret must match the server's SYNTHETICS_RUNNER_SECRET)")
	}
	if cfg.name == "" {
		if hostname, err := os.Hostname(); err == nil {
			cfg.name = hostname
		} else {
			cfg.name = "unnamed"
		}
	}
	if raw := os.Getenv("RUNNER_WORKERS"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 1 && parsed <= 16 {
			cfg.workers = parsed
		}
	}
	if cfg.harnessDir == "" {
		cfg.harnessDir = "."
	}
	return cfg
}

// pollClient outlives the server's 25s long-poll hold.
var pollClient = &http.Client{Timeout: 40 * time.Second}
var resultClient = &http.Client{Timeout: 30 * time.Second}

func pollLoop(ctx context.Context, cfg runnerConfig, jobs chan<- job) {
	for {
		if ctx.Err() != nil {
			return
		}
		claimed, err := poll(ctx, cfg)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("poll failed: %v (retrying in 5s)", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		for _, j := range claimed {
			select {
			case <-ctx.Done():
				return
			case jobs <- j:
			}
		}
	}
}

func poll(ctx context.Context, cfg runnerConfig) ([]job, error) {
	body, _ := json.Marshal(map[string]int{"maxJobs": cfg.workers})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.baseURL+"/api/runners/poll", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	setAuthHeaders(req, cfg)
	resp, err := pollClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		log.Fatalf("the runner secret was rejected: %s", strings.TrimSpace(string(data)))
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("poll returned %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var parsed struct {
		Jobs []job `json:"jobs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	return parsed.Jobs, nil
}

// executeGuarded isolates a panicking job: log it, report it, keep the
// worker alive for the next claim.
func executeGuarded(ctx context.Context, cfg runnerConfig, j job) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("run %d (check %d): panicked: %v", j.RunId, j.CheckId, r)
			traceway.CaptureException(fmt.Errorf("panic executing run %d (check %d): %v", j.RunId, j.CheckId, r))
		}
	}()
	execute(ctx, cfg, j)
}

func execute(ctx context.Context, cfg runnerConfig, j job) {
	timeout := time.Duration(j.TimeoutSeconds) * time.Second
	if timeout < 5*time.Second || timeout > 2*time.Minute {
		timeout = time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	log.Printf("run %d (check %d): executing", j.RunId, j.CheckId)
	started := time.Now()
	result, err := browserexec.Run(runCtx, browserexec.Options{
		HarnessDir: cfg.harnessDir,
		Script:     j.Script,
		Env:        j.Env,
		Timeout:    timeout,
		Sandbox:    cfg.resolvedSandbox,
	})

	payload := map[string]any{"status": "down", "latencyMs": float64(time.Since(started).Microseconds()) / 1000}
	if err != nil {
		payload["errorMessage"] = "browser run failed to start: " + err.Error()
		traceway.CaptureException(fmt.Errorf("browser run %d (check %d) failed to start: %w", j.RunId, j.CheckId, err))
	} else {
		if result.DurationMs > 0 {
			payload["latencyMs"] = result.DurationMs
		}
		if result.Passed {
			payload["status"] = "up"
		} else {
			payload["errorMessage"] = result.ErrorMsg
			if len(result.Screenshot) > 0 {
				payload["screenshotBase64"] = base64.StdEncoding.EncodeToString(result.Screenshot)
			}
			if result.Output != "" {
				payload["outputTail"] = result.Output
			}
		}
	}
	log.Printf("run %d (check %d): %s", j.RunId, j.CheckId, payload["status"])

	// One task trace per executed run; no-op unless self-monitoring is on.
	traceway.CaptureTask(
		&traceway.TraceContext{Id: uuid.NewString(), IsTask: true},
		"browser_run",
		time.Since(started),
		started,
		map[string]string{
			"check_id": strconv.Itoa(j.CheckId),
			"run_id":   strconv.Itoa(j.RunId),
			"status":   payload["status"].(string),
		},
	)

	if err := postResult(ctx, cfg, j.RunId, payload); err != nil {
		log.Printf("run %d: failed to report result: %v", j.RunId, err)
		traceway.CaptureException(fmt.Errorf("failed to report result for run %d (check %d): %w", j.RunId, j.CheckId, err))
	}
}

func postResult(ctx context.Context, cfg runnerConfig, runId int, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	// A couple of quick retries; past that the server reclaims the stale
	// claim and re-runs, so the result would be discarded anyway.
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return lastErr
			case <-time.After(2 * time.Second):
			}
		}
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, cfg.baseURL+"/api/runners/results/"+strconv.Itoa(runId), bytes.NewReader(body))
		if reqErr != nil {
			return reqErr
		}
		setAuthHeaders(req, cfg)
		resp, doErr := resultClient.Do(req)
		if doErr != nil {
			lastErr = doErr
			continue
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		lastErr = fmt.Errorf("result returned %d", resp.StatusCode)
	}
	return lastErr
}

func setAuthHeaders(req *http.Request, cfg runnerConfig) {
	req.Header.Set("Authorization", "Bearer "+cfg.secret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Traceway-Runner-Version", version)
	req.Header.Set("X-Traceway-Runner-Name", cfg.name)
}
