package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/tracewayapp/traceway/backend/app/config"

	_ "github.com/lib/pq"
	"github.com/tracewayapp/lit/v2"
)

var DB *sql.DB          // PostgreSQL-replacement: relational/config data (transactional)
var TelemetryDB *sql.DB // ClickHouse-replacement: append-only telemetry data (non-transactional)
var Driver lit.Driver = lit.PostgreSQL

// TelemetryDriver is the lit dialect for TelemetryDB. Every embedded/engine
// telemetry store (SQLite, DuckDB, Firebolt) takes `?` placeholders, so this
// stays lit.SQLite even when the main DB is Postgres; ClickHouse builds have
// no TelemetryDB and never use it.
var TelemetryDriver lit.Driver = lit.SQLite

var telemetryIsDuckDB bool
var telemetryIsFirebolt bool

func IsSQLite() bool {
	return Driver == lit.SQLite
}

// Init opens the main (transactional) database selected by the transactional_* build axis, then
// the telemetry database selected by the telemetry_* build axis.
func Init() error {
	if err := initMainDB(); err != nil {
		return err
	}
	return initTelemetryDB()
}

func IsDuckDBTelemetry() bool {
	return telemetryIsDuckDB
}

func initPostgres() error {
	cfg := config.Config

	host := cfg.PostgresHost
	port := cfg.PostgresPort
	database := cfg.PostgresDatabase
	username := cfg.PostgresUsername
	password := cfg.PostgresPassword
	sslMode := cfg.PostgresSSLMode

	if sslMode == "" {
		sslMode = "disable"
	}
	if port == "" {
		port = "5432"
	}

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, username, password, database, sslMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open postgres connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping postgres: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)

	DB = db
	Driver = lit.PostgreSQL

	return nil
}

func GetDB() *sql.DB {
	return DB
}

const TransactionContextKey = "dbTx"

func GetTx(ctx context.Context) *sql.Tx {
	if tx, ok := ctx.Value(TransactionContextKey).(*sql.Tx); ok {
		return tx
	}
	return nil
}

type ctxKey struct{}

func ContextWithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, ctxKey{}, tx)
}

func QueryerFromContext(ctx context.Context) interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
} {
	if tx, ok := ctx.Value(ctxKey{}).(*sql.Tx); ok && tx != nil {
		return tx
	}
	return DB
}

func ExecuteTransaction[T any](f func(tx *sql.Tx) (T, error)) (T, error) {
	tx, err := DB.Begin()

	if err != nil {
		var zero T
		return zero, err
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	result, err := f(tx)

	if err != nil {
		tx.Rollback()
		var zero T
		return zero, err
	}

	if err := tx.Commit(); err != nil {
		var zero T
		return zero, err
	}

	return result, nil
}
