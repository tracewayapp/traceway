//go:build transactional_pg || telemetry_ch

package migrations

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/stub"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// What the operator needs while an index build holds up boot is a line naming
// the migration that is *running*, which golang-migrate emits only in verbose
// mode — along with read-ahead lines naming the next migration, which the
// logger drops. Both halves depend on golang-migrate's own wording, so drive
// the real Up() through the stub driver rather than asserting on transcribed
// format strings: an upgrade that renames either one fails here.
func TestMigrateLoggerReportsEachMigrationAsItRuns(t *testing.T) {
	source, err := iofs.New(fstest.MapFS{
		"m/1_first.up.sql":  &fstest.MapFile{Data: []byte("SELECT 1")},
		"m/2_second.up.sql": &fstest.MapFile{Data: []byte("SELECT 2")},
	}, "m")
	if err != nil {
		t.Fatalf("failed to create migration source: %v", err)
	}

	driver, err := stub.WithInstance(nil, &stub.Config{})
	if err != nil {
		t.Fatalf("failed to create stub driver: %v", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "stub", driver)
	if err != nil {
		t.Fatalf("failed to create migrate instance: %v", err)
	}

	var buf bytes.Buffer
	t.Cleanup(captureLog(&buf))

	m.Log = migrateLogger{store: "postgres"}
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}
	got := buf.String()

	for _, want := range []string{
		"migrations: postgres Read and execute 1/u first",
		"migrations: postgres Read and execute 2/u second",
		"Finished 1/u first",
		"Finished 2/u second",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in the migration log, got:\n%s", want, got)
		}
	}

	for _, unwanted := range migrateReadAheadPrefixes {
		if strings.Contains(got, unwanted) {
			t.Fatalf("expected read-ahead line %q to be dropped, got:\n%s", unwanted, got)
		}
	}
}
