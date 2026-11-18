//go:build telemetry_ch

package clickhouse

import (
	"context"
	"os"
	"strings"
	"testing"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

func scratchDatabase(t *testing.T) (context.Context, driver.Conn) {
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
	database := "span_scratch_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec(ctx, "CREATE DATABASE "+database); err != nil {
		t.Fatal(err)
	}
	auth.Database = database
	conn, err := ch.Open(&ch.Options{Addr: []string{server}, Auth: auth})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn.Close()
		admin.Exec(ctx, "DROP DATABASE "+database)
		admin.Close()
	})
	return ctx, conn
}

func createSpansTable(t *testing.T, ctx context.Context, conn driver.Conn) {
	for _, name := range []string{"0085_create_spans_v2.up.sql", "0090_add_span_version.up.sql"} {
		migration, err := os.ReadFile("../../../migrations/ch/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := conn.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
}

func benchValue(v string) *commonpb.AnyValue {
	return &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}
}

func benchInt(v int64) *commonpb.AnyValue {
	return &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}}
}

func benchKV(key string, value *commonpb.AnyValue) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: value}
}
