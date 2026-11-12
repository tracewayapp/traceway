//go:build write

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpampWrites(t *testing.T) {
	rule := `{"name":"Full sessions","enabled":false,"conditions":{},"settings":{"sessions":{"enabled":true,"baselineSampleRate":1},"replay":{"mode":"stream","sessionSampleRate":1,"maskAllInputs":true}}}`
	for _, tc := range []struct {
		name, method, path string
		args               []string
	}{
		{"create", "POST", "/api/sdk-config/rules", []string{"create", "--file", "-"}},
		{"update", "PUT", "/api/sdk-config/rules/7", []string{"update", "7", "--file", "-", "--expected-updated-at", "2026-11-12T10:00:00Z"}},
		{"delete", "DELETE", "/api/sdk-config/rules/7", []string{"delete", "7"}},
		{"reorder", "PUT", "/api/sdk-config/rules/reorder", []string{"reorder", "7", "2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorded{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("projectId") == "" {
					t.Error("missing project scope")
				}
				rec.add(r)
				_, _ = w.Write([]byte(`{"id":7}`))
			}))
			defer srv.Close()
			seedSessionFor(t, srv.URL)
			flagYes = false
			t.Cleanup(func() { flagYes = false })
			args := append([]string{"opamp", "rules"}, tc.args...)
			if _, _, err := runCmd(t, rule, args...); err == nil || len(rec.calls) != 0 {
				t.Fatal("non-interactive mutation must require --yes before making a request")
			}
			if _, stderr, err := runCmd(t, rule, append(args, "--yes", "--output", "json")...); err != nil {
				t.Fatalf("write: %v %s", err, stderr.String())
			}
			if len(rec.calls) != 1 || rec.calls[0] != tc.method+" "+tc.path {
				t.Fatalf("calls: %v", rec.calls)
			}
			if tc.name == "create" || tc.name == "update" {
				if rec.body["enabled"] != false || rec.body["conditions"] == nil || rec.body["settings"] == nil {
					t.Fatalf("lost rule fields: %v", rec.body)
				}
			}
			if tc.name == "update" && rec.body["expectedUpdatedAt"] != "2026-11-12T10:00:00Z" {
				t.Fatalf("missing version: %v", rec.body)
			}
		})
	}
}

func TestOpampRejectsInvalidInputBeforeRequest(t *testing.T) {
	seedSessionFor(t, "http://127.0.0.1:1")
	for _, body := range []string{`null`, `{}`, `{"name":"x","settings":{}}`, `{"name":"x","conditions":{},"settings":{},"typo":true}`, `{} {}`} {
		_, stderr, err := runCmd(t, body, "opamp", "rules", "create", "--file", "-", "--yes")
		if err == nil || strings.Contains(stderr.String(), "connection refused") {
			t.Errorf("invalid body accepted or sent: %s %v %s", body, err, stderr.String())
		}
	}
	_, _, err := runCmd(t, `{"name":"x","conditions":{},"settings":{}}`, "opamp", "rules", "update", "7", "--file", "-", "--yes")
	if err == nil {
		t.Fatal("update requires --expected-updated-at")
	}
}

func TestOpampSaveSummaryNamesTheIdOnlyForUpdates(t *testing.T) {
	create := opampSaveSummary(false, "Full sessions", 0)
	if create != `About to create OpAMP rule "Full sessions". Settings affect matching browsers at their next poll.` {
		t.Errorf("create summary: %s", create)
	}
	update := opampSaveSummary(true, "Full sessions", 7)
	if update != `About to update OpAMP rule "Full sessions" (id 7). Settings affect matching browsers at their next poll.` {
		t.Errorf("update summary: %s", update)
	}
}
