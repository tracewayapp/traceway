package browserexec

import (
	"strings"
	"testing"
)

func TestParseReportPassed(t *testing.T) {
	report := `{
		"suites": [{
			"specs": [{
				"title": "page loads",
				"ok": true,
				"tests": [{"results": [{"status": "passed", "duration": 812}]}]
			}]
		}],
		"stats": {"duration": 950, "expected": 1, "unexpected": 0}
	}`
	summary, err := parseReport([]byte(report))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !summary.passed || summary.errorMsg != "" {
		t.Fatalf("expected pass, got %+v", summary)
	}
	if summary.durationMs != 950 {
		t.Errorf("duration = %f, want 950", summary.durationMs)
	}
}

func TestParseReportFailureWithScreenshot(t *testing.T) {
	report := `{
		"suites": [{
			"suites": [{
				"specs": [{
					"title": "login works",
					"ok": false,
					"tests": [{"results": [{
						"status": "failed",
						"duration": 3000,
						"error": {"message": "\u001b[31mexpect(locator).toContainText\u001b[0m failed"},
						"attachments": [
							{"name": "trace", "contentType": "application/zip", "path": "/tmp/x/trace.zip"},
							{"name": "screenshot", "contentType": "image/png", "path": "/tmp/x/test-failed-1.png"}
						]
					}]}]
				}]
			}]
		}],
		"stats": {"duration": 3200}
	}`
	summary, err := parseReport([]byte(report))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if summary.passed {
		t.Fatal("expected failure")
	}
	if !strings.HasPrefix(summary.errorMsg, "login works: ") || strings.Contains(summary.errorMsg, "\x1b") {
		t.Errorf("error message not prefixed/ANSI-stripped: %q", summary.errorMsg)
	}
	if summary.screenshotPath != "/tmp/x/test-failed-1.png" {
		t.Errorf("screenshot path = %q", summary.screenshotPath)
	}
}

func TestParseReportNoTests(t *testing.T) {
	summary, err := parseReport([]byte(`{"suites": [], "errors": [{"message": "SyntaxError: unexpected token"}], "stats": {"duration": 12}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if summary.passed {
		t.Fatal("a report with no tests must not pass")
	}
	if !strings.Contains(summary.errorMsg, "SyntaxError") {
		t.Errorf("expected the top-level error surfaced, got %q", summary.errorMsg)
	}
}

func TestValidateMissingHarness(t *testing.T) {
	if err := Validate(t.TempDir()); err == nil {
		t.Fatal("expected an error for an empty harness dir")
	}
}
