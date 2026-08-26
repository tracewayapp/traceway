# benchmarks/metricsdb

Which store should hold OTel metric points? Traceway keeps them in ClickHouse,
DuckDB or SQLite today (`metric_points`, tags as a Map or JSON column, no
codecs). This benchmark gives VictoriaMetrics, ClickHouse, DuckDB and Firebolt
Core one identical box each and measures, per database:

1. Max sustainable ingest in points per second.
2. Disk bytes per point once the store has settled (merges, checkpoints, vacuum).
3. Latency of routine dashboard queries while ingesting, and cold afterwards.
4. Where each one falls behind, and where it becomes unusable, at 10B points
   across 1M series.

The bench binary generates already parsed OTel-shaped data in memory and
pushes it through each database's most efficient native write path over
loopback, or in-process for DuckDB. No OTLP decode, no gzip, no network hop.
The binary stands in for the thin ingest API Traceway would put in front of
the store, so the numbers are the store's, not the API's.

Results are workflow artifacts plus the job step summary. Nothing is
committed. Numbers get written up under `docs/` once two runs agree.

## Topology

One cell is one database on one Hetzner box.

```
ccx33 (8 dedicated cores, 32 GB, local NVMe at /data)
 cores 0-5: DB container  (--cpuset-cpus 0-5 --memory 29g --network host, /data/<db> bind mount)
 cores 6-7: taskset -c 6,7 ./metricsdb-bench --db <db> ...
            generator threads -> bounded channel -> W writers -> loopback HTTP (in-process for DuckDB)
            + query task (routine queries every 15 s)
            + probe task (cgroup RSS/CPU, du, DB health)
            -> /opt/bench/out/result.json rewritten atomically every 10 s window
 orchestrator (GitHub runner or laptop): entry.sh polls once a minute, pulls
 the partial result.json, sends SIGTERM on the disk guard or the hard
 deadline, collects a post-mortem, tears down via trap
```

DuckDB has no container. It runs inside the bench process with
`threads = cores - 2`, and the bench's own cgroup is what gets sampled.

## Databases

| Database | Image | Write path | One ack means | Settle step | Disk measure |
|---|---|---|---|---|---|
| VictoriaMetrics | `victoriametrics/victoria-metrics:v1.150.0` | Prometheus remote write (protobuf + snappy) to `/api/v1/write`, 20k samples per request, 8 writers | 204: in an in-memory part, flushed within about 1 s | `/internal/force_flush`, `/internal/force_merge`, wait until `vm_active_merges == 0` and `vm_data_size_bytes` is stable | `du` of the storage dir |
| ClickHouse | `clickhouse/clickhouse-server:26.3.22-alpine` | RowBinary inserts of 500k rows over HTTP, 4 connections, `async_insert=0` | 200: the part is on disk | poll until no merges, the part count is stable twice and no merged-away part is left (`old_parts_lifetime = 30` on the table); never `OPTIMIZE FINAL` | `du` and `sum(bytes_on_disk)` both reported |
| clickhouse-map (optional) | same | same client, 200k rows into the current `metric_points` shape (tags as a Map, no codecs) | same | same | same |
| DuckDB | `libduckdb` 1.5.5 in-process (`bench/duckdb-version.txt`) | Arrow appender on batches of 4M rows sorted by `(series_id, ts)`, 1 writer | appender flush returned: in the WAL and visible | `CHECKPOINT`, assert the WAL is empty | `.duckdb` plus `.wal` |
| Firebolt Core | `ghcr.io/firebolt-db/engine:dev` (`db/firebolt/image.txt`; the resolved digest is recorded in every result) | Parquet in memory, multipart `INSERT ... FROM READ_PARQUET('upload://batch')`, 1M rows, 2 writers; `VACUUM` every 20 inserts or above 50 tablets, timed as maintenance because Core has no auto-vacuum; tables created `WITH (compression = ZSTD, compression_level = 3)` | INSERT returned: tablet committed | `VACUUM` until the tablet count stops shrinking | `du` of `/var/lib/firebolt` and the live `compressed_bytes` from `information_schema.tables`; vacuumed tablets stay on disk until Core's own garbage collection runs |

After every store's settle step the bench keeps re-walking the data dir
until it has stopped shrinking (up to 3 minutes), because merged-away parts
and vacuumed tablets are deleted asynchronously. `summary.md` shows both the
data-dir size and the store-reported live size; a gap is index, metadata and
uncollected garbage.

