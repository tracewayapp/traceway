//go:build telemetry_ch || transactional_pg

package migrations

import (
	"bytes"
	"strings"
	"testing"
)

// The verbose stream migrateLogger subscribes to carries migrate's buffering
// internals alongside the pre-run announcement, so Printf filters it. Filtering
// by a skip list rather than a keep list is what leaves migrate's error lines --
// which arrive through the same Printf -- visible, so pin that here: swapping the
// two would silently swallow the reason a migration failed.
func TestMigrateLoggerKeepsAnnouncementsAndErrors(t *testing.T) {
	var buf bytes.Buffer
	t.Cleanup(captureLog(&buf))

	l := migrateLogger("postgres")
	l.Printf("Start buffering %v\n", "34/u add_thing")
	l.Printf("Scheduled %v\n", "34/u add_thing")
	l.Printf("Read and execute %v\n", "34/u add_thing")
	l.Printf("Finished %v (read %v, ran %v)\n", "34/u add_thing", "1ms", "9.4s")
	l.Printf("Closing source and database\n")
	l.Printf("error: %v", "connection refused")

	got := buf.String()
	for _, dropped := range []string{"Start buffering", "Scheduled", "Closing source"} {
		if strings.Contains(got, dropped) {
			t.Errorf("%q should have been filtered out, got:\n%s", dropped, got)
		}
	}
	for _, kept := range []string{
		"migration postgres: Read and execute 34/u add_thing",
		"migration postgres: Finished 34/u add_thing (read 1ms, ran 9.4s)",
		"migration postgres: error: connection refused",
	} {
		if !strings.Contains(got, kept) {
			t.Errorf("%q should have survived, got:\n%s", kept, got)
		}
	}
	// migrate terminates its formats with a newline and log adds its own only
	// when one is absent, which is why Printf does not trim.
	if strings.Contains(got, "\n\n") {
		t.Errorf("blank line in output: %q", got)
	}
}
