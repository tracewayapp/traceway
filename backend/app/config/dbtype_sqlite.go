//go:build !transactional_pg

package config

// defaultDBType is what DB_TYPE means when it is not set.
//
// Without the transactional_pg tag, SQLite is the only supported main
// database: runMainDBMigrations ignores the value entirely and applies
// SQLite-dialect migrations unconditionally, so any other value could only
// ever produce a broken database. Defaulting here rather than at each of the
// three `DBType != "sqlite"` call sites (db_transactional_sqlite.go,
// db_telemetry_sqlite.go, db_telemetry_duckdb.go) keeps one definition, and
// is what lets a fresh clone, an IDE run, CI, Docker and a Nix dev shell all
// start without a preconfigured environment.
func defaultDBType() string { return "sqlite" }