Codecs were chosen by measurement, not habit. On this corpus timestamps and
series ids compress to under 1% of the bytes; the value column is
everything. ClickHouse `Delta(8), ZSTD(3)` on `value` beats `Gorilla` by a
third (52 MB vs 81 MB for 20M points; ZSTD(6)/(9) buy 3-4% more for 2-3x the
CPU). Firebolt ZSTD level 3 beats its LZ4 default by 39%. DuckDB picks ALP
for doubles itself and gains 7% from 4M-row appends over 1M.

Every SQL store gets the same normalized model: a `series` dimension table
loaded once before ingest (timed, excluded from throughput) and a narrow
`points(series_id, ts, value)` fact table. Series ids are dense and
metric-major, so `name = X` is a primary-key range on every store.
VictoriaMetrics gets the same series as `__name__` plus sorted labels.
Firebolt Core has no DIMENSION tables, so its `series` is a fact table with
its own primary index. The DDL lives in `db/<name>/schema*.sql`, next to the
server config, and is applied by the bench binary itself. The shell scripts
never run SQL.

Server configs live in `db/<name>/`. ClickHouse gets `background_pool_size`
and `max_threads` equal to its cores (the `number_of_free_entries_in_pool_*`
settings scale with it, because ClickHouse refuses to start on an existing
database when they exceed the pool), a 30 GB merge size cap so merge scratch
fits the 240 GB disk, and its system logs removed so they never pollute the
disk measurement. The container runs with `TZ=UTC` and every query literal
is `toDateTime64(..., 'UTC')`, so a host timezone can never shift a window. `parts_to_delay_insert` and `parts_to_throw_insert` are
left at their defaults on purpose: "too many parts" is a legitimate
fell-behind signal. VictoriaMetrics gets its series caps disabled and a 100y
retention because the corpus carries simulated timestamps.

## How the sustainable rate is found

Acknowledged writes per second is not throughput for a store that can defer
work: ClickHouse defers merges, VictoriaMetrics defers merging small parts,
DuckDB defers checkpoints, Firebolt defers vacuum and garbage collection.
Every insert returns 200 while the backlog grows, until queries slow down or
inserts get throttled. Three things in the bench make that visible.

1. **Deferred work is tracked every window** as a per-store "debt":
   ClickHouse active parts, VictoriaMetrics small plus in-memory parts,
   DuckDB WAL bytes, Firebolt tablets. During the fill, a trailing 10 minute
   mean more than 30% above the previous 10 minutes for a full minute, or any
   throttled write (ClickHouse `DelayedInserts`), is a fell-behind verdict
   (`--debt-window`, `--debt-growth`).
2. **Digest time is charged to throughput.** `amortized_pps = fill points /
   (fill wall time + digest time)`, where digest is the settle step plus the
   wait for merged-away data to be deleted. A store that acknowledges 10B
   points in two hours and then merges for ninety minutes is scored on three
   and a half hours.
3. **`--mode ramp-then-fill` (the default)** runs fixed-rate steps
   (`--ramp-rates 250k,500k,1M,2M,4M,8M`, `--step 8m`, first `--ramp-in 60s`
   excluded). A step passes when the store accepts at least 90% of the
   offered rate, its debt does not grow across the step's two halves, the
   visibility lag stays under `--lag-threshold`, routine queries stay under
   `--query-slow-ms` with no timeouts, and nothing is throttled or lost. The
   highest passing rate is `sustainable_pps`; one geometric bisection narrows
   the gap (`--ramp-bisect`); the fill then runs at `--fill-fraction` (0.9)
   of it with the debt rule armed, because merge cost per row grows with
   table size and a rate that held at 500M points can fail at 8B. If every
   step passes the fill runs at saturation and the sustainable rate is
   reported as a lower bound; if none passes the fill runs at half the lowest
   rate. `--mode saturate` is the old behaviour (unbounded offered load) and
   `--mode ramp` stops after the ladder.

On ccx33 the default ladder plus bisection takes about an hour before the
fill starts, which leaves roughly 2.5 h of fill inside the 225 minute cap.

A dispatch runs `max_parallel` stores at once (default 2, one Hetzner box
each; a ccx33 is 8 dedicated vCPUs, so 2 fit a 24 vCPU quota next to one
other server). Separate dispatches for different stores also run side by
side, because the concurrency group is keyed on the `databases` input. Then
`scripts/combine.sh <out-dir> <run-id> [<run-id>...]` downloads the result
artifacts of those runs into one folder and renders the combined
`summary.md`, `report.html` and charts.

