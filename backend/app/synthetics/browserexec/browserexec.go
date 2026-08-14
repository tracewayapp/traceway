// Package browserexec runs @playwright/test specs by spawning Node against a
// prepared harness directory (package.json + node_modules with a pinned
// @playwright/test). It is shared by the embedded browser executor and the
// standalone traceway-runner binary. The harness never uses npm/npx at
// runtime: the Playwright CLI is invoked as
// node <harness>/node_modules/@playwright/test/cli.js.
package browserexec

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type Options struct {
	// HarnessDir holds package.json + node_modules with @playwright/test.
	HarnessDir string
	// Script is the full @playwright/test spec text.
	Script string
	// Env is the check-scoped environment passed to the spec. The parent
	// process environment is deliberately NOT inherited: user scripts must
	// never see JWT_SECRET, DB credentials, or anything else from the server.
	Env map[string]string
	// Timeout bounds the whole run; the per-test Playwright timeout is set
	// slightly below it so test failures surface as test errors, not kills.
	Timeout time.Duration
}

type Result struct {
	Passed     bool
	DurationMs float64
	ErrorMsg   string
	// Screenshot is the first failure screenshot Playwright captured, if any.
	Screenshot []byte
	// Output is the tail of stdout+stderr for runs that died before the JSON
	// report existed.
	Output string
}

const maxOutputBytes = 16 << 10

// Validate reports whether browser checks can execute with this harness:
// node resolvable and the Playwright CLI present.
func Validate(harnessDir string) error {
	if _, err := exec.LookPath("node"); err != nil {
		return fmt.Errorf("node is not on PATH: %w", err)
	}
	cli := filepath.Join(harnessDir, "node_modules", "@playwright", "test", "cli.js")
	if _, err := os.Stat(cli); err != nil {
		return fmt.Errorf("playwright CLI not found at %s (install @playwright/test in the harness directory): %w", cli, err)
	}
	return nil
}

// Run executes one spec and parses the JSON reporter output.
func Run(ctx context.Context, opts Options) (*Result, error) {
	// The run dir lives INSIDE the harness dir so Node module resolution
	// walks up into the harness node_modules from the spec file.
	runsDir := filepath.Join(opts.HarnessDir, ".runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create runs dir: %w", err)
	}
	runDir, err := os.MkdirTemp(runsDir, "run-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create run dir: %w", err)
	}
	defer os.RemoveAll(runDir)

	specDir := filepath.Join(runDir, "spec")
	outputDir := filepath.Join(runDir, "output")
	homeDir := filepath.Join(runDir, "home")
	for _, dir := range []string{specDir, outputDir, homeDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create run subdir: %w", err)
		}
	}
	if err := os.WriteFile(filepath.Join(specDir, "check.spec.ts"), []byte(opts.Script), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write spec: %w", err)
	}

	// The per-test timeout stays under the hard deadline so assertion
	// failures and hangs surface as Playwright errors instead of a kill.
	testTimeout := opts.Timeout - 5*time.Second
	if testTimeout < 5*time.Second {
		testTimeout = opts.Timeout * 8 / 10
	}
	configPath := filepath.Join(runDir, "traceway.config.cjs")
	configSrc := fmt.Sprintf(`module.exports = {
	testDir: %q,
	outputDir: %q,
	timeout: %d,
	retries: 0,
	workers: 1,
	reporter: [['json']],
	use: {
		headless: true,
		screenshot: 'only-on-failure',
		launchOptions: { args: ['--no-sandbox', '--disable-dev-shm-usage'] },
	},
};
`, specDir, outputDir, testTimeout.Milliseconds())
	if err := os.WriteFile(configPath, []byte(configSrc), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	reportPath := filepath.Join(runDir, "report.json")
	cli := filepath.Join(opts.HarnessDir, "node_modules", "@playwright", "test", "cli.js")

	cmd := exec.CommandContext(ctx, "node", cli, "test", "--config", configPath)
	cmd.Dir = opts.HarnessDir
	// Allowlisted environment only; see Options.Env.
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + homeDir,
		"TMPDIR=" + runDir,
		"PLAYWRIGHT_JSON_OUTPUT_NAME=" + reportPath,
	}
	// HOME is sandboxed to the run dir, which would also hide Playwright's
	// default per-user browser cache; resolve it from the REAL home first.
	browsers := os.Getenv("PLAYWRIGHT_BROWSERS_PATH")
	if browsers == "" {
		browsers = defaultBrowsersPath()
	}
	if browsers != "" {
		env = append(env, "PLAYWRIGHT_BROWSERS_PATH="+browsers)
	}
	for name, value := range opts.Env {
		name = strings.TrimSpace(name)
		if name == "" || strings.ContainsAny(name, "=\x00") {
			continue
		}
		env = append(env, name+"="+value)
	}
	cmd.Env = env
	configureProcessGroup(cmd)

	output, runErr := cmd.CombinedOutput()
	if len(output) > maxOutputBytes {
		output = output[len(output)-maxOutputBytes:]
	}

	result := &Result{Output: string(output)}

	report, reportErr := os.ReadFile(reportPath)
	if reportErr != nil {
		// The runner died before writing a report (bad node install, OOM,
		// killed by the deadline).
		if ctx.Err() != nil {
			result.ErrorMsg = "the test run exceeded its timeout and was killed"
		} else if runErr != nil {
			result.ErrorMsg = "playwright did not produce a report: " + firstOutputLines(string(output), 5)
		} else {
			result.ErrorMsg = "playwright did not produce a report"
		}
		return result, nil
	}

	parsed, err := parseReport(report)
	if err != nil {
		result.ErrorMsg = "failed to parse the playwright report: " + err.Error()
		return result, nil
	}
	result.Passed = parsed.passed
	result.DurationMs = parsed.durationMs
	result.ErrorMsg = parsed.errorMsg
	if parsed.screenshotPath != "" {
		if data, err := os.ReadFile(parsed.screenshotPath); err == nil {
			result.Screenshot = data
		}
	}
	return result, nil
}

