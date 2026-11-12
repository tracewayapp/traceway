package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpampReadsAndEditionBoundary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("projectId") == "" {
			t.Error("missing project scope")
		}
		if (r.Method != http.MethodGet || r.URL.Path != "/api/sdk-config/rules") &&
			(r.Method != http.MethodPost || r.URL.Path != "/api/sdk-config/preview") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[],"heartbeatIntervalSeconds":300}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	for _, args := range [][]string{{"opamp", "rules", "list"}, {"opamp", "preview", "--file", "-"}} {
		stdout, stderr, err := runCmd(t, `{"attributes":{}}`, append(args, "--output", "json")...)
		if err != nil || !strings.Contains(stdout.String(), "heartbeatIntervalSeconds") {
			t.Fatalf("read: %v %s %s", err, stdout.String(), stderr.String())
		}
	}
	if !editionWrite {
		for _, name := range []string{"create", "update", "delete", "reorder"} {
			cmd, _, err := newRootCmd().Find([]string{"opamp", "rules", name})
			if err == nil && cmd.Name() == name && !cmd.Hidden {
				t.Errorf("read-only edition exposes %s", name)
			}
		}
	}
}
