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
