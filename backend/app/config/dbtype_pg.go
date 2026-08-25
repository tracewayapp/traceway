//go:build transactional_pg

package config

// defaultDBType is what DB_TYPE means when it is not set.
//
// In the transactional_pg build an empty value already selects PostgreSQL --
// db_transactional_pg.go connects unconditionally and runMainDBMigrations
// treats anything other than "sqlite" as PostgreSQL. Returning "" keeps that
// build byte-for-byte unchanged; only the SQLite builds gain a default.
func defaultDBType() string { return "" }
