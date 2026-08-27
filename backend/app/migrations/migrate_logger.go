//go:build telemetry_ch || transactional_pg

package migrations

import (
	"fmt"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/config"
)

// migrateLogger is the backend name golang-migrate's output is labelled with.
type migrateLogger string

// migrate's buffering internals ride the same verbose stream as the pre-run
// announcement, and only the announcement is worth a boot log line. Skipping by
// prefix rather than keeping by one leaves migrate's error lines, and anything a
// later version adds, to pass through.
var migrateInternalPrefixes = []string{"Start buffering ", "Scheduled ", "Closing source and database"}

func (l migrateLogger) Printf(format string, v ...any) {
	message := fmt.Sprintf(format, v...)
	for _, prefix := range migrateInternalPrefixes {
		if strings.HasPrefix(message, prefix) {
			return
		}
	}
	config.Logf("migration %s: %s", string(l), message)
}

// Verbose is on because migrate names a migration before running it only on its
// verbose stream. Without it a stall here is silent and the newest line names the
// migration that already finished -- the opposite of what runMigrationsOn gives
// the embedded backends.
func (migrateLogger) Verbose() bool { return true }
