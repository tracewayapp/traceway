package suite_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
)

// TestFixtureHistory checks every fixture's history.yaml: each overlay
// directory holds at least one file (git does not keep empty directories, so
// an empty overlay exists only in the checkout that made it), and every
// absent path is a file of the fixture.
func TestFixtureHistory(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(env.EvalsRoot(), "testdata", "repos", "*", "history.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		repo := filepath.Dir(f)
		t.Run(filepath.Base(repo), func(t *testing.T) {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			var hist struct {
				Commits []struct {
					Dir    string   `yaml:"dir"`
					Absent []string `yaml:"absent"`
				} `yaml:"commits"`
			}
			if err := yaml.Unmarshal(raw, &hist); err != nil {
				t.Fatal(err)
			}
			for i, c := range hist.Commits {
				if c.Dir != "" && !hasFile(filepath.Join(repo, c.Dir)) {
					t.Errorf("commit %d: overlay %s has no files; drop its dir: key", i+1, c.Dir)
				}
				for _, a := range c.Absent {
					if !exists(filepath.Join(repo, filepath.FromSlash(a))) {
						t.Errorf("commit %d: absent path %s is not in the fixture", i+1, a)
					}
				}
			}
		})
	}
}

func hasFile(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}
