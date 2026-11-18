//go:build transactional_pg || telemetry_ch

package migrations

import (
	"fmt"
	"strings"

	"github.com/tracewayapp/traceway/backend/v2/app/config"
)

// Verbose mode is what names the running migration; its read-ahead lines name the next one and are dropped.
var migrateReadAheadPrefixes = []string{"Start buffering", "Scheduled"}

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
