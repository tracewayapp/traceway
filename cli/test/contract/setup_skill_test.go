package contract

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the actual credential writer copied out of the setup skill. The fake
// curl returns an approved draft so the Cloud case never needs external access.
func TestSetupSkillWritesRuntimeAndUploaderDestinations(t *testing.T) {
	for _, tool := range []string{"sh", "jq", "awk"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s unavailable", tool)
		}
	}
	doc, err := os.ReadFile("../../../skills/traceway-setup/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	_, body, found := strings.Cut(string(doc), "```sh\n#!/bin/sh\n")
	script, _, closed := strings.Cut(body, "\n```")
	if !found || !closed {
		t.Fatal("setup skill is missing its shell credential writer")
	}

	dir := t.TempDir()
	write := func(name, value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0700); err != nil {
			t.Fatal(err)
		}
	}
	write("apply.sh", script)
	write("curl", "#!/bin/sh\ncat approved.json\n")
	write("approved.json", `{"status":"approved","projects":[{"name":"app","status":"existing","token":"runtime-fixture","sourceMapToken":"upload-fixture"},{"name":"legacy","status":"existing","token":"legacy-fixture"}]}`)
	write("plan.json", `{"projects":[{"name":"app","framework":"react","envFile":".env","envVar":"VITE_TRACEWAY_TOKEN","uploadTokenEnvFile":".env.upload"},{"name":"legacy","framework":"react-native","envFile":".env.native","envVar":"DSN","envFormat":"connectionString"}]}`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TRACEWAY_SETUP_TOKEN", "setup-fixture")

	cloud := "https://cloud.tracewayapp.com"
	for i, base := range []string{cloud, "http://api/gateway", cloud} {
		t.Setenv("TRACEWAY_URL", base)
		cmd := exec.Command("sh", "apply.sh", "plan.json")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("apply %d failed: %v: %s", i, err, out)
		}
		if strings.Contains(string(out), "-fixture") {
			t.Fatal("the credential writer printed a token")
		}
		runtime, upload := "VITE_TRACEWAY_TOKEN=runtime-fixture\n", "TRACEWAY_SOURCEMAP_TOKEN=upload-fixture\n"
		if i > 0 {
			runtime += "VITE_TRACEWAY_URL=" + base + "\n"
			upload += "TRACEWAY_URL=" + base + "\n"
		}
		for file, want := range map[string]string{
			".env": runtime, ".env.upload": upload, ".env.native": "DSN=legacy-fixture@" + base + "/api/report\n",
		} {
			got, err := os.ReadFile(filepath.Join(dir, file))
			if err != nil || string(got) != want {
				t.Errorf("apply %d, %s: got %q, want %q, err %v", i, file, got, want, err)
			}
		}
	}
}
