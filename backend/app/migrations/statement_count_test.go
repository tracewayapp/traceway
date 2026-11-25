package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every pg and ch migration holds exactly one statement: the ClickHouse driver runs a file as one statement, and pg
// follows the same rule.
func TestPgAndChMigrationsHoldOneStatement(t *testing.T) {
	for _, dir := range []string{"pg", "ch"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no migrations in %s: %v", dir, err)
		}
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var lines []string
			for _, line := range strings.Split(string(data), "\n") {
				if !strings.HasPrefix(strings.TrimSpace(line), "--") {
					lines = append(lines, line)
				}
			}
			count := 0
			for _, statement := range splitStatements(strings.Join(lines, "\n")) {
				if strings.TrimSpace(statement) != "" {
					count++
				}
			}
			if count != 1 {
				t.Errorf("%s holds %d statements", file, count)
			}
		}
	}
}
