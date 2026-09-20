//go:build telemetry_ch

package telemetry

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func setupTestDB(t *testing.T) {
	t.Helper()

	chServer := os.Getenv("TEST_CLICKHOUSE_SERVER")
	if chServer == "" {
		t.Skip("TEST_CLICKHOUSE_SERVER not set, skipping ClickHouse tests")
	}

	chDatabase := os.Getenv("TEST_CLICKHOUSE_DATABASE")
	if chDatabase == "" {
		chDatabase = "traceway_test"
	}
	chUsername := os.Getenv("TEST_CLICKHOUSE_USERNAME")
	if chUsername == "" {
		chUsername = "default"
	}
	chPassword := os.Getenv("TEST_CLICKHOUSE_PASSWORD")

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{chServer},
		Auth: clickhouse.Auth{
			Database: chDatabase,
			Username: chUsername,
			Password: chPassword,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		MaxOpenConns: 5,
		MaxIdleConns: 5,
	})
	if err != nil {
		t.Fatalf("failed to connect to ClickHouse: %v", err)
	}

	if err := conn.Ping(context.Background()); err != nil {
		t.Fatalf("failed to ping ClickHouse: %v", err)
	}

	chdb.Conn = conn

	pgHost := os.Getenv("TEST_POSTGRES_HOST")
	if pgHost == "" {
		pgHost = "localhost"
	}
	pgPort := os.Getenv("TEST_POSTGRES_PORT")
	if pgPort == "" {
		pgPort = "5432"
	}
	pgDatabase := os.Getenv("TEST_POSTGRES_DATABASE")
	if pgDatabase == "" {
		pgDatabase = "traceway_test"
	}
	pgUsername := os.Getenv("TEST_POSTGRES_USERNAME")
	if pgUsername == "" {
		pgUsername = "traceway"
	}
	pgPassword := os.Getenv("TEST_POSTGRES_PASSWORD")
	pgSSLMode := os.Getenv("TEST_POSTGRES_SSLMODE")
	if pgSSLMode == "" {
		pgSSLMode = "disable"
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		pgHost, pgPort, pgUsername, pgPassword, pgDatabase, pgSSLMode)

	pgDB, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("failed to connect to PostgreSQL: %v", err)
	}

	if err := pgDB.Ping(); err != nil {
		t.Fatalf("failed to ping PostgreSQL: %v", err)
	}

	db.DB = pgDB
	db.Driver = lit.PostgreSQL

	models.Init(db.Driver)

	t.Cleanup(func() {
		ctx := context.Background()
		tables := []string{
			"endpoints_v2", "tasks_v2", "exceptions_v2", "spans_v2", "ai_traces_v2",
			"endpoints", "tasks", "exception_stack_traces", "spans", "ai_traces",
			"metric_points", "session_recordings",
			"archived_exceptions", "slow_endpoints", "fired_notifications",
		}
		for _, table := range tables {
			_ = conn.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE IF EXISTS %s", table))
		}
		pgDB.Close()
	})
}

// legacyExec writes to the tables V2 replaced, which no repository writes any more.
func legacyExec(t *testing.T, query string, args ...any) {
	t.Helper()
	for i, arg := range args {
		switch value := arg.(type) {
		case bool:
			args[i] = uint8(0)
			if value {
				args[i] = uint8(1)
			}
		case *uuid.UUID:
			args[i] = nil
			if value != nil {
				args[i] = *value
			}
		}
	}
	if err := chdb.Conn.Exec(context.Background(), query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func legacyReset(t *testing.T) {
	t.Helper()
	for _, statement := range []string{"TRUNCATE TABLE IF EXISTS endpoints", "TRUNCATE TABLE IF EXISTS tasks", "TRUNCATE TABLE IF EXISTS ai_traces",
		"TRUNCATE TABLE IF EXISTS exception_stack_traces", "TRUNCATE TABLE IF EXISTS spans"} {
		legacyExec(t, statement)
	}
}

func moveOverRowCount(t *testing.T, table string) int64 {
	t.Helper()
	var count uint64
	if err := chdb.Conn.QueryRow(context.Background(), "SELECT count() FROM "+table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return int64(count)
}
