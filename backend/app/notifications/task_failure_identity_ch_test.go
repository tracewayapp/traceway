//go:build telemetry_ch

package notifications

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
)

func TestTaskFailuresKeepTraceIdentityClickHouse(t *testing.T) {
	server := os.Getenv("TEST_CLICKHOUSE_SERVER")
	if server == "" {
		t.Skip("TEST_CLICKHOUSE_SERVER not set")
	}
	ctx := context.Background()
	auth := ch.Auth{Database: "default", Username: os.Getenv("TEST_CLICKHOUSE_USERNAME"), Password: os.Getenv("TEST_CLICKHOUSE_PASSWORD")}
	if auth.Username == "" {
		auth.Username = "default"
	}
	admin, err := ch.Open(&ch.Options{Addr: []string{server}, Auth: auth})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	auth.Database = "task_identity_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec(ctx, "CREATE DATABASE "+auth.Database); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP DATABASE "+auth.Database)
	conn, err := ch.Open(&ch.Options{Addr: []string{server}, Auth: auth})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	previous := chdb.Conn
	chdb.Conn = conn
	defer func() { chdb.Conn = previous }()
	for _, query := range []string{
		"CREATE TABLE tasks_v2 (project_id UUID, trace_id String, span_id String, task_name String, recorded_at DateTime64(6)) ENGINE=Memory",
		"CREATE TABLE exceptions_v2 (project_id UUID, trace_id String, span_id String, trace_type String, recorded_at DateTime64(6)) ENGINE=Memory",
	} {
		if err := conn.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	project, at := uuid.New(), time.Now().UTC().Truncate(time.Second)
	for _, trace := range []string{"trace-a", "trace-b"} {
		if err := conn.Exec(ctx, "INSERT INTO tasks_v2 VALUES (?, ?, 'reused', 'task', ?)", project, trace, at); err != nil {
			t.Fatal(err)
		}
		if err := conn.Exec(ctx, "INSERT INTO exceptions_v2 VALUES (?, ?, 'reused', 'task', ?)", project, trace, at); err != nil {
			t.Fatal(err)
		}
	}
	for _, named := range []bool{true, false} {
		got, err := countFailedTaskExecutions(ctx, project, "task", named, at.Add(-time.Hour), at.Add(time.Hour))
		if err != nil || got != 2 {
			t.Fatalf("named=%v: got %d, %v; want 2", named, got, err)
		}
	}
	if err := conn.Exec(ctx, "TRUNCATE TABLE exceptions_v2"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Exec(ctx, "TRUNCATE TABLE tasks_v2"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Exec(ctx, "INSERT INTO tasks_v2 VALUES (?, 'trace-a', 'reused', 'task', ?), (?, 'trace-b', 'reused', 'other', ?)", project, at, project, at); err != nil {
		t.Fatal(err)
	}
	if err := conn.Exec(ctx, "INSERT INTO exceptions_v2 VALUES (?, 'trace-b', 'reused', 'task', ?)", project, at); err != nil {
		t.Fatal(err)
	}
	got, err := countFailedTaskExecutions(ctx, project, "task", true, at.Add(-time.Hour), at.Add(time.Hour))
	if err != nil || got != 0 {
		t.Fatalf("successful task inherited another trace's failure: %d, %v", got, err)
	}
}
