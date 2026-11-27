package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

const cloud = "https://cloud.tracewayapp.com"

func approvedProject(id, name, framework, token, uploadToken, backendURL string) map[string]any {
	p := map[string]any{"id": id, "name": name, "framework": framework, "token": token, "backendUrl": backendURL, "status": "created"}
	if uploadToken != "" {
		p["sourceMapToken"] = uploadToken
	}
	return p
}

func TestSetupApplySelfHostedWithoutAppBaseURLWritesItsOwnURL(t *testing.T) {
	setupTestEnvIsolation(t)
	// A server without APP_BASE_URL used to report the cloud address for itself.
	server := newSetupTestServerWith(t, 0, cloud, []map[string]any{
		approvedProject("p-web", "Web", "react", "tok_web-secret", "", cloud),
	})
	defer server.Close()

	dir := t.TempDir()
	planPath := writePlan(t, dir, map[string]any{"url": server.URL + "/", "projects": []map[string]any{
		{"name": "Web", "framework": "react", "envFile": "apps/web/.env.local", "envVar": "VITE_TRACEWAY_TOKEN"},
	}})

	if _, stderr, err := runCmd(t, testSetupToken+"\n", "setup", "apply", "--plan", planPath, "--token-stdin"); err != nil {
		t.Fatalf("setup apply failed: %v\nstderr: %s", err, stderr.String())
	}
	got := readFile(t, filepath.Join(dir, "apps", "web", ".env.local"))
	want := "VITE_TRACEWAY_TOKEN=tok_web-secret\nVITE_TRACEWAY_URL=" + server.URL + "\n"
	if got != want {
		t.Errorf("env = %q, want %q (the env file's directories are created too)", got, want)
	}
	if strings.Contains(got, "cloud.tracewayapp.com") || strings.Contains(got, "/api/report") {
		t.Errorf("a self-hosted env file must not name the cloud or /api/report: %q", got)
	}
}

func TestSetupApplyWritesUploadTokensWhenThePlanAsks(t *testing.T) {
	setupTestEnvIsolation(t)
	selfHosted := "https://traceway.acme.test"
	server := newSetupTestServerWith(t, 0, selfHosted, []map[string]any{
		approvedProject("p-ios", "App iOS", "ios", "tok_ios-secret", "up_ios-secret", selfHosted),
		approvedProject("p-android", "App Android", "android", "tok_android-secret", "up_android-secret", selfHosted),
		approvedProject("p-web", "App Web", "react", "tok_web-secret", "up_web-secret", selfHosted),
		approvedProject("p-flutter", "App Flutter", "flutter", "tok_flutter-secret", "up_flutter-secret", selfHosted),
	})
	defer server.Close()

	dir := t.TempDir()
	planPath := writePlan(t, dir, map[string]any{"url": server.URL, "projects": []map[string]any{
		{"name": "App iOS", "framework": "ios", "envFile": "ios/Traceway.xcconfig", "envVar": "TRACEWAY_TOKEN",
			"uploadTokenEnvFile": "ios/Traceway.secrets.xcconfig"},
		{"name": "App Android", "framework": "android", "envFile": "android/local.properties", "envVar": "traceway.token",
			"uploadTokenEnvVar": "traceway.uploadToken"},
		{"name": "App Web", "framework": "react", "envFile": "web/.env", "envVar": "VITE_TRACEWAY_TOKEN"},
		{"name": "App Flutter", "framework": "flutter", "envFile": "flutter/.env", "envVar": "TRACEWAY_TOKEN",
			"uploadTokenEnvVar": "TRACEWAY_UPLOAD_TOKEN"},
	}})

	stdout, stderr, err := runCmd(t, testSetupToken+"\n", "setup", "apply", "--plan", planPath, "--token-stdin", "--output", "table")
	if err != nil {
		t.Fatalf("setup apply failed: %v\nstderr: %s", err, stderr.String())
	}
	for _, secret := range []string{"tok_ios-secret", "up_ios-secret", "up_android-secret", "up_web-secret", "up_flutter-secret"} {
		if strings.Contains(stdout.String()+stderr.String(), secret) {
			t.Fatalf("token value %s leaked into the output", secret)
		}
	}

	files := map[string]string{
		// "//" starts a comment in an .xcconfig file.
		"ios/Traceway.xcconfig":         "TRACEWAY_TOKEN=tok_ios-secret\nTRACEWAY_URL=https:/$()/traceway.acme.test\n",
		"ios/Traceway.secrets.xcconfig": "TRACEWAY_UPLOAD_TOKEN=up_ios-secret\nTRACEWAY_URL=https:/$()/traceway.acme.test\n",
		"android/local.properties":      "traceway.token=tok_android-secret\ntraceway.url=https://traceway.acme.test\ntraceway.uploadToken=up_android-secret\n",
		"web/.env":                      "VITE_TRACEWAY_TOKEN=tok_web-secret\nVITE_TRACEWAY_URL=https://traceway.acme.test\n",
		"flutter/.env":                  "TRACEWAY_TOKEN=tok_flutter-secret\nTRACEWAY_URL=https://traceway.acme.test\nTRACEWAY_UPLOAD_TOKEN=up_flutter-secret\n",
	}
	for rel, want := range files {
		if got := readFile(t, filepath.Join(dir, rel)); got != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}

	if !strings.Contains(stdout.String(), "UPLOAD TOKEN") || !strings.Contains(stdout.String(), "on the Connection page") {
		t.Errorf("table should say where the web upload token is: %s", stdout.String())
	}
	// The plan decides whether a release build needs the upload token; one it did not ask for is not pushed.
	if strings.Contains(stdout.String(), "Upload tokens") || strings.Contains(stdout.String(), "give the release build") {
		t.Errorf("summary should not suggest an upload token the plan did not ask for: %s", stdout.String())
	}
}