## What is measured

- **Peak points/s**: the best 10 s window after the 60 s warmup.
- **Sustained points/s**: the plateau, the highest trailing 5-window median
  of acked points/s after warmup. Acked means the database confirmed the
  write in the sense of the table above; each result records the semantics.
- **Bytes per point**: settled disk bytes divided by acked points. Two
  baselines sit next to it: `raw16` (16 bytes per point, timestamp plus
  value) and `logical` (what Traceway's converter holds in memory, including
  name and tag bytes).
- **Query latency**: seven routine dashboard queries (one metric per host,
  one series, top-k latest, a 24 h heavy aggregation, two discovery queries,
  an alert-style p95) with identical semantics per store. During ingest they
  rotate one at a time every 15 s with a 30 s deadline; a query returning
  fewer than half the expected rows is marked `suspect`. After settle the
  database is restarted and the page cache dropped, then each query runs
  three times for a cold first latency and a warm median.
- **Fell behind**: the first window where, for three windows in a row,
  visibility lag exceeds 120 s and keeps growing, throughput drops below
  half the plateau, query p95 exceeds `query_threshold_ms`, or more than 1%
  of writes error. The run keeps going.
- **Unusable**: the first window where, for three windows in a row, every
  query fails, nothing is acked while producing, more than half of writes
  error, or the database is unreachable, OOM-killed or restarted. Ingest
  continues 5 more minutes to show whether it recovers, then settle and cold
  queries run best effort.
- **Outcome**: `completed`, `unusable`, `wall_deadline`, `disk_full`,
  `interrupted` or `bench_error`; `setup_failed` and `crashed` come from
  `entry.sh` stubs when the bench never produced a result.
- **Bench bottleneck suspected**: writers idle more than 20% while
  generators stall less than 5%, or the bench uses more than 90% of its two
  cores. The summary flags such a run as invalid.

## Fairness rules

- Same box tier, same cpuset split: the database gets all cores but two,
  the bench the last two. DuckDB gets `threads = cores - 2` instead.
- `--memory` on every container is total RAM minus 3 GB, with swap disabled,
  so an OOM is attributable via `docker inspect` and the dmesg count in the
  post-mortem. DuckDB gets the same number as its `memory_limit`.
- Page cache dropped before launch and before the cold queries. The cold
  restart is a real `docker restart` (DuckDB closes and reopens the file).
- No `OPTIMIZE FINAL` on ClickHouse. Settle uses only what the store does on
  its own, plus the explicit flush or vacuum call listed in the table.
- Every database gets the same corpus. The generator's output depends only
  on `(seed, series_id, round)`, and a commutative fingerprint of all points
  is recorded in every result so equality can be checked.
- Batch sizes and writer counts are each database's own documented sweet
  spot, recorded in the result. `writers` and `batch_size` override them for
  every database at once.
- Firebolt's `VACUUM` and DuckDB's `CHECKPOINT` are timed as maintenance
  inside the ingest window. That is what an ingester would have to pay.

## Running

### Locally with Docker

```bash
./benchmarks/metricsdb/scripts/run-local.sh clickhouse --smoke
./benchmarks/metricsdb/scripts/run-local.sh victoriametrics,duckdb --smoke
```

Builds the bench (`fetch-libduckdb.sh` for the host platform, then
`cargo build --release`), starts the database with `db/<name>/up.sh`, runs
the bench in the foreground and writes `results-local/local-<db>.json`, then
runs `summarize.py` over the folder. On macOS the setup layer switches to
Docker Desktop mode by itself: no cpuset pinning, ports published on
loopback instead of host networking, a memory cap of at most 6 GB, data
under `results-local/data/` (Docker Desktop only shares `/Users` by default,
so a bind mount under `/tmp` is invisible from the host and the disk walk
would see nothing). Firebolt Core needs 16 GB and a Linux 6.1+ kernel and is
best effort there: `MEM_LIMIT_MB=7000` lets it start on an 8 GB Docker VM,
and a failure to start is a warning, not an error.

Local numbers say nothing about the real ones. Use `--smoke` to check the
pipeline, then run on Hetzner.

### From a laptop on Hetzner

```bash
export HCLOUD_TOKEN=... BENCHMARK_SSH_KEY=~/.ssh/hetzner_benchmark
SMOKE=1 ./benchmarks/metricsdb/scripts/run-local.sh clickhouse --hetzner
./benchmarks/metricsdb/scripts/run-local.sh victoriametrics,clickhouse,duckdb,firebolt --hetzner
```

