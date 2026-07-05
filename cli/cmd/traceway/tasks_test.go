package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTasksList_table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/grouped" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"data":[
				{"taskName":"nightly-sync","count":42,"p50Duration":50000000,"p95Duration":150000000,"avgDuration":80000000,"lastSeen":"2026-07-01T12:00:00Z","hasRoot":true,"hasNonRoot":false}
			],
			"pagination":{"total":1}
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "tasks", "list", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "TASK") || !strings.Contains(out, "P95") {
		t.Errorf("table missing headers: %s", out)
	}
	if !strings.Contains(out, "nightly-sync") || !strings.Contains(out, "42") {
		t.Errorf("table missing row data: %s", out)
	}
	if strings.Contains(out, "50000000") {
		t.Errorf("table should format ns as human duration: %s", out)
	}
}

func TestTasksList_mapsOrderByAndRootFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		body := string(buf[:n])
		if !strings.Contains(body, `"orderBy":"p95_duration"`) {
			t.Errorf("expected snake_case orderBy in body, got: %s", body)
		}
		if !strings.Contains(body, `"rootFilter":"non_root"`) {
			t.Errorf("expected rootFilter=non_root in body, got: %s", body)
		}
		_, _ = w.Write([]byte(`{"data":[],"pagination":{}}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	if _, _, err := runCmd(t, "", "tasks", "list", "--order-by", "p95", "--root-filter", "non-root"); err != nil {
		t.Fatal(err)
	}
}

func TestTasksList_rejectsBogusOrderBy(t *testing.T) {
	seedSessionFor(t, "http://127.0.0.1:0")

	_, stderr, err := runCmd(t, "", "tasks", "list", "--order-by", "p95_duration", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"usage_error"`) {
		t.Errorf("expected usage_error envelope, got: %s", stderr.String())
	}
}

func TestTasksRuns_allTasks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"data":[
				{"id":"11111111-1111-1111-1111-111111111111","taskName":"nightly-sync","duration":50000000,"recordedAt":"2026-07-01T12:00:00Z"}
			],
			"pagination":{"total":1}
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "tasks", "runs", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "11111111-1111-1111-1111-111111111111") {
		t.Errorf("table missing run id: %s", out)
	}
	// recordedAt must be shown in RFC3339 so it can feed tasks show --recorded-at
	if !strings.Contains(out, "2026-07-01T12:00:00Z") {
		t.Errorf("table missing RFC3339 recordedAt: %s", out)
	}
}

func TestTasksRuns_scopedToTask_showsStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/task" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("task") != "nightly-sync" {
			t.Errorf("task = %q", r.URL.Query().Get("task"))
		}
		_, _ = w.Write([]byte(`{
			"data":[
				{"id":"11111111-1111-1111-1111-111111111111","taskName":"nightly-sync","duration":50000000,"recordedAt":"2026-07-01T12:00:00Z"}
			],
			"stats":{"count":42,"avgDuration":80.5,"medianDuration":75.0,"p95Duration":150.0,"p99Duration":300.0,"throughput":1.4},
			"pagination":{"total":1}
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "tasks", "runs", "--task", "nightly-sync", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "THROUGHPUT") || !strings.Contains(out, "1.40/min") {
		t.Errorf("table missing stats block: %s", out)
	}
	if !strings.Contains(out, "nightly-sync") {
		t.Errorf("table missing run row: %s", out)
	}
}

func TestTasksRuns_mapsOrderBy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		body := string(buf[:n])
		if !strings.Contains(body, `"orderBy":"recorded_at"`) {
			t.Errorf("expected snake_case orderBy in body, got: %s", body)
		}
		_, _ = w.Write([]byte(`{"data":[],"pagination":{}}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	if _, _, err := runCmd(t, "", "tasks", "runs", "--order-by", "recordedAt"); err != nil {
		t.Fatal(err)
	}
}
