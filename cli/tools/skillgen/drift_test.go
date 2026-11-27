package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// repoRoot is the repository root as seen from this package's directory, where go test runs.
const repoRoot = "../../.."

// Directories whose every file comes from the generator, so a file the generator no longer writes is stale.
var generatedOnlyDirs = []string{
	"skills/traceway",
	"skills/traceway-learn",
	"plugins/tw-write/skills/traceway-write",
}

// TestCommittedSkillsMatchTheGenerator fails when a generated skill file differs from what the knowledge chunks
// and templates produce, including the two copies in the hand-written skills/traceway-setup directory.
func TestCommittedSkillsMatchTheGenerator(t *testing.T) {
	if _, err := os.Stat(filepath.Join(repoRoot, "skills", "traceway-setup", "SKILL.md")); err != nil {
		t.Skip("not run inside the traceway repository")
	}

	tmp := t.TempDir()
	generate(tmp)

	var generated []string
	err := filepath.WalkDir(tmp, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(tmp, path)
		if err != nil {
			return err
		}
		generated = append(generated, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(generated) == 0 {
		t.Fatal("the generator wrote no files")
	}

	for _, rel := range generated {
		want, err := os.ReadFile(filepath.Join(tmp, rel))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Errorf("%s is missing; run just gen-skills in cli/", rel)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from the generator's output; edit cli/pkg/mcpserver/knowledge or cli/tools/skillgen and run just gen-skills in cli/", rel)
		}
	}

	for _, dir := range generatedOnlyDirs {
		entries, err := os.ReadDir(filepath.Join(repoRoot, dir))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			rel := dir + "/" + entry.Name()
			if !entry.IsDir() && !slices.Contains(generated, rel) {
				t.Errorf("%s is not written by the generator; delete it or add it to cli/tools/skillgen", rel)
			}
		}
	}
}
