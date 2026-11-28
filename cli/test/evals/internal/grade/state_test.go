package grade

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
)

func TestCheckFilesSkipsGeneratedDirs(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("app/main.py", "app = FastAPI()\n")
	for _, dir := range []string{".venv/lib/otel", ".next/server", "dist/assets", "node_modules/sdk", ".twevals-venv/lib"} {
		write(dir+"/resource.py", "SERVICE_NAME = 'service.name'\n")
	}
	write(".twevals-run.log", "service.name\n")
	spec := catalog.StateAssert{Path: "**/*", Contains: []string{`service\.name`}}
	if err := checkFiles(root, spec); err == nil {
		t.Fatal("service.name in installed packages and build output counted as the agent's")
	}
	write("app/telemetry.py", "resource = Resource.create({'service.name': 'inventory-api'})\n")
	if err := checkFiles(root, spec); err != nil {
		t.Fatalf("the agent's own file was not found: %v", err)
	}
}

func TestExpandBraces(t *testing.T) {
	got := expandBraces("{app/build.gradle.kts,.github/workflows/*}")
	if len(got) != 2 || got[0] != "app/build.gradle.kts" || got[1] != ".github/workflows/*" {
		t.Fatalf("expandBraces = %q", got)
	}
	got = expandBraces("**/*.{kt,java}")
	if len(got) != 2 || got[0] != "**/*.kt" || got[1] != "**/*.java" {
		t.Fatalf("expandBraces = %q", got)
	}
	if got := expandBraces("**/*.go"); len(got) != 1 || got[0] != "**/*.go" {
		t.Fatalf("expandBraces = %q", got)
	}
}

func TestCheckFilesSameFile(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{"a.swift": "@main struct App {}\n", "b.swift": "Traceway.initialize(token: t)\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	spec := catalog.StateAssert{Path: "*.swift", Contains: []string{"@main", `Traceway\.initialize\(`}}
	if err := checkFiles(root, spec); err != nil {
		t.Fatalf("across files: %v", err)
	}
	spec.SameFile = true
	if err := checkFiles(root, spec); err == nil {
		t.Fatal("same_file matched across two files")
	}
}
