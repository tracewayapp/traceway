//go:build telemetry_ch

package monitoring

import (
	"context"
	"os"
	"testing"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/chdb"
)

func TestReadRunningQueriesSeesALongReadAndNotItself(t *testing.T) {
	server := os.Getenv("TEST_CLICKHOUSE_SERVER")
	if server == "" {
		t.Skip("TEST_CLICKHOUSE_SERVER not set")
	}
	auth := ch.Auth{Database: "default", Username: os.Getenv("TEST_CLICKHOUSE_USERNAME"), Password: os.Getenv("TEST_CLICKHOUSE_PASSWORD")}
	if auth.Username == "" {
		auth.Username = "default"
	}
	conn, err := ch.Open(&ch.Options{Addr: []string{server}, Auth: auth})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	previous := chdb.Conn
	chdb.Conn = conn
	defer func() { chdb.Conn = previous }()

	ctx := context.Background()
	idle, err := readRunningQueries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if idle["read"].count != 0 || idle["insert"].count != 0 {
		t.Fatalf("an idle server reports running queries: %+v", idle)
	}

	slow, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = conn.Exec(slow, "SELECT sleepEachRow(0.5) FROM numbers(8) FORMAT Null SETTINGS max_block_size = 1")
	}()

	deadline := time.Now().Add(3 * time.Second)
	for {
		busy, err := readRunningQueries(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if busy["read"].count == 1 && busy["read"].longestSec > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the running read was not seen: %+v", busy)
		}
		time.Sleep(100 * time.Millisecond)
	}
	cancel()
	<-done
}
