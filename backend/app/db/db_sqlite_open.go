//go:build !transactional_pg || (!telemetry_ch && !telemetry_duckdb && !telemetry_firebolt)

package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Pragmas go on the DSN (modernc.org/sqlite supports `_pragma=name(value)`)
// rather than via db.Exec because Exec only configures the first pooled
// connection — once SetMaxOpenConns goes above 1 new connections would
// otherwise boot without the pragmas applied.
func openSQLite(path string, telemetry bool) (*sql.DB, error) {
	var dsn string
	if path == ":memory:" {
		dsn = path
	} else {
		params := []string{
			"_pragma=journal_mode(WAL)",
			"_pragma=busy_timeout(5000)",
		}
		if telemetry {
			params = append(params,
				"_pragma=synchronous(NORMAL)",
				"_pragma=cache_size(-524288)",
				"_pragma=temp_store(MEMORY)",
				"_pragma=mmap_size(1073741824)",
				"_pragma=wal_autocheckpoint(50000)",
				"_pragma=auto_vacuum(incremental)",
			)
		} else {
			params = append(params, "_pragma=foreign_keys(ON)")
		}
		dsn = "file:" + path + "?" + strings.Join(params, "&")
	}

	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite at %s: %w", path, err)
	}
	if err := d.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite at %s: %w", path, err)
	}

	if path == ":memory:" {
		if !telemetry {
			if _, err := d.Exec("PRAGMA foreign_keys = ON"); err != nil {
				return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
			}
		}
		if _, err := d.Exec("PRAGMA journal_mode = WAL"); err != nil {
			return nil, fmt.Errorf("failed to set WAL mode: %w", err)
		}
	}

	if path == ":memory:" {
		d.SetMaxOpenConns(1)
	} else if telemetry {
		// WAL allows concurrent readers; SQLite still serializes writes at the
		// file level and busy_timeout absorbs short contention windows.
		d.SetMaxOpenConns(4)
	} else {
		d.SetMaxOpenConns(1)
	}
	return d, nil
}
