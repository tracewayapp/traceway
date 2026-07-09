package transactional

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The pg and sqlite backends are intentionally identical, dialect-neutral
// implementations (lit :name queries rendered per db.Driver); the build tag
// only selects which copy is compiled in. This test fails when a change lands
// in one copy but not the other, so the two backends cannot silently drift
// until a repository genuinely needs dialect-specific SQL — at which point the
// diverging file should be added to the exceptions list below with a comment
// explaining the divergence.
func TestPgAndSqliteBackendsStayInSync(t *testing.T) {
	exceptions := map[string]bool{}

	pgFiles := listGoFiles(t, "pg")
	sqliteFiles := listGoFiles(t, "sqlite")

	for name := range pgFiles {
		if !sqliteFiles[name] {
			t.Errorf("pg/%s has no sqlite/ counterpart", name)
		}
	}
	for name := range sqliteFiles {
		if !pgFiles[name] {
			t.Errorf("sqlite/%s has no pg/ counterpart", name)
		}
	}

	for name := range pgFiles {
		if !sqliteFiles[name] || exceptions[name] {
			continue
		}
		pgBody := normalizedBody(t, filepath.Join("pg", name))
		sqliteBody := normalizedBody(t, filepath.Join("sqlite", name))
		if pgBody != sqliteBody {
			t.Errorf("pg/%s and sqlite/%s differ beyond the build tag and package clause; apply the change to both copies or add the file to the exceptions list", name, name)
		}
	}
}

func listGoFiles(t *testing.T, dir string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	files := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			files[e.Name()] = true
		}
	}
	return files
}

func normalizedBody(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var kept []string
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "//go:build ") || strings.HasPrefix(line, "package ") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
