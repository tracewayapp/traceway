package migrations

import (
	"embed"
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// Other tables carry a _v2 suffix of their own (sessions), so the five are named.
var v2TelemetryMigration = regexp.MustCompile(`_create_(v2_tables|(spans|endpoints|tasks|ai_traces|exceptions)_v2)\.up\.sql$`)

// V2 lives next to the tables it replaces. Its migrations only create, so an upgrade rewrites nothing, a rollback
// finds the old tables as they were, and the move-over script has an untouched source to read.
func assertV2MigrationsOnlyCreate(t *testing.T, source embed.FS, dir string) {
	t.Helper()
	found := 0
	err := fs.WalkDir(source, dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !v2TelemetryMigration.MatchString(path) {
			return err
		}
		found++
		data, err := source.ReadFile(path)
		if err != nil {
			return err
		}
		for _, statement := range splitStatements(string(data)) {
			upper := strings.ToUpper(strings.Join(strings.Fields(statement), " "))
			if upper == "" {
				continue
			}
			if !strings.HasPrefix(upper, "CREATE TABLE IF NOT EXISTS ") && !strings.HasPrefix(upper, "CREATE INDEX IF NOT EXISTS ") {
				t.Errorf("%s: a V2 migration only creates tables and indexes, got: %.80s", path, upper)
			}
			target := strings.Fields(strings.TrimPrefix(strings.TrimPrefix(upper, "CREATE TABLE IF NOT EXISTS "), "CREATE INDEX IF NOT EXISTS "))
			if strings.HasPrefix(upper, "CREATE INDEX") && len(target) >= 3 {
				target = target[2:]
			}
			if len(target) == 0 || !strings.Contains(strings.SplitN(target[0], "(", 2)[0], "_V2") {
				t.Errorf("%s: a V2 migration must not touch the tables V2 replaces: %.80s", path, upper)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == 0 {
		t.Fatalf("no V2 migrations under %s", dir)
	}
}
