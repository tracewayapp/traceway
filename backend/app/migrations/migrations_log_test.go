package migrations

import (
	"bytes"
	"database/sql"
	"log"
	"strings"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"
)

// openMemorySQLite returns a fresh in-memory database pinned to one connection:
// every new connection to ":memory:" opens a separate, empty database, so a
// second connection would see none of the migrated schema.
func openMemorySQLite(t *testing.T) *sql.DB {
	t.Helper()
	target, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { target.Close() })
	target.SetMaxOpenConns(1)
	return target
}

// captureLog redirects the standard logger into buf with its flags cleared, so
// no timestamp precedes a line and assertions can match from the line start.
// The returned func restores both.
func captureLog(buf *bytes.Buffer) func() {
	out, flags := log.Writer(), log.Flags()
	log.SetOutput(buf)
	log.SetFlags(0)
	return func() {
		log.SetOutput(out)
		log.SetFlags(flags)
	}
}

// assertLogLines checks buf holds exactly these lines in order. Each is matched
// as a prefix so the elapsed-time suffix does not pin a duration.
func assertLogLines(t *testing.T, buf *bytes.Buffer, want ...string) {
	t.Helper()
	got := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(got) != len(want) {
		t.Fatalf("want %d log lines, got %d:\n%s", len(want), len(got), buf.String())
	}
	for i := range want {
		if !strings.HasPrefix(got[i], want[i]) {
			t.Errorf("line %d: want prefix %q, got %q", i, want[i], got[i])
		}
	}
}

// The announcement has to land before the statements run, since naming a
// migration only once it finishes is the silence this replaced. A failing
// migration is what makes that visible: it leaves the announcement with no
// completion. And an already-applied migration has to stay quiet, or every
// restart reprints the full history.
func TestRunMigrationsOnAnnouncesEachMigrationBeforeRunningIt(t *testing.T) {
	var buf bytes.Buffer
	t.Cleanup(captureLog(&buf))
	target := openMemorySQLite(t)

	fsys := fstest.MapFS{
		"m/0001_first.up.sql":  {Data: []byte("CREATE TABLE first (id INTEGER)")},
		"m/0002_second.up.sql": {Data: []byte("CREATE TABLE second (")},
	}
	if err := runMigrationsOn(target, fsys, "m", "schema_migrations", sqliteTrackingDDL); err == nil {
		t.Fatal("expected the broken migration to fail")
	}
	assertLogLines(t, &buf,
		"[tracewaybackend] migrations: applying m/0001_first",
		"[tracewaybackend] migrations: applied m/0001_first in ",
		"[tracewaybackend] migrations: applying m/0002_second",
	)

	fsys["m/0002_second.up.sql"] = &fstest.MapFile{Data: []byte("CREATE TABLE second (id INTEGER)")}
	buf.Reset()
	if err := runMigrationsOn(target, fsys, "m", "schema_migrations", sqliteTrackingDDL); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}
	assertLogLines(t, &buf,
		"[tracewaybackend] migrations: applying m/0002_second",
		"[tracewaybackend] migrations: applied m/0002_second in ",
	)

	buf.Reset()
	if err := runMigrationsOn(target, fsys, "m", "schema_migrations", sqliteTrackingDDL); err != nil {
		t.Fatalf("third run failed: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("re-running applied migrations logged %q, want silence", buf.String())
	}
}
