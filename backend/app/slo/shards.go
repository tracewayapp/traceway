package slo

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
)

// SLOs are split into shardCount shards by id. On Postgres every backend
// instance leases a fair share of the shards, so the work spreads across
// instances instead of one advisory-lock holder computing every tenant.
const shardCount = 64

var (
	workerId       = uuid.NewString()
	ownedShards    atomic.Int64
	knownInstances atomic.Int64
)

type shardSet struct {
	all   bool
	owned map[int]bool
}

func (s shardSet) owns(sloId int) bool {
	return s.all || s.owned[sloId%shardCount]
}

func (s shardSet) size() int {
	if s.all {
		return shardCount
	}
	return len(s.owned)
}

func leaseDuration() time.Duration {
	if d := 3 * pollInterval(); d > 5*time.Minute {
		return d
	}
	return 5 * time.Minute
}

func claimShards(ctx context.Context) (shardSet, error) {
	if db.IsSQLite() {
		ownedShards.Store(shardCount)
		knownInstances.Store(1)
		return shardSet{all: true}, nil
	}
	lease := leaseDuration().Seconds()
	if _, err := db.DB.ExecContext(ctx, "INSERT INTO slo_workers (owner, seen_at) VALUES ($1, now()) ON CONFLICT (owner) DO UPDATE SET seen_at = now()", workerId); err != nil {
		return shardSet{}, fmt.Errorf("register slo worker: %w", err)
	}
	if _, err := db.DB.ExecContext(ctx, "DELETE FROM slo_workers WHERE seen_at < now() - make_interval(secs => $1)", 2*lease); err != nil {
		return shardSet{}, fmt.Errorf("prune slo workers: %w", err)
	}
	var instances int
	if err := db.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM slo_workers WHERE seen_at > now() - make_interval(secs => $1)", lease).Scan(&instances); err != nil {
		return shardSet{}, fmt.Errorf("count slo workers: %w", err)
	}
	if instances < 1 {
		instances = 1
	}
	owned, err := queryShards(ctx, "UPDATE slo_shard_leases SET expires_at = now() + make_interval(secs => $2) WHERE owner = $1 RETURNING shard", workerId, lease)
	if err != nil {
		return shardSet{}, fmt.Errorf("renew slo shard leases: %w", err)
	}
	target := (shardCount + instances - 1) / instances
	for shard := range owned {
		if len(owned) <= target {
			break
		}
		if _, err := db.DB.ExecContext(ctx, "DELETE FROM slo_shard_leases WHERE shard = $1 AND owner = $2", shard, workerId); err != nil {
			return shardSet{}, fmt.Errorf("release slo shard %d: %w", shard, err)
		}
		delete(owned, shard)
	}
	if missing := target - len(owned); missing > 0 {
		claimed, err := queryShards(ctx,
			"INSERT INTO slo_shard_leases (shard, owner, expires_at) "+
				"SELECT s, $1, now() + make_interval(secs => $2) FROM generate_series(0, $3 - 1) AS s "+
				"WHERE NOT EXISTS (SELECT 1 FROM slo_shard_leases l WHERE l.shard = s AND l.expires_at > now()) "+
				"ORDER BY random() LIMIT $4 "+
				"ON CONFLICT (shard) DO UPDATE SET owner = EXCLUDED.owner, expires_at = EXCLUDED.expires_at WHERE slo_shard_leases.expires_at <= now() "+
				"RETURNING shard",
			workerId, lease, shardCount, missing)
		if err != nil {
			return shardSet{}, fmt.Errorf("claim slo shards: %w", err)
		}
		for shard := range claimed {
			owned[shard] = true
		}
	}
	ownedShards.Store(int64(len(owned)))
	knownInstances.Store(int64(instances))
	return shardSet{owned: owned}, nil
}

func renewShards(ctx context.Context) error {
	if db.IsSQLite() {
		return nil
	}
	_, err := db.DB.ExecContext(ctx, "UPDATE slo_shard_leases SET expires_at = now() + make_interval(secs => $2) WHERE owner = $1", workerId, leaseDuration().Seconds())
	return err
}

func releaseShards() {
	if db.IsSQLite() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = db.DB.ExecContext(ctx, "DELETE FROM slo_shard_leases WHERE owner = $1", workerId)
	_, _ = db.DB.ExecContext(ctx, "DELETE FROM slo_workers WHERE owner = $1", workerId)
	ownedShards.Store(0)
}

func queryShards(ctx context.Context, query string, args ...any) (map[int]bool, error) {
	rows, err := db.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	shards := map[int]bool{}
	for rows.Next() {
		var shard int
		if err := rows.Scan(&shard); err != nil {
			return nil, err
		}
		shards[shard] = true
	}
	return shards, rows.Err()
}
