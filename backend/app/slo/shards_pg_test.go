//go:build transactional_pg

package slo

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
)

func setupShardDB(t *testing.T) {
	t.Helper()
	password := os.Getenv("TEST_POSTGRES_PASSWORD")
	if password == "" {
		t.Skip("TEST_POSTGRES_PASSWORD is not set")
	}
	host := os.Getenv("TEST_POSTGRES_HOST")
	if host == "" {
		host = "localhost"
	}
	base := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable", host, os.Getenv("TEST_POSTGRES_USERNAME"), password, os.Getenv("TEST_POSTGRES_DATABASE"))
	admin, err := sql.Open("postgres", base)
	if err != nil {
		t.Fatal(err)
	}
	schema := "slo_shards_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	conn, err := sql.Open("postgres", base+" search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"0157_create_slo_workers.up.sql", "0158_create_slo_shard_leases.up.sql"} {
		migration, err := os.ReadFile("../migrations/pg/" + file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	previousDB, previousDriver, previousConfig, previousWorker := db.DB, db.Driver, config.Config, workerId
	db.DB, db.Driver = conn, lit.PostgreSQL
	config.Init(&config.Cfg{})
	t.Cleanup(func() {
		conn.Close()
		_, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		admin.Close()
		db.DB, db.Driver, config.Config, workerId = previousDB, previousDriver, previousConfig, previousWorker
	})
}

func claimAs(t *testing.T, worker string) shardSet {
	t.Helper()
	workerId = worker
	shards, err := claimShards(context.Background())
	if err != nil {
		t.Fatalf("claim as %s: %v", worker, err)
	}
	return shards
}

func TestShardsSplitBetweenInstancesAndMoveWhenOneStops(t *testing.T) {
	setupShardDB(t)
	if got := claimAs(t, "a").size(); got != shardCount {
		t.Fatalf("a single instance owns every shard, got %d", got)
	}
	if got := claimAs(t, "b").size(); got != 0 {
		t.Fatalf("a new instance waits for shards to be released, got %d", got)
	}
	a := claimAs(t, "a")
	b := claimAs(t, "b")
	if a.size() != shardCount/2 || b.size() != shardCount/2 {
		t.Fatalf("two instances split the shards: a=%d b=%d", a.size(), b.size())
	}
	for shard := range a.owned {
		if b.owned[shard] {
			t.Fatalf("shard %d is leased twice", shard)
		}
	}
	for id := 1; id <= 200; id++ {
		if a.owns(id) == b.owns(id) {
			t.Fatalf("SLO %d must belong to exactly one instance", id)
		}
	}

	if _, err := db.DB.Exec("UPDATE slo_workers SET seen_at = now() - interval '1 hour' WHERE owner = 'b'"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("UPDATE slo_shard_leases SET expires_at = now() - interval '1 second' WHERE owner = 'b'"); err != nil {
		t.Fatal(err)
	}
	if got := claimAs(t, "a").size(); got != shardCount {
		t.Fatalf("the remaining instance takes over the stopped one's shards, got %d", got)
	}

	workerId = "a"
	releaseShards()
	if got := claimAs(t, "c").size(); got != shardCount {
		t.Fatalf("released shards are free at once, got %d", got)
	}
}
