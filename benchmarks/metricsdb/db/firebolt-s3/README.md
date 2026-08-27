# firebolt-s3: does disaggregation save Firebolt?

The single-node Firebolt Core cell failed the 10B-point endurance run in a
specific way: at ~450k points/s sustained, tablet merge debt grew without
bound (tablet floor 16 -> 38), vacuumed tablets stayed on disk until the
engine's own garbage collection got to them (205.9 GB on disk for 16.6 GB of
live data), the disk hit 91%, and writers stopped acknowledging at 4.40G
points with 150M points lost. ClickHouse completed the same corpus at
900k points/s on the same box.

Firebolt's production self-managed shape is not single-node local disk: it is
the Kubernetes operator with engines over object storage, where compaction can
run on compute the ingest path never sees and "disk" is a bucket that cannot
fill. This cell tests exactly that claim, on one Hetzner box:

    MinIO (docker)  <- tablet storage, du-walked as the bytes/point metric
    k3s
      firebolt-operator
      FireboltInstance (metadata + gateway)
      FireboltEngine "ingest"     ~40% of cores/RAM  <- all writes
      FireboltEngine "analytics"  ~25% of cores/RAM  <- all queries, all VACUUM

Routing uses the gateway's `X-Firebolt-Engine` header (`--fb-write-engine` /
`--fb-query-engine` / `--fb-maint-engine` on the bench). The schema is the
codec-tuned variant (`schema.sql` here: per-column Delta / DoubleDelta /
Gorilla chained with ZSTD(3)), closing the compression gap the single-node
cell ran with (table-level ZSTD only, 46.8 bytes/point on disk vs
ClickHouse's 2.23 with per-column codecs).

## What it measures

Same harness, corpus, ramp ladder and query set as every other cell, plus a
sidecar (`s3-stats.sh` -> `out/s3-metrics.jsonl` in the logs artifact)
sampling every 30 s:

1. **Debt steady state** - the tablet-count floor over time, on a deployment
   where VACUUM runs on the analytics engine. The single-node question
   "does merge keep up?" becomes "can *separate* compute keep up?"
2. **Compaction compute cost** - per-pod CPU/memory from the k3s
   metrics-server, so the analytics engine's VACUUM burn is visible.
3. **S3 request rates** - MinIO's `s3_requests_total` by API call and traffic
   counters: the hidden bill of tablets-as-objects (PUT storms on ingest,
   LIST/DELETE storms on GC).
4. **Query latency across engines** - queries hit the analytics engine while
   the ingest engine takes writes, so read p95 under ingest measures true
   workload isolation; the cold phase (`restart.sh` bounces every Firebolt
   pod) pays full metadata reload + tablet fetch from the object store.

## Verdict criteria

- Debt floor flat over the fill and bytes/point in the object store within
  ~2x of live data: disaggregation genuinely fixes the failure mode - the
  conversation with Firebolt moves to cost per point vs a ClickHouse shard.
- Debt floor still rising, or writers stalling while the bucket grows
  unbounded: the collapse is architectural, not a local-disk artifact, and
  single-node ClickHouse remains strictly better for this workload.

## Running it

Dispatch `benchmark-metricsdb` from this branch with:

    databases: firebolt-s3
    tier:      ccx43        (two engines + metadata + MinIO + k3s need 16 cores / 64 GB)
    smoke:     true         first - validates operator install, engine routing,
                            codec DDL and the sidecar in ~20 min for cents

then the full run with defaults (10B points, 1M series). Costs about
EUR 0.55/h on ccx43; a full cell is under EUR 3.

Caveats worth carrying into the write-up: MinIO over loopback flatters S3
latency (no network RTT, no request pricing), a single node still shares
total CPU between the engines, and `engine:dev` is an unpinned tag - the
result JSON records the resolved digest.