func TestSetupApplyJSONReportsUploadTokens(t *testing.T) {
	setupTestEnvIsolation(t)
	server := newSetupTestServerWith(t, 0, "https://traceway.acme.test", []map[string]any{
		approvedProject("p-ios", "App iOS", "ios", "tok_ios-secret", "up_ios-secret", "https://traceway.acme.test"),
		approvedProject("p-api", "Api", "opentelemetry", "tok_api-secret", "", "https://traceway.acme.test"),
	})
	defer server.Close()

	dir := t.TempDir()
	planPath := writePlan(t, dir, map[string]any{"url": server.URL, "projects": []map[string]any{
		{"name": "App iOS", "framework": "ios", "envFile": "ios/Traceway.xcconfig", "envVar": "TRACEWAY_TOKEN", "uploadTokenEnvVar": "TRACEWAY_UPLOAD_TOKEN"},
		{"name": "Api", "framework": "opentelemetry", "envFile": ".env", "envVar": "TRACEWAY_TOKEN"},
	}})

	stdout, stderr, err := runCmd(t, testSetupToken+"\n", "setup", "apply", "--plan", planPath, "--token-stdin", "--output", "json")
	if err != nil {
		t.Fatalf("setup apply failed: %v\nstderr: %s", err, stderr.String())
	}
	if strings.Contains(stdout.String(), "secret") {
		t.Fatalf("json output leaked a token value: %s", stdout.String())
	}
	var payload struct {
		Projects []setupApplyProjectSummary `json:"projects"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	ios, api := payload.Projects[0], payload.Projects[1]
	if !ios.HasUploadToken || ios.UploadTokenEnvFile != "ios/Traceway.xcconfig" || ios.UploadTokenEnvVar != "TRACEWAY_UPLOAD_TOKEN" || ios.UploadTokenResult != "added" {
		t.Errorf("ios summary = %+v", ios)
	}
	if ios.URLEnvVar != "TRACEWAY_URL" || ios.EnvResult != "created" {
		t.Errorf("ios env summary = %+v", ios)
	}
	if api.HasUploadToken || api.UploadTokenEnvFile != "" {
		t.Errorf("an OpenTelemetry project has no upload token: %+v", api)
	}
	got := readFile(t, filepath.Join(dir, "ios", "Traceway.xcconfig"))
	if got != "TRACEWAY_TOKEN=tok_ios-secret\nTRACEWAY_URL=https:/$()/traceway.acme.test\nTRACEWAY_UPLOAD_TOKEN=up_ios-secret\n" {
		t.Errorf("xcconfig = %q", got)
	}
}

func TestSetupApplyOldPlanKeepsAnEquivalentConnectionString(t *testing.T) {
	setupTestEnvIsolation(t)
	server := newSetupTestServerWith(t, 0, "https://traceway.example.com", []map[string]any{
		approvedProject("p-web", "Web", "svelte", "tok_web-secret", "", "https://traceway.example.com"),
		approvedProject("p-mobile", "Mobile", "flutter", "tok_mobile-secret", "", "https://traceway.example.com"),
	})
	defer server.Close()

	dir := t.TempDir()
	planPath := writePlan(t, dir, map[string]any{"url": server.URL, "projects": []map[string]any{
		{"name": "Web", "framework": "svelte", "envFile": "web/.env", "envVar": "PUBLIC_TRACEWAY_CONNECTION_STRING", "envFormat": "connectionString"},
		{"name": "Mobile", "framework": "flutter", "envFile": "mobile/.env", "envVar": "TRACEWAY_CONNECTION_STRING", "envFormat": "connectionString"},
	}})
	// Written by an older CLI: the 1.x suffix, spaces and quotes; the SDKs read every one of these the same way.
	oldWeb := "OTHER=1\nPUBLIC_TRACEWAY_CONNECTION_STRING=\"  tok_web-secret@traceway.example.com/api/report/  \"\n"
	if err := os.MkdirAll(filepath.Join(dir, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "web", ".env"), []byte(oldWeb), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runCmd(t, testSetupToken+"\n", "setup", "apply", "--plan", planPath, "--token-stdin")
	if err != nil {
		t.Fatalf("setup apply failed: %v\nstderr: %s", err, stderr.String())
	}
	if got := readFile(t, filepath.Join(dir, "web", ".env")); got != oldWeb {
		t.Errorf("an equivalent connection string must be left alone, got %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "mobile", ".env")); got != "TRACEWAY_CONNECTION_STRING=tok_mobile-secret@https://traceway.example.com\n" {
		t.Errorf("an old plan gets <token>@<server> without /api/report, got %q", got)
	}
	if !strings.Contains(stdout.String(), "unchanged") {
		t.Errorf("the kept web env file should report unchanged: %s", stdout.String())
	}
}

func TestSetupApplyInvalidURLIsAUsageError(t *testing.T) {
	setupTestEnvIsolation(t)
	dir := t.TempDir()
	planPath := writeSetupPlanFile(t, dir, "ftp://traceway.example.com")
	_, _, err := runCmd(t, "", "setup", "apply", "--plan", planPath, "--token", testSetupToken)
	var cliErr *cliError
	if !asCLIError(err, &cliErr) || cliErr.code != 2 {
		t.Errorf("error = %v, want usage error (exit 2)", err)
	}
}

func TestSetupApplyAcceptsAReportURL(t *testing.T) {
	setupTestEnvIsolation(t)
	server := newSetupTestServer(t, 0)
	defer server.Close()

	dir := t.TempDir()
	planPath := writeSetupPlanFile(t, dir, server.URL+"/api/report")
	if _, stderr, err := runCmd(t, testSetupToken+"\n", "setup", "apply", "--plan", planPath, "--token-stdin"); err != nil {
		t.Fatalf("a plan url with the 1.x suffix should reach the server: %v\nstderr: %s", err, stderr.String())
	}
}

func TestSetupInstanceURL(t *testing.T) {
	cases := []struct{ server, cli, want string }{
		{cloud, cloud, cloud},
		{cloud + "/", cloud, cloud},
		{"https://traceway.acme.test/", "http://10.0.0.5:8082", "https://traceway.acme.test"},
		{cloud, "http://traceway.internal:8082", "http://traceway.internal:8082"},
		{"", "http://traceway.internal:8082", "http://traceway.internal:8082"},
		{"ftp://broken", "https://traceway.acme.test", "https://traceway.acme.test"},
		{"https://corp.example/traceway/api/report", "https://corp.example/traceway", "https://corp.example/traceway"},
	}
	for _, tc := range cases {
		if got := setupInstanceURL(tc.server, tc.cli); got != tc.want {
			t.Errorf("setupInstanceURL(%q, %q) = %q, want %q", tc.server, tc.cli, got, tc.want)
		}
	}
}

func TestSetupEnvWritesOnCloudNeedNoURL(t *testing.T) {
	p := client.SetupPlanProject{Name: "Web", Framework: "react", EnvFile: ".env", EnvVar: "VITE_TRACEWAY_TOKEN", UploadTokenEnvFile: ".env.sourcemaps"}
	writes := setupEnvWrites(p, "tok", cloud)
	if len(writes) != 1 || writes[0].key != "VITE_TRACEWAY_TOKEN" || writes[0].value != "tok" {
		t.Errorf("cloud writes = %+v, want the token only", writes)
	}
	upload := setupUploadTokenWrites(p, "up", cloud, writes)
	if len(upload) != 1 || upload[0].file != ".env.sourcemaps" || upload[0].key != "TRACEWAY_SOURCEMAP_TOKEN" {
		t.Errorf("cloud upload writes = %+v", upload)
	}
	if got := setupUploadTokenWrites(client.SetupPlanProject{Framework: "react", EnvFile: ".env", EnvVar: "X"}, "up", cloud, writes); got != nil {
		t.Errorf("an upload token the plan does not ask for is not written: %+v", got)
	}
	if got := setupUploadTokenWrites(p, "", cloud, writes); got != nil {
		t.Errorf("no upload token, nothing to write: %+v", got)
	}

	legacy := setupEnvWrites(client.SetupPlanProject{EnvFile: ".env", EnvVar: "CS", EnvFormat: "connectionString"}, "tok", cloud)
	if len(legacy) != 1 || legacy[0].value != "tok@"+cloud {
		t.Errorf("legacy writes = %+v", legacy)
	}
}

func TestURLEnvVarFor(t *testing.T) {
	cases := map[string]string{
		"TRACEWAY_TOKEN":             "TRACEWAY_URL",
		"VITE_TRACEWAY_TOKEN":        "VITE_TRACEWAY_URL",
		"NEXT_PUBLIC_TRACEWAY_TOKEN": "NEXT_PUBLIC_TRACEWAY_URL",
		"traceway.token":             "traceway.url",
		"TRACEWAY_KEY":               "TRACEWAY_KEY_URL",
		"traceway.key":               "traceway.key.url",
	}
	for envVar, want := range cases {
		if got := urlEnvVarFor(client.SetupPlanProject{EnvVar: envVar}); got != want {
			t.Errorf("urlEnvVarFor(%s) = %s, want %s", envVar, got, want)
		}
	}
	if got := urlEnvVarFor(client.SetupPlanProject{EnvVar: "TRACEWAY_TOKEN", URLEnvVar: "MY_SERVER"}); got != "MY_SERVER" {
		t.Errorf("an explicit urlEnvVar wins, got %s", got)
	}
}

func TestValidateSetupPlanStartAPIRules(t *testing.T) {
	valid := []client.SetupPlanProject{
		{Name: "A", Framework: "android", EnvFile: "local.properties", EnvVar: "traceway.token", URLEnvVar: "traceway.url"},
		{Name: "B", Framework: "ios", EnvFile: "Traceway.xcconfig", EnvVar: "TRACEWAY_TOKEN", UploadTokenEnvFile: "Secrets.xcconfig"},
		{Name: "C", Framework: "react", EnvFile: ".env", EnvVar: "VITE_TRACEWAY_TOKEN", UploadTokenEnvVar: "TRACEWAY_SOURCEMAP_TOKEN"},
	}
	if err := validateSetupPlan(&setupPlan{Projects: valid}); err != nil {
		t.Errorf("valid plan rejected: %v", err)
	}
	invalid := map[string]client.SetupPlanProject{
		"dotted key in .env":       {Name: "A", Framework: "android", EnvFile: ".env", EnvVar: "traceway.token"},
		"bad urlEnvVar":            {Name: "A", Framework: "react", EnvFile: ".env", EnvVar: "T", URLEnvVar: "1X"},
		"public upload token":      {Name: "A", Framework: "react", EnvFile: ".env", EnvVar: "T", UploadTokenEnvVar: "VITE_TRACEWAY_UPLOAD_TOKEN"},
		"upload var without file":  {Name: "A", Framework: "ios", UploadTokenEnvVar: "TRACEWAY_UPLOAD_TOKEN"},
		"dotted upload key in env": {Name: "A", Framework: "ios", UploadTokenEnvFile: ".env", UploadTokenEnvVar: "traceway.upload"},
		"upload token over token":  {Name: "A", Framework: "ios", EnvFile: "Traceway.xcconfig", EnvVar: "TRACEWAY_TOKEN", UploadTokenEnvVar: "TRACEWAY_TOKEN"},
		"upload token over url":    {Name: "A", Framework: "ios", EnvFile: "./Traceway.xcconfig", EnvVar: "TRACEWAY_TOKEN", UploadTokenEnvFile: "Traceway.xcconfig", UploadTokenEnvVar: "TRACEWAY_URL"},
		"url over token":           {Name: "A", Framework: "react", EnvFile: ".env", EnvVar: "VITE_TRACEWAY_TOKEN", URLEnvVar: "VITE_TRACEWAY_TOKEN"},
	}
	for name, p := range invalid {
		if err := validateSetupPlan(&setupPlan{Projects: []client.SetupPlanProject{p}}); err == nil {
			t.Errorf("%s: plan accepted, want an error", name)
		}
	}
}
