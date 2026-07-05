package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSessionsList_table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/sessions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"data":[
				{"id":"22222222-2222-2222-2222-222222222222","startedAt":"2026-07-01T12:00:00Z","duration":95000000000,"appVersion":"1.4.0","serverName":"web-1"}
			],
			"pagination":{"total":1}
		}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "sessions", "list", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "22222222-2222-2222-2222-222222222222") {
		t.Errorf("table missing session id: %s", out)
	}
	// startedAt must be RFC3339 so it can feed sessions show --started-at
	if !strings.Contains(out, "2026-07-01T12:00:00Z") {
		t.Errorf("table missing RFC3339 startedAt: %s", out)
	}
}

func TestSessionsList_mapsOrderByAndAttrs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		body := string(buf[:n])
		if !strings.Contains(body, `"orderBy":"started_at"`) {
			t.Errorf("expected snake_case orderBy in body, got: %s", body)
		}
		if !strings.Contains(body, `{"key":"user.id","value":"42"}`) {
			t.Errorf("expected attribute filter in body, got: %s", body)
		}
		_, _ = w.Write([]byte(`{"data":[],"pagination":{}}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	if _, _, err := runCmd(t, "", "sessions", "list",
		"--order-by", "startedAt", "--attr", "user.id=42"); err != nil {
		t.Fatal(err)
	}
}