The same `scripts/entry.sh` the workflow uses, one database after another.
It needs a linux-amd64 artifact in `_artifact/`. On a linux-amd64 laptop it
builds one; elsewhere download it from a workflow build job with
`gh run download <run-id> -n metricsdb-bench -D benchmarks/metricsdb/_artifact`.
`--dry-run` runs preflight and prints the plan without provisioning.

The Hetzner SSH key must be uploaded under the name `benchmark-key`
(`hcloud ssh-key create --name benchmark-key --public-key-from-file ...`).
See `benchmarks/README.md` for the one-time setup.

### From GitHub Actions

`.github/workflows/benchmark-metricsdb.yml`, `workflow_dispatch` only.
Dispatch with `smoke=true` first (about 15 min and EUR 0.05 per database),
then one full `databases=clickhouse` run, then the whole matrix. Inputs
mirror the env vars below.

Required repository secrets: `HCLOUD_TOKEN` and `BENCHMARK_SSH_PRIVATE_KEY`
(the private half of the Hetzner-side `benchmark-key`).

Jobs: `build` compiles the bench on `ubuntu-24.04` (glibc matches the
`ubuntu-24.04` box image, and its 6.8 kernel satisfies Firebolt), `setup`
turns the database list into a matrix, `run` provisions one box per
database with `max_parallel` at a time and uploads
`metricsdb-result-<tier>-<db>` (the result JSON plus `logs/`), `summary`
renders `summary.md`, `report.html` and the charts into the step summary and
the `metricsdb-summary` artifact.

Three layers keep a run under the 6 h job cap: the bench's own
`--max-ingest`, `--max-settle` and `--max-cold`, then `entry.sh`'s hard
deadline (SIGTERM, SIGKILL 180 s later), then the step's
`timeout-minutes: 330`. The partial result is pulled every minute, so every
layer still leaves data.

## Knobs

| Env / input | Default | Meaning |
|---|---|---|
| `TIER` | `ccx33` | Hetzner tier for every box. 20B points needs `ccx43`. |
| `LOCATION` | `nbg1` | Hetzner datacenter. |
| `TARGET_POINTS` | `10000000000` | Points per database. |
| `SERIES` | `1000000` | Distinct series in the generated fleet. |
| `INTERVAL_SECONDS` | `10` | Simulated scrape interval. |
| `MAX_MINUTES` | `225` | Ingest wall-clock cap. |
| `MAX_SETTLE_MINUTES`, `MAX_COLD_MINUTES` | `30`, `30` | Caps on the settle and cold phases (env only). |
| `QUERY_THRESHOLD_MS` | `5000` | Query p95 that counts as fallen behind. |
| `max_parallel` | `2` | Boxes provisioned at once for one dispatch; 2 x ccx33 fits a 24 dedicated-vCPU quota with one other server running. |
| `MODE` / `mode` | `ramp-then-fill` | `saturate` or `ramp` as described above. |
| `RAMP_RATES` / `ramp_rates` | `250k,500k,1M,2M,4M,8M` | Ramp ladder, strictly increasing. |
| `RAMP_STEP_SECONDS` / `step_seconds` | `480` | Seconds per ramp step; `RAMP_IN_SECONDS` (60) and `RAMP_BISECT` (1) are env only. |
| `WRITERS`, `BATCH_SIZE` | per database | Override the write concurrency and points per write. |
| `SMOKE` | `0` | `1` = 150M points, 10k series, ladder 500k,1M,2M with 30 s steps and no bisection, 5 min ingest cap, 1 min settle, 2 min cold. |
| `FB_STAGE` | `upload` | `s3` stages Firebolt batches through a loopback MinIO (`db/firebolt/minio-up.sh`), the fallback if `upload://` does not work on Core. Env only. |
| `OUT_DIR` | `results-local/` locally, `$RUNNER_TEMP/metricsdb-results` in CI | Where `<tier>-<db>.json` and `logs/` land. |
| `EXTRA_ARGS` / `extra_args` | empty | Extra `metricsdb-bench` flags appended verbatim, for example `--gen-threads 4` after a bench-limited run, `--rate 500000` for a fixed-rate stability run, or `--warmup 15s --query-interval 5s` for a short local run. |
| `DB_CPUS`, `BENCH_CPUS`, `MEM_LIMIT_MB`, `DATA_ROOT`, `HOST_NET`, `RESET` | see `db/_common.sh` | Setup-layer overrides, mostly for laptops. |

