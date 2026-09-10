package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/apifixture"
)

func TestAgentAttempts_listsAndFilters(t *testing.T) {
	server := apifixture.New(t)
	seedSessionFor(t, server.URL)
	stdout, stderr, err := runCmd(t, "", "agent", "attempts", "--status", "awaiting_review", "-o", "json")
	if err != nil {
		t.Fatalf("agent attempts: %v\n%s", err, stderr.String())
	}
	out := stdout.String()
	var resp struct {
		Data []struct {
			Number int    `json:"number"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if len(resp.Data) != 2 || resp.Data[0].Number != 2 || resp.Data[0].Status != "awaiting_review" {
		t.Fatalf("data = %+v", resp.Data)
	}
	requests := server.Requests()
	if len(requests) != 1 || requests[0].Method != "POST" || requests[0].Path != "/api/agent/attempts/list" || !strings.Contains(requests[0].Query, "projectId=proj-1") {
		t.Fatalf("requests = %+v", requests)
	}
	tableOut, _, err := runCmd(t, "", "agent", "attempts", "-o", "table")
	if err != nil || !strings.Contains(tableOut.String(), "awaiting_review") || !strings.Contains(tableOut.String(), "NUMBER") {
		t.Fatalf("table = %v\n%s", err, tableOut.String())
	}
}

func TestAgentShow_printsLinksAndEvents(t *testing.T) {
	server := apifixture.New(t)
	seedSessionFor(t, server.URL)
	stdout, stderr, err := runCmd(t, "", "agent", "show", apifixture.Known.AttemptID.String(), "-o", "json")
	if err != nil {
		t.Fatalf("agent show: %v\n%s", err, stderr.String())
	}
	out := stdout.String()
	var resp attemptShowResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if resp.Attempt.Number != 2 || len(resp.Links) != 2 || len(resp.Events) != 5 || resp.Events[4].Kind != "status" {
		t.Fatalf("show = %+v", resp)
	}
	textOut, _, err := runCmd(t, "", "agent", "show", apifixture.Known.AttemptID.String(), "-o", "table")
	text := textOut.String()
	if err != nil || !strings.Contains(text, "Attempt 2 on 0123456789abcdef") || !strings.Contains(text, "acme/app#42") || !strings.Contains(text, "assistant_text") {
		t.Fatalf("text = %v\n%s", err, text)
	}
}

func TestAgentReport_sendsTheOutcome(t *testing.T) {
	server := apifixture.New(t)
	seedSessionFor(t, server.URL)
	reportFile := filepath.Join(t.TempDir(), "report.md")
	if err := os.WriteFile(reportFile, []byte("STATUS: fixed\nHASH: 0123456789abcdef\nThe nil map write in cache.go.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runCmd(t, "", "agent", "report", "--hash", "0123456789abcdef", "--status", "fixed", "--branch", "traceway/fix-0123456789abcdef-77.1", "--pr", "https://github.com/acme/app/pull/77", "--report-file", reportFile, "-o", "json")
	if err != nil {
		t.Fatalf("agent report: %v\n%s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"number":3`) && !strings.Contains(out, `"number": 3`) {
		t.Fatalf("output = %s", out)
	}
	requests := server.Requests()
	if len(requests) != 1 || requests[0].Method != "POST" || requests[0].Path != "/api/agent/attempts/report" {
		t.Fatalf("requests = %+v", requests)
	}
	body := requests[0].Body
	if !strings.Contains(body, `"hash":"0123456789abcdef"`) || !strings.Contains(body, `"status":"fixed"`) || !strings.Contains(body, `"pullRequestUrl":"https://github.com/acme/app/pull/77"`) || !strings.Contains(body, `"report":"The nil map write in cache.go."`) || strings.Contains(body, "STATUS:") {
		t.Fatalf("body = %s", body)
	}

	if _, _, err := runCmd(t, "", "agent", "report", "--hash", "0123456789abcdef", "--status", "done", "--report", "x"); err == nil {
		t.Fatal("an unknown status must be refused before any request")
	}
	if _, _, err := runCmd(t, "", "agent", "report", "--hash", "0123456789abcdef", "--status", "analysis"); err == nil {
		t.Fatal("a report is required")
	}
	if len(server.Requests()) != 1 {
		t.Fatalf("refused reports must not reach the server: %+v", server.Requests())
	}
	if got := stripReportHeader("STATUS: analysis\nHASH: x\nSUBJECT: y\n\nBody line\n"); got != "Body line" {
		t.Fatalf("stripReportHeader = %q", got)
	}
}
