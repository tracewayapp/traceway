package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSetupApplyLinksTheSetupPageThatOpensForExistingOrganizations(t *testing.T) {
	setupTestEnvIsolation(t)
	server := newSetupTestServer(t, 0)
	defer server.Close()

	dir := t.TempDir()
	planPath := writeSetupPlanFile(t, dir, server.URL)
	stdout, stderr, err := runCmd(t, testSetupToken+"\n", "setup", "apply", "--plan", planPath, "--token-stdin", "--output", "table")
	if err != nil {
		t.Fatalf("setup apply failed: %v\nstderr: %s", err, stderr.String())
	}
	page := server.URL + "/setup?mode=add&organizationId=1"
	if !strings.Contains(stderr.String(), "review and approve it at "+page+" ") {
		t.Errorf("approval prompt should link %s: %s", page, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Product Web: add PUBLIC_TRACEWAY_TOKEN and PUBLIC_TRACEWAY_URL (the server URL) to Vercel. The exact command with the secret value is on "+page+".") {
		t.Errorf("next steps should link %s: %s", page, stdout.String())
	}
	if strings.Contains(stdout.String()+stderr.String(), "/setup ") || strings.Contains(stdout.String()+stderr.String(), "/setup.") {
		t.Errorf("no message may link the bare /setup page, which redirects accounts with projects:\n%s\n%s", stdout.String(), stderr.String())
	}
}

func TestSetupPageURL(t *testing.T) {
	if got := setupPageURL("https://traceway.acme.test", 7); got != "https://traceway.acme.test/setup?mode=add&organizationId=7" {
		t.Errorf("setupPageURL = %s", got)
	}
	if got := setupPageURL("https://traceway.acme.test", 0); got != "https://traceway.acme.test/setup?mode=add" {
		t.Errorf("setupPageURL without an organization = %s", got)
	}
}

func TestSetupApplyFlutterWithoutUploadTokenFollowsThePlan(t *testing.T) {
	setupTestEnvIsolation(t)
	server := newSetupTestServerWith(t, 0, "https://traceway.acme.test", []map[string]any{
		approvedProject("p-flutter", "Shop App", "flutter", "tok_flutter-secret", "up_flutter-secret", "https://traceway.acme.test"),
	})
	defer server.Close()

	dir := t.TempDir()
	planPath := writePlan(t, dir, map[string]any{"url": server.URL, "projects": []map[string]any{
		{"name": "Shop App", "framework": "flutter", "envFile": ".env", "envVar": "TRACEWAY_TOKEN"},
	}})
	stdout, stderr, err := runCmd(t, testSetupToken+"\n", "setup", "apply", "--plan", planPath, "--token-stdin", "--output", "table")
	if err != nil {
		t.Fatalf("setup apply failed: %v\nstderr: %s", err, stderr.String())
	}
	for _, unwanted := range []string{"Upload tokens", "uploadTokenEnvFile", "give the release build", "up_flutter-secret"} {
		if strings.Contains(stdout.String()+stderr.String(), unwanted) {
			t.Errorf("output mentions %q for a plan that asked for no upload token:\n%s\n%s", unwanted, stdout.String(), stderr.String())
		}
	}
	if strings.Contains(stderr.String(), "Note:") {
		t.Errorf("an https server needs no note: %s", stderr.String())
	}
}

func TestSetupApplyNotesALocalServerForMobileApps(t *testing.T) {
	setupTestEnvIsolation(t)
	server := newSetupTestServerWith(t, 0, "http://localhost:18182", []map[string]any{
		approvedProject("p-flutter", "Shop App", "flutter", "tok_flutter-secret", "", "http://localhost:18182"),
		approvedProject("p-android", "Pinboard Android", "android", "tok_android-secret", "", "http://localhost:18182"),
		approvedProject("p-web", "Shop Web", "react", "tok_web-secret", "", "http://localhost:18182"),
	})
	defer server.Close()

	dir := t.TempDir()
	planPath := writePlan(t, dir, map[string]any{"url": server.URL, "projects": []map[string]any{
		{"name": "Shop App", "framework": "flutter", "envFile": "app/.env", "envVar": "TRACEWAY_TOKEN"},
		{"name": "Pinboard Android", "framework": "android", "envFile": "android/local.properties", "envVar": "TRACEWAY_TOKEN"},
		{"name": "Shop Web", "framework": "react", "envFile": "web/.env", "envVar": "VITE_TRACEWAY_TOKEN"},
	}})
	stdout, stderr, err := runCmd(t, testSetupToken+"\n", "setup", "apply", "--plan", planPath, "--token-stdin", "--output", "json")
	if err != nil {
		t.Fatalf("setup apply failed: %v\nstderr: %s", err, stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("the note must stay off stdout, which is JSON: %v\n%s", err, stdout.String())
	}
	if got := readFile(t, dir+"/app/.env"); got != "TRACEWAY_TOKEN=tok_flutter-secret\nTRACEWAY_URL=http://localhost:18182\n" {
		t.Errorf("the written URL stays the server's: %q", got)
	}

	note := stderr.String()
	if strings.Count(note, "Note:") != 1 {
		t.Fatalf("want exactly one note on stderr, got:\n%s", note)
	}
	for _, want := range []string{
		"Shop App (http://localhost:18182), Pinboard Android (http://localhost:18182)",
		"10.0.2.2", "adb reverse tcp:18182 tcp:18182", "iOS Simulator",
		"Android 9+", "network security config", "App Transport Security",
		"the written file is left as it is",
	} {
		if !strings.Contains(note, want) {
			t.Errorf("note should mention %q:\n%s", want, note)
		}
	}
	if strings.Contains(note, "Shop Web (") {
		t.Errorf("a browser project needs no note:\n%s", note)
	}
}

func TestRenderLocalServerNote(t *testing.T) {
	render := func(urls ...string) string {
		var written []mobileServerURL
		for _, u := range urls {
			written = append(written, mobileServerURL{project: "App", url: u})
		}
		var buf bytes.Buffer
		renderLocalServerNote(&buf, written)
		return buf.String()
	}
	if got := render("https://traceway.acme.test", "https://cloud.tracewayapp.com"); got != "" {
		t.Errorf("a public https server needs no note: %q", got)
	}
	if got := render(); got != "" {
		t.Errorf("nothing written, no note: %q", got)
	}

	plain := render("http://traceway.lan:8082")
	if !strings.Contains(plain, "Android 9+") || strings.Contains(plain, "10.0.2.2") {
		t.Errorf("a plain-http LAN server gets the cleartext line only: %q", plain)
	}
	loopbackTLS := render("https://127.0.0.1:8443")
	if !strings.Contains(loopbackTLS, "10.0.2.2") || !strings.Contains(loopbackTLS, "tcp:8443") || strings.Contains(loopbackTLS, "Android 9+") {
		t.Errorf("an https loopback server gets the emulator line only: %q", loopbackTLS)
	}
	for _, u := range []string{"http://localhost", "http://[::1]:9000", "http://0.0.0.0:8082", "http://app.localhost:3000", "http://127.0.0.2:8082"} {
		if got := render(u); !strings.Contains(got, "10.0.2.2") || !strings.Contains(got, "Android 9+") {
			t.Errorf("%s: want both lines, got %q", u, got)
		}
	}
	if got := render("http://localhost"); !strings.Contains(got, "tcp:80 tcp:80") {
		t.Errorf("the default http port is 80: %q", got)
	}
	if got := render("http://localhost:1", "http://localhost:2"); strings.Contains(got, "adb reverse") || strings.Count(got, "Note:") != 1 {
		t.Errorf("two ports: one note without an adb reverse command, got %q", got)
	}
}
