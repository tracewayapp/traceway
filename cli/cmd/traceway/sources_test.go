package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/config"
)

func TestSourcesList_showsImplicitTraceway(t *testing.T) {
	seedSessionFor(t, "https://primary.example.com")
	stdout, _, err := runCmd(t, "", "sources", "list", "--output", "table")
	if err != nil {
		t.Fatalf("sources list: %v", err)
	}
	if row := tableRow(stdout.String(), "traceway"); len(row) != 4 || row[1] != "traceway" || row[3] != "yes" {
		t.Fatalf("implicit source row = %v in:\n%s", row, stdout.String())
	}
}

// tableRow returns the whitespace-separated cells of the table row starting
// with the given name.
func tableRow(table, name string) []string {
	for _, line := range strings.Split(table, "\n") {
		if fields := strings.Fields(line); len(fields) > 0 && fields[0] == name {
			return fields
		}
	}
	return nil
}

func TestSourcesAdd_rejectsUnknownProvider(t *testing.T) {
	seedSessionFor(t, "https://primary.example.com")
	_, stderr, err := runCmd(t, "", "sources", "add", "datadog", "--name", "dd", "--output", "json")
	if err == nil {
		t.Fatal("expected a usage error")
	}
	if !strings.Contains(stderr.String(), `unknown provider \"datadog\"`) || !strings.Contains(stderr.String(), "registered providers: traceway") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestSourcesAddListRemove_roundTrip(t *testing.T) {
	seedSessionFor(t, "https://primary.example.com")

	_, stderr, err := runCmd(t, "", "sources", "add", "traceway", "--name", "staging", "--domains", "logs,exceptions", "--set", "url=https://staging.example.com", "--set", "token=twp_x")
	if err != nil {
		t.Fatalf("sources add: %v\n%s", err, stderr.String())
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	sources := cfg.Profiles["default"].Sources
	if len(sources) != 1 || sources[0].Name != "staging" || sources[0].Config["token"] != "twp_x" || strings.Join(sources[0].Domains, ",") != "logs,exceptions" {
		t.Fatalf("stored sources = %+v", sources)
	}

	if _, stderr, err = runCmd(t, "", "sources", "add", "traceway", "--name", "staging", "--set", "url=u", "--set", "token=t"); err == nil || !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("duplicate add: err=%v stderr=%s", err, stderr.String())
	}
	if _, stderr, err = runCmd(t, "", "sources", "add", "traceway", "--name", "traceway", "--set", "url=u", "--set", "token=t"); err == nil || !strings.Contains(stderr.String(), "always present") {
		t.Fatalf("implicit name: err=%v stderr=%s", err, stderr.String())
	}
	if _, stderr, err = runCmd(t, "", "sources", "add", "traceway", "--name", "broken", "--set", "url=u"); err == nil || !strings.Contains(stderr.String(), "token") {
		t.Fatalf("missing setting: err=%v stderr=%s", err, stderr.String())
	}
	if _, stderr, err = runCmd(t, "", "sources", "add", "traceway", "--name", "x", "--domains", "weather", "--set", "url=u", "--set", "token=t"); err == nil || !strings.Contains(stderr.String(), `unknown domain \"weather\"`) {
		t.Fatalf("bad domain: err=%v stderr=%s", err, stderr.String())
	}

	stdout, _, err := runCmd(t, "", "sources", "list", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	if row := tableRow(stdout.String(), "staging"); len(row) != 3 || row[1] != "traceway" || row[2] != "logs,exceptions" {
		t.Fatalf("staging row = %v in:\n%s", row, stdout.String())
	}

	if _, _, err := runCmd(t, "", "sources", "remove", "staging"); err != nil {
		t.Fatalf("sources remove: %v", err)
	}
	cfg, _ = config.Load()
	if len(cfg.Profiles["default"].Sources) != 0 {
		t.Fatalf("sources after remove = %+v", cfg.Profiles["default"].Sources)
	}
	if _, stderr, err := runCmd(t, "", "sources", "remove", "staging"); err == nil || !strings.Contains(stderr.String(), "has no source") {
		t.Fatalf("remove again: err=%v stderr=%s", err, stderr.String())
	}
}

func logsServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func seedSecondSource(t *testing.T, url string) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	profile := cfg.Profiles["default"]
	profile.Sources = append(profile.Sources, config.Source{
		Name:     "staging",
		Provider: "traceway",
		Domains:  []string{"logs"},
		Config:   map[string]string{"url": url, "token": "twp_x"},
	})
	cfg.Profiles["default"] = profile
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestLogsQuery_mergesBoundSourcesWithSourceColumn(t *testing.T) {
	primary := logsServer(t, `{"data":[{"id":"00000000-0000-0000-0000-000000000001","timestamp":"2026-05-13T12:00:00Z","severityText":"ERROR","severityNumber":17,"serviceName":"api","body":"from primary"}],"pagination":{"total":1}}`)
	staging := logsServer(t, `{"data":[{"id":"00000000-0000-0000-0000-000000000002","timestamp":"2026-05-13T12:30:00Z","severityText":"WARN","severityNumber":13,"serviceName":"api","body":"from staging"}],"pagination":{"total":1}}`)
	seedSessionFor(t, primary.URL)
	seedSecondSource(t, staging.URL)

	stdout, stderr, err := runCmd(t, "", "logs", "query", "--output", "table")
	if err != nil {
		t.Fatalf("logs query: %v\n%s", err, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "SOURCE") {
		t.Fatalf("expected a SOURCE column and two rows:\n%s", stdout.String())
	}
	if !strings.HasPrefix(lines[1], "staging") || !strings.Contains(lines[1], "from staging") || !strings.HasPrefix(lines[2], "traceway") {
		t.Fatalf("rows must be merged newest first and tagged:\n%s", stdout.String())
	}

	stdout, _, err = runCmd(t, "", "logs", "query", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"source":"staging"`) || !strings.Contains(stdout.String(), `"source":"traceway"`) {
		t.Fatalf("json rows must carry their source:\n%s", stdout.String())
	}

	stdout, _, err = runCmd(t, "", "logs", "query", "--source", "traceway", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "SOURCE") || strings.Contains(stdout.String(), "from staging") {
		t.Fatalf("--source must narrow to one source and drop the column:\n%s", stdout.String())
	}

	_, stderr, err = runCmd(t, "", "logs", "query", "--source", "nope", "--output", "json")
	if err == nil || !strings.Contains(stderr.String(), `unknown source \"nope\"`) {
		t.Fatalf("unknown --source: err=%v stderr=%s", err, stderr.String())
	}
	_, stderr, err = runCmd(t, "", "endpoints", "list", "--source", "staging", "--output", "json")
	if err == nil || !strings.Contains(stderr.String(), "does not answer endpoints") {
		t.Fatalf("--source outside its domains: err=%v stderr=%s", err, stderr.String())
	}
}

func TestLogsQuery_reportsFailedSourceAsWarning(t *testing.T) {
	primary := logsServer(t, `{"data":[{"id":"00000000-0000-0000-0000-000000000001","timestamp":"2026-05-13T12:00:00Z","severityText":"ERROR","severityNumber":17,"serviceName":"api","body":"still here"}],"pagination":{"total":1}}`)
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(broken.Close)
	seedSessionFor(t, primary.URL)
	seedSecondSource(t, broken.URL)

	stdout, stderr, err := runCmd(t, "", "logs", "query", "--output", "json")
	if err != nil {
		t.Fatalf("one healthy source must still answer: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "still here") {
		t.Fatalf("merged answer missing:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning: source staging") {
		t.Fatalf("failed source must be reported on stderr:\n%s", stderr.String())
	}
}
