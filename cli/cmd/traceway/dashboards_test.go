package main

import (
	"net/http"
	"testing"
)

func TestDashboardReadCommandsAndDiscovery(t *testing.T) {
	var calls []string
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		q := r.URL.Query()
		if q.Get("projectId") == "" && r.URL.Path != "/api/dashboards/library" {
			t.Error("missing projectId")
		}
		if r.URL.Path == "/api/metrics/discover/tags" && (q.Get("name") != "jobs.completed" || q.Get("key") != "outcome" || q.Get("from") == "" || q.Get("to") == "") {
			t.Errorf("discovery query: %v", q)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	for _, args := range [][]string{{"dashboards", "list"}, {"dashboards", "library"}, {"dashboards", "export", "42"}, {"metrics", "discover", "--since", "1h"}, {"metrics", "discover", "--name", "jobs.completed", "--key", "outcome", "--since", "24h"}} {
		if _, stderr, err := runCmd(t, "", args...); err != nil {
			t.Fatalf("%v: %v %s", args, err, stderr.String())
		}
	}
	want := []string{"GET /api/dashboards", "GET /api/dashboards/library", "GET /api/dashboards/42/export", "GET /api/metrics/discover", "GET /api/metrics/discover/tags"}
	if len(calls) != len(want) {
		t.Fatalf("calls: %v", calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Errorf("call: %s, want %s", calls[i], want[i])
		}
	}
	if _, _, err := runCmd(t, "", "metrics", "discover", "--name", "jobs.completed"); err == nil || len(calls) != 5 {
		t.Fatal("incomplete tag discovery should not make a request")
	}
	if !editionWrite {
		cmd, _, err := newRootCmd().Find([]string{"dashboards", "import"})
		if err == nil && cmd.Name() == "import" {
			t.Fatal("read edition exposes dashboard import")
		}
	}
}

func TestDashboardValidateOffline(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, tc := range []struct {
		data  string
		valid bool
	}{
		{`{"schemaVersion":1,"name":"Empty","widgets":[]}`, true},
		{`null`, false}, {`{"name":"No widgets"}`, false},
	} {
		_, _, err := runCmd(t, tc.data, "dashboards", "validate", "--file", "-", "--output", "json")
		if (err == nil) != tc.valid {
			t.Errorf("validate %s: %v", tc.data, err)
		}
	}
}
