package migrations

import (
	"bytes"
	"database/sql"
	"log"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// captureLog redirects the standard logger into buf and returns the restore
// func. Restoring matters beyond tidiness: log.SetOutput(nil) leaves the logger
// with a nil writer, so the next line logged anywhere in the binary panics.
func captureLog(buf *bytes.Buffer) func() {
	out, flags := log.Writer(), log.Flags()
	log.SetOutput(buf)
	log.SetFlags(0)
	return func() {
		log.SetOutput(out)
		log.SetFlags(flags)
	}
}

// Two properties carry the whole point of the migration logging, and neither is
// visible to a functional test: the announcement has to precede the work, since
// naming a migration only once it finishes is the silence this replaced; and an
// already-applied migration has to stay quiet, or every restart reprints the
// full history.
func TestRunMigrationsOnAnnouncesBeforeApplyingAndOnlyOnce(t *testing.T) {
	var buf bytes.Buffer
	t.Cleanup(captureLog(&buf))

	target, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	defer target.Close()
	target.SetMaxOpenConns(1)

	if err := runMigrationsOn(target, migrationsSqliteFS, "sqlite", "schema_migrations", sqliteTrackingDDL); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 2 || len(lines)%2 != 0 {
		t.Fatalf("want paired announce/complete lines, got %d:\n%s", len(lines), buf.String())
	}
	for i := 0; i < len(lines); i += 2 {
		announcement, completion := lines[i], lines[i+1]
		version, ok := strings.CutSuffix(strings.TrimPrefix(announcement, "[tracewaybackend] migration "), ": applying")
		if !ok {
			t.Fatalf("line %d should announce a migration, got %q", i, announcement)
		}
		if !strings.Contains(completion, "migration "+version+": applied in ") {
			t.Errorf("%q was announced but the next line is not its completion: %q", version, completion)
		}
	}

	buf.Reset()
	if err := runMigrationsOn(target, migrationsSqliteFS, "sqlite", "schema_migrations", sqliteTrackingDDL); err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("re-running applied migrations logged %q, want silence", buf.String())
	}
}