## Reading the output

Per cell: `<tier>-<db>.json` (the result, with `postmortem` merged in by
`entry.sh`) and `logs/<tier>-<db>.{bench.log,db.log,postmortem.json,dmesg.txt,df.txt}`.
The result carries a `timeline` of 10 s windows (acked points, write
latency percentiles, visibility lag, the query that ran, DB health,
process RSS and CPU, disk by class), the per-phase durations, throughput
and disk summaries, per-query latency during ingest and cold, and the
verdict.

`summarize.py` writes `summary.md` (one headline table per tier: peak and
sustained points/s, points stored, whether `max_minutes` was hit, bytes per
point, disk, settle time, per-query p50/p95 during ingest, cold first and
warm latencies, fell-behind and unusable times, outcome, image digest,
cpuset split, bench-bottleneck flag), `report.html`, and one chart per
tier for throughput over time (with fell-behind and unusable markers), disk
over time (with the settle delta), per-query p95 against points stored on a
log axis with the threshold line, RSS against the memory cap, visibility
lag, and grouped headline bars. Stubs render as `setup_failed` or `crashed`
rows so a cell that never ran is visible rather than missing.

A row with the bench-bottleneck flag is not a database result. Rerun that
matrix with `--gen-threads 4` and a wider bench cpuset for every database.

## Failure modes

- **Leftover servers.** `entry.sh` sets its teardown trap before the first
  `hcloud` call and deletes by name and by label; the workflow has a second
  `always()` delete by name. If both fail, `hcloud server list -l bench=true`
  shows what is left and anything with that label is safe to delete.
- **Firebolt `dev` drift.** `image.txt` pins `ghcr.io/firebolt-db/engine:dev`,
  the only documented tag. `up.sh` records the resolved digest in
  `IMAGE_REF`, so two runs can be compared. Replace the tag with an
  `@sha256:` pin once a run is worth reproducing.
- **DuckDB lib and crate mismatch.** `bench/duckdb-version.txt` drives
  `fetch-libduckdb.sh`; it must equal the `duckdb` crate version in
  `bench/Cargo.toml`. The fetch script warns when they differ. A mismatch
  shows up as a link error or a symbol error at startup.
- **Disk guard.** `entry.sh` sends SIGTERM when `/data` reaches 92%. The
  bench finishes its window and writes the result; `entry.sh` sets
  `verdict.stopped_reason` to `disk_full`. Points stored so far are kept.
- **`max_minutes` hit.** The result says `hit_max_ingest=true` with the
  points stored. Compare bytes per point and query latency at equal points
  stored, not at equal wall time.
- **Bench died within 5 s of launch.** `entry.sh` prints the tail of
  `bench.log`; exit code 3 is a schema, setup or catalog failure and
  produces a `setup_failed` stub.
- **Box unreachable mid-run.** Five failed polls in a row end the poll loop.
  The last pulled partial result stays, the post-mortem is skipped, and the
  box is deleted.

## Layout

```
benchmarks/metricsdb/
  README.md
  .gitignore
  bench/                    Rust crate metricsdb-bench (Cargo.lock committed)
    duckdb-version.txt      libduckdb pin, read by scripts/fetch-libduckdb.sh
  db/
    _common.sh              cpuset/memory math, DATA_DIR, docker args, health probe, db.env
    victoriametrics/        up.sh, restart.sh
    clickhouse/             up.sh, restart.sh, schema.sql, schema-map.sql, config.d/, users.d/
    duckdb/                 up.sh, restart.sh, schema.sql
    firebolt/               up.sh, restart.sh, schema.sql, minio-up.sh, image.txt
  scripts/
    entry.sh                one cell end to end on Hetzner (CI and laptop --hetzner)
    remote-prep.sh          runs on the box once: docker, jq, ufw, sysctls, dirs
    remote-status.sh        runs on the box each poll: "<done|running|gone> <disk%> <last log line>"
    remote-collect.sh       runs on the box at the end: db.log, docker inspect, dmesg, df, du
    run-local.sh            laptop mirror with Docker, and --hetzner passthrough
    fetch-libduckdb.sh      pinned libduckdb for linux-amd64, linux-arm64, osx-universal
    preflight.sh            hcloud, key, benchmark-key, db name, artifact --version
    summarize.py            summary.md, report.html and charts over one result.json per cell
```
