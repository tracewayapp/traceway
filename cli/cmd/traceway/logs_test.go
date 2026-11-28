package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

func TestLogsQuery_basic(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/logs" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"data":[
				{"id":"00000000-0000-0000-0000-000000000001","timestamp":"2026-05-13T12:00:00Z","severityText":"ERROR","severityNumber":17,"serviceName":"api","body":"failed to connect"}
			],
			"pagination":{"total":1}
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "logs", "query", "--output", "json")
	if err != nil {
		t.Fatalf("logs query: %v", err)
	}
	if !strings.Contains(stdout.String(), "failed to connect") {
		t.Errorf("expected log body in output: %s", stdout.String())
	}
}

func TestLogsQuery_table(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"data":[
				{"id":"00000000-0000-0000-0000-000000000001","timestamp":"2026-05-13T12:00:00Z","severityText":"ERROR","severityNumber":17,"serviceName":"api","body":"failed to connect"}
			],
			"pagination":{"total":1}
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "logs", "query", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "TIMESTAMP") || !strings.Contains(out, "SEVERITY") || !strings.Contains(out, "SERVICE") {
		t.Errorf("table missing headers: %s", out)
	}
	if !strings.Contains(out, "ERROR") || !strings.Contains(out, "api") || !strings.Contains(out, "failed") {
		t.Errorf("table missing row data: %s", out)
	}
}

func TestLogsQuery_passesServiceFilter(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decode body and assert serviceName was passed through
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		body := string(buf[:n])
		if !strings.Contains(body, `"serviceName":"api"`) {
			t.Errorf("expected serviceName=api in body, got: %s", body)
		}
		_, _ = w.Write([]byte(`{"data":[],"pagination":{}}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	if _, _, err := runCmd(t, "", "logs", "query", "--service", "api"); err != nil {
		t.Fatal(err)
	}
}

func TestLogsQuery_tableShowsEventRecords(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"data":[
				{"id":"00000000-0000-0000-0000-000000000001","timestamp":"2026-05-13T12:00:00Z","severityText":"INFO","serviceName":"shop-web","body":"",
				 "logAttributes":{"event.name":"browser.user_action.click","target":"button#add-to-cart","url":"/"}},
				{"id":"00000000-0000-0000-0000-000000000002","timestamp":"2026-05-13T12:00:01Z","severityText":"INFO","serviceName":"shop-web","body":"",
				 "logAttributes":{"event.name":"session.start"}},
				{"id":"00000000-0000-0000-0000-000000000003","timestamp":"2026-05-13T12:00:02Z","severityText":"WARN","serviceName":"shop-web","body":"cart low stock\nsecond line",
				 "logAttributes":{"event.name":"ignored.when.there.is.a.body"}},
				{"id":"00000000-0000-0000-0000-000000000004","timestamp":"2026-05-13T12:00:03Z","severityText":"INFO","serviceName":"shop-web","body":""}
			],
			"pagination":{"total":4}
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "logs", "query", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("want a header and 4 rows:\n%s", stdout.String())
	}
	for i, want := range []string{
		"[browser.user_action.click] target=button#add-to-cart url=/",
		"[session.start]",
		"cart low stock",
	} {
		if !strings.HasSuffix(strings.TrimSpace(lines[i+1]), want) {
			t.Errorf("row %d = %q, want it to end with %q", i+1, lines[i+1], want)
		}
	}
	if strings.Contains(stdout.String(), "ignored.when.there.is.a.body") || strings.Contains(stdout.String(), "second line") {
		t.Errorf("a body wins over the event name and shows its first line only:\n%s", stdout.String())
	}
	if !strings.HasSuffix(strings.TrimSpace(lines[4]), "-") {
		t.Errorf("an empty record shows -: %q", lines[4])
	}
}

func TestLogBodyCellCutsLongEvents(t *testing.T) {
	attrs := map[string]string{"event.name": "exception", "exception.stacktrace": strings.Repeat("x", 400)}
	got := logBodyCell(client.LogRecord{LogAttributes: attrs})
	if len([]rune(got)) != logEventCellLimit || !strings.HasSuffix(got, "…") || !strings.HasPrefix(got, "[exception] exception.stacktrace=") {
		t.Errorf("cell = %q (%d runes)", got, len([]rune(got)))
	}
}
