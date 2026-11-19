//go:build write

package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestAlertingWriteValidationAndReplacement(t *testing.T) {
	calls := 0
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/user-notification-rules" || r.Method != "PUT" || r.URL.Query().Get("projectId") == "" {
			t.Errorf("request: %s %s", r.Method, r.URL)
		}
		_, _ = w.Write([]byte(`{"high":[],"low":[]}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	valid := `{"high":[],"low":[]}`
	if _, stderr, err := runCmd(t, valid, "notification-chains", "replace", "--file", "-", "--yes"); err != nil {
		t.Fatalf("%v %s", err, stderr)
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	if _, _, err := runCmd(t, `{"high":[]}`, "notification-chains", "replace", "--file", "-", "--yes"); err == nil || calls != 1 {
		t.Fatal("invalid replacement reached server")
	}
	if _, _, err := runCmd(t, valid, "notification-chains", "replace", "--file", "-"); err == nil || calls != 1 {
		t.Fatal("unconfirmed replacement reached server")
	}
}
func TestAlertingOfflineValidation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out, stderr, err := runCmd(t, `{"enabled":false}`, "contact-methods", "validate", "update", "4", "--file", "-")
	if err != nil {
		t.Fatalf("%v %s", err, stderr)
	}
	if !strings.Contains(out.String(), `"valid":true`) {
		t.Errorf("output %s", out)
	}
	if _, _, err := runCmd(t, `{"name":"Ops"}`, "teams", "validate", "create", "--organization", "../1", "--file", "-"); err == nil {
		t.Fatal("invalid organization accepted")
	}
}
