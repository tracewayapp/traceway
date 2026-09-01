//go:build transactional_pg || telemetry_ch

package migrations

import (
	"fmt"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/config"
)

// golang-migrate announces the migration it is about to run only in verbose
// mode, and that is the line an operator needs while a slow index build holds
// up boot. Verbose also emits the read-ahead goroutine's buffering lines, which
// name the *next* migration while the current one is still running and would
// read as progress that is not happening, so they are dropped here.
var migrateReadAheadPrefixes = []string{"Start buffering", "Scheduled"}

// migrateLogger gives the ClickHouse and Postgres paths the per-migration
// progress that runMigrationsOn logs itself on the embedded stores: one line
// naming each migration as it starts, one more with its elapsed time.
type migrateLogger struct {
	store string
}

func (l migrateLogger) Printf(format string, v ...any) {
	msg := strings.TrimRight(fmt.Sprintf(format, v...), "\n")
	for _, prefix := range migrateReadAheadPrefixes {
		if strings.HasPrefix(msg, prefix) {
			return
		}
	}
	config.Logf("migrations: %s %s", l.store, msg)
}

func (migrateLogger) Verbose() bool { return true }
