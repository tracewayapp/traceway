package contract

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupSkillWorkspace holds the credential writer copied out of the setup skill, the plan, and a fake curl that
// returns the approved draft, so no case needs external access.
func setupSkillWorkspace(t *testing.T, approved, plan string) string {
	t.Helper()
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
	for name, value := range map[string]string{
		"apply.sh": script, "curl": "#!/bin/sh\ncat approved.json\n", "approved.json": approved, "plan.json": plan,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TRACEWAY_SETUP_TOKEN", "setup-fixture")
	return dir
}

func applySetupSkill(t *testing.T, dir, base string) {
	t.Helper()
	t.Setenv("TRACEWAY_URL", base)
	cmd := exec.Command("sh", "apply.sh", "plan.json")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("apply against %s failed: %v: %s", base, err, out)
	}
	if strings.Contains(string(out), "-fixture") {
		t.Fatal("the credential writer printed a token")
	}
}

func TestSetupSkillWritesRuntimeAndUploaderDestinations(t *testing.T) {
	dir := setupSkillWorkspace(t,
		`{"status":"approved","projects":[{"name":"app","status":"existing","token":"runtime-fixture","sourceMapToken":"upload-fixture"},{"name":"legacy","status":"existing","token":"legacy-fixture"}]}`,
		`{"projects":[{"name":"app","framework":"react","envFile":".env","envVar":"VITE_TRACEWAY_TOKEN","uploadTokenEnvFile":".env.upload"},{"name":"legacy","framework":"react-native","envFile":".env.native","envVar":"DSN","envFormat":"connectionString"}]}`)

	cloud := "https://cloud.tracewayapp.com"
	for i, base := range []string{cloud, "http://api/gateway", cloud} {
		applySetupSkill(t, dir, base)
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

func TestSetupSkillWritesAReactNativeConnectionString(t *testing.T) {
	cloud := "https://cloud.tracewayapp.com"
	for _, base := range []string{cloud, "https://traceway.acme.test"} {
		dir := setupSkillWorkspace(t,
			`{"status":"approved","projects":[{"name":"app","status":"created","token":"runtime-fixture","backendUrl":"`+base+`"}]}`,
			`{"projects":[{"name":"app","framework":"react-native","envFile":".env","envVar":"EXPO_PUBLIC_TRACEWAY_CONNECTION_STRING"}]}`)
		applySetupSkill(t, dir, base)
		want := "EXPO_PUBLIC_TRACEWAY_CONNECTION_STRING=runtime-fixture@" + base + "/api/report\n"
		if got, err := os.ReadFile(filepath.Join(dir, ".env")); err != nil || string(got) != want {
			t.Errorf("%s: got %q, want %q, err %v", base, got, want, err)
		}
	}
}

// A server without APP_BASE_URL names itself scheme://host from the request: no path prefix, http behind a TLS proxy.
func TestSetupSkillKeepsTheWorkingURLOfTheSameHost(t *testing.T) {
	for _, tc := range []struct{ reported, base, want string }{
		{"https://corp.example", "https://corp.example/traceway", "https://corp.example/traceway"},
		{"http://corp.example", "https://corp.example", "https://corp.example"},
		{"https://corp.example", "http://corp.example", "https://corp.example"},
		{"https://traceway.acme.test", "http://10.0.0.5:8082", "https://traceway.acme.test"},
	} {
		dir := setupSkillWorkspace(t,
			`{"status":"approved","projects":[{"name":"app","status":"created","token":"runtime-fixture","sourceMapToken":"upload-fixture","backendUrl":"`+tc.reported+`"}]}`,
			`{"projects":[{"name":"app","framework":"react","envFile":".env","envVar":"VITE_TRACEWAY_TOKEN","uploadTokenEnvFile":".env.upload"}]}`)
		applySetupSkill(t, dir, tc.base)
		for file, want := range map[string]string{
			".env":        "VITE_TRACEWAY_TOKEN=runtime-fixture\nVITE_TRACEWAY_URL=" + tc.want + "\n",
			".env.upload": "TRACEWAY_SOURCEMAP_TOKEN=upload-fixture\nTRACEWAY_URL=" + tc.want + "\n",
		} {
			if got, err := os.ReadFile(filepath.Join(dir, file)); err != nil || string(got) != want {
				t.Errorf("server %s, caller %s, %s: got %q, want %q, err %v", tc.reported, tc.base, file, got, want, err)
			}
		}
	}
}