type reportSummary struct {
	passed         bool
	durationMs     float64
	errorMsg       string
	screenshotPath string
}

type jsonReport struct {
	Suites []jsonSuite `json:"suites"`
	Errors []jsonError `json:"errors"`
	Stats  struct {
		Duration   float64 `json:"duration"`
		Expected   int     `json:"expected"`
		Unexpected int     `json:"unexpected"`
	} `json:"stats"`
}

type jsonSuite struct {
	Suites []jsonSuite `json:"suites"`
	Specs  []jsonSpec  `json:"specs"`
}

type jsonSpec struct {
	Title string     `json:"title"`
	Ok    bool       `json:"ok"`
	Tests []jsonTest `json:"tests"`
}

type jsonTest struct {
	Results []jsonTestResult `json:"results"`
}

type jsonTestResult struct {
	Status      string           `json:"status"`
	Duration    float64          `json:"duration"`
	Error       *jsonError       `json:"error"`
	Errors      []jsonError      `json:"errors"`
	Attachments []jsonAttachment `json:"attachments"`
}

type jsonError struct {
	Message string `json:"message"`
}

type jsonAttachment struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Path        string `json:"path"`
}

func parseReport(data []byte) (*reportSummary, error) {
	var report jsonReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}

	summary := &reportSummary{passed: true, durationMs: report.Stats.Duration}
	specCount := 0
	var walk func(suite jsonSuite)
	walk = func(suite jsonSuite) {
		for _, spec := range suite.Specs {
			specCount++
			if !spec.Ok {
				summary.passed = false
			}
			for _, test := range spec.Tests {
				for _, testResult := range test.Results {
					if testResult.Status == "passed" || testResult.Status == "skipped" {
						continue
					}
					if summary.errorMsg == "" {
						summary.errorMsg = spec.Title + ": " + stripAnsi(firstErrorMessage(testResult))
					}
					if summary.screenshotPath == "" {
						for _, attachment := range testResult.Attachments {
							if attachment.ContentType == "image/png" && attachment.Path != "" {
								summary.screenshotPath = attachment.Path
								break
							}
						}
					}
				}
			}
		}
		for _, child := range suite.Suites {
			walk(child)
		}
	}
	for _, suite := range report.Suites {
		walk(suite)
	}

	if specCount == 0 {
		summary.passed = false
		summary.errorMsg = "the script contains no tests"
		if len(report.Errors) > 0 {
			summary.errorMsg = stripAnsi(report.Errors[0].Message)
		}
	}
	return summary, nil
}

func firstErrorMessage(result jsonTestResult) string {
	if result.Error != nil && result.Error.Message != "" {
		return result.Error.Message
	}
	for _, e := range result.Errors {
		if e.Message != "" {
			return e.Message
		}
	}
	return "test " + result.Status
}

// defaultBrowsersPath mirrors Playwright's per-OS registry default, needed
// because the subprocess runs with a sandboxed HOME.
func defaultBrowsersPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Caches", "ms-playwright")
	case "windows":
		return filepath.Join(home, "AppData", "Local", "ms-playwright")
	default:
		return filepath.Join(home, ".cache", "ms-playwright")
	}
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripAnsi(s string) string {
	s = ansiPattern.ReplaceAllString(s, "")
	if len(s) > 2000 {
		s = s[:2000]
	}
	return s
}

func firstOutputLines(output string, n int) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return stripAnsi(strings.Join(lines, " | "))
}
