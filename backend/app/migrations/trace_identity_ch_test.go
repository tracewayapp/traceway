//go:build telemetry_ch

package migrations

import (
	"database/sql"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"testing"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
)

func traceIdentityDatabase(t *testing.T) (*sql.DB, fs.FS, string, string, func(fs.FS) error) {
	server := os.Getenv("TEST_CLICKHOUSE_SERVER")
	if server == "" {
		t.Skip("TEST_CLICKHOUSE_SERVER not set, skipping ClickHouse migration tests")
	}
	username := os.Getenv("TEST_CLICKHOUSE_USERNAME")
	if username == "" {
		username = "default"
	}
	params := url.Values{"username": {username}, "password": {os.Getenv("TEST_CLICKHOUSE_PASSWORD")}}
	dsn := "clickhouse://" + server + "?"
	admin, err := sql.Open("clickhouse", dsn+params.Encode())
	if err != nil {
		t.Fatal(err)
	}
	name := "trace_identity_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE " + name + " SYNC"); err != nil {
			t.Errorf("clean up migration test database: %v", err)
		}
		admin.Close()
	})
	params.Set("database", name)
	targetDSN := dsn + params.Encode()
	target, err := sql.Open("clickhouse", targetDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { target.Close() })
	return target, migrationsChFS, "ch", "0091", func(source fs.FS) error {
		return runMigrationsClickhouseFrom(targetDSN, source)
	}
}
