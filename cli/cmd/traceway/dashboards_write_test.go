//go:build write

package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestDashboardImportConfirmationAndWire(t *testing.T) {
	rec := &recorded{}
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		if r.URL.Query().Get("projectId") == "" {
			t.Error("missing project scope")
		}
		_, _ = w.Write([]byte(`{"created":1,"updated":0,"dashboards":[{"id":42}]}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	t.Cleanup(func() { flagYes = false })
	doc := `{"schemaVersion":1,"name":"Worker","widgets":[]}`
	if _, _, err := runCmd(t, doc, "dashboards", "import", "--file", "-", "--output", "json"); err == nil || len(rec.calls) != 0 {
		t.Fatal("import must require confirmation")
	}
	if _, stderr, err := runCmd(t, doc, "dashboards", "import", "--file", "-", "--mode", "upsert", "--yes", "--output", "json"); err != nil {
		t.Fatalf("import: %v %s", err, stderr.String())
	}
	if len(rec.calls) != 1 || rec.calls[0] != "POST /api/dashboards/import" || rec.body["mode"] != "upsert" {
		t.Fatalf("wire: %v %v", rec.calls, rec.body)
	}
	data, ok := rec.body["data"].(map[string]any)
	if !ok || data["name"] != "Worker" {
		t.Fatalf("data: %v", rec.body)
	}
	for _, input := range []string{`{`, doc + doc, `null`, `{"name":"missing widgets"}`} {
		if _, _, err := runCmd(t, input, "dashboards", "import", "--file", "-", "--yes"); err == nil {
			t.Error("invalid JSON accepted")
		}
	}
	if _, _, err := runCmd(t, doc, "dashboards", "import", "--file", "-", "--mode", "replace", "--yes"); err == nil {
		t.Error("invalid mode accepted")
	}
	if len(rec.calls) != 1 {
		t.Fatalf("invalid input made HTTP calls: %v", rec.calls)
	}
}

func TestDashboardEmbedConfirmationAndWire(t *testing.T) {
	rec := &recorded{}
	var query string
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		query = r.URL.RawQuery
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"dashboardUuid":"d-1","token":"jwt","expiresAt":"2026-12-01T00:00:00Z","expiresInSeconds":259200,"embedPath":"/embed/dashboard?dashboard=d-1#token=jwt"}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	t.Cleanup(func() { flagYes = false })

	if _, _, err := runCmd(t, "", "dashboards", "embed", "42", "--output", "json"); err == nil || len(rec.calls) != 0 {
		t.Fatal("embed must require confirmation")
	}
	stdout, stderr, err := runCmd(t, "", "dashboards", "embed", "42", "--timeframe", "7d", "--expires-in-days", "3",
		"--tag", "account_id=acme", "--tag", "plan=a=b", "--theme", "light", "--yes", "--output", "json")
	if err != nil {
		t.Fatalf("embed: %v %s", err, stderr.String())
	}
	if len(rec.calls) != 1 || rec.calls[0] != "POST /api/dashboards/42/embed-token" {
		t.Fatalf("wire: %v", rec.calls)
	}
	for _, want := range []string{"projectId=", "timeframe=7d", "theme=light"} {
		if !strings.Contains(query, want) {
			t.Errorf("query %q lacks %s", query, want)
		}
	}
	filters, _ := rec.body["tagFilters"].(map[string]any)
	if rec.body["expiresInDays"] != float64(3) || filters["account_id"] != "acme" || filters["plan"] != "a=b" || len(rec.body) != 2 {
		t.Fatalf("body: %v", rec.body)
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil || out["url"] != srv.URL+"/embed/dashboard?dashboard=d-1#token=jwt" || out["expiresInSeconds"] != float64(259200) {
		t.Fatalf("output: %s (%v)", stdout.String(), err)
	}

	if _, _, err := runCmd(t, "", "dashboards", "embed", "42", "--from", "2026-11-01T00:00:00Z", "--to", "2026-11-08T00:00:00Z", "--expires-at", "2026-12-01T00:00:00Z", "--yes", "--output", "json"); err != nil {
		t.Fatal(err)
	}
	if rec.body["from"] != "2026-11-01T00:00:00Z" || rec.body["to"] != "2026-11-08T00:00:00Z" || rec.body["expiresAt"] != "2026-12-01T00:00:00Z" || strings.Contains(query, "timeframe") {
		t.Fatalf("fixed range: %v %s", rec.body, query)
	}

	for _, args := range [][]string{
		{"0"}, {"42", "--timeframe", "week"}, {"42", "--timeframe", "7d", "--from", "2026-11-01T00:00:00Z", "--to", "2026-11-08T00:00:00Z"},
		{"42", "--from", "2026-11-01T00:00:00Z"}, {"42", "--from", "yesterday", "--to", "2026-11-08T00:00:00Z"},
		{"42", "--expires-in-days", "91"}, {"42", "--expires-in-days", "0"}, {"42", "--expires-in-days", "3", "--expires-at", "2026-12-01T00:00:00Z"},
		{"42", "--tag", "account_id"}, {"42", "--tag", "account_id="}, {"42", "--theme", "sepia"},
	} {
		if _, _, err := runCmd(t, "", append(append([]string{"dashboards", "embed"}, args...), "--yes", "--output", "json")...); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	if len(rec.calls) != 2 {
		t.Fatalf("invalid input made HTTP calls: %v", rec.calls)
	}
}
