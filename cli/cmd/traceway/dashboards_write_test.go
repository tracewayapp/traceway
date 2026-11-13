//go:build write

package main

import (
	"net/http"
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
