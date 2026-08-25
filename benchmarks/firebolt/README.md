# benchmarks/firebolt/

Direct-engine telemetry benchmark. Evaluates [Firebolt](https://docs.firebolt.io/self-managed/standalone-binaries/overview)
(self-managed engine, HTTP API on port 3473) as a candidate replacement for
ClickHouse as the main telemetry DB — **before** any backend integration
exists. Where the main hardware bench (`benchmarks/loadgen`) drives the
whole Traceway backend over OTLP, this one speaks SQL-over-HTTP straight at
the engine, so the numbers isolate the storage engine from the Go ingest
path. A `--dialect clickhouse` mode runs the identical workload against a
ClickHouse instance over its HTTP interface (port 8123), so the two engines
can be compared head-to-head on the same machine at the same abstraction
level:

```bash
./firebolt-bench --dialect clickhouse --target http://localhost:8123 --signal spans \
  --ch-user default --ch-password default --ch-database bench \
  --report-out ../results-firebolt-local/local-clickhouse-spans.json
```

The ClickHouse mode confines itself to the `--ch-database` database
(created if missing, tables dropped/truncated inside it) — safe to point at
a shared dev instance.

> These numbers are NOT comparable to `results-throughput/` /
> `results-probe/` numbers. Those include OTLP decode, auth, and the
> backend's insert path; these are raw engine ceilings. Use them to decide
> whether integration is worth building, and to compare Firebolt against
> raw-DuckDB / raw-ClickHouse equivalents at the same abstraction level.

## What it does

Per signal (`spans`, `metrics`, `logs`), one binary run:

1. **Schema.** Drops and recreates the signal's tables, mirroring
   `backend/app/migrations/duckdb_telemetry/` with Firebolt types
   (`TEXT`/`BIGINT`/`DOUBLE PRECISION`/`TIMESTAMP`) plus a `PRIMARY INDEX`
   matching the read pattern (`project_id, recorded_at` etc.).
2. **Ingest ramp.** Multi-row `INSERT INTO ... VALUES` with `--workers`
   concurrent connections, batch sizes stepping through `--batch-sizes`
   (default `256,1024,4096,8192,16384`), `--step-seconds` each. Records
   rows/sec and request p50/p95/p99 per step. Data variety mirrors the
   loadgen (13 endpoint paths, weighted status codes, 10 metric names,
   INFO/WARN/ERROR logs with 120-char bodies), timestamps spread over the
   trailing 24h.
3. **Read probe.** Truncates, then fills to each `--fill-levels` row count
   (default `1000000,10000000`) at `--fill-batch-size`, and runs the
   signal's dashboard-shaped queries `--probe-runs` times each:
   - spans → the grouped-endpoints query (Apdex buckets + p50/p95/p99 via
     `percentile_cont WITHIN GROUP ... FILTER`) and a 5-min-bucket latency
     chart
   - metrics → 1-min-bucket time series, the same grouped by a JSON tag
     (`JSON_POINTER_EXTRACT_TEXT`), and name discovery
   - logs → newest-50 page, range count, and a severity + `LIKE` search

   Run 0 is the cold query; later runs hit Firebolt's result cache. The
   JSON keeps every run, so report cold and warm separately.

## Running

Firebolt Core must be up (see `benchmarks/compose/docker-compose.firebolt.yml`):

```bash
docker compose -f benchmarks/compose/docker-compose.firebolt.yml up -d
curl -s http://localhost:3473/ping   # -> Ok.
```

Then:

```bash
cd benchmarks/firebolt && go build -o firebolt-bench .
for sig in spans metrics logs; do
  ./firebolt-bench --signal $sig \
    --report-out ../results-firebolt-local/local-firebolt-$sig.json
done
```

Useful flags: `--skip-ramp` (straight to fill+probe), `--reset=false`
(keep existing tables/data), `--fill-levels 1000000` (cheaper), `--workers 8`.

## Tuned mode (`--fb-tuned`): designing around Firebolt

The plain pass treats Firebolt as a drop-in ClickHouse (raw tables, raw
queries) — that is the *floor*. `--fb-tuned` redesigns the storage layer the
way an actual Firebolt-backed Traceway would:

- **Aggregating indexes** on each table — Firebolt's flagship feature:
  auto-maintained materialized aggregate states with transparent query
  rewrite. Verified via EXPLAIN on this build: `COUNT`/`AVG`/`SUM(CASE ...)`
  and **exact `PERCENTILE_CONT` states** (`percentile_contmerge`) all rewrite,
  including with `DATE_TRUNC` and `JSON_POINTER_EXTRACT_TEXT` expressions as
  grouping keys. Two gotchas found empirically: `FILTER (WHERE ...)` on an
  aggregate blocks the rewrite (the tuned grouped-endpoints query filters
  `is_stream` via a `WHERE` on the index key in a second subquery instead),
  and the query's time filter must be expressed on the `DATE_TRUNC` key
  (bucket-aligned windows — hour for spans/log counts, minute for metrics).
- **`VACUUM <table>` after each fill**, timed and recorded as
  `vacuumSeconds`. Thousands of INSERT batches leave fragmented tablets;
  probing without compaction measures fragmentation, not the engine — the
  same lesson as the main bench's DuckDB digestion gate.
- The ingest ramp stays ON in tuned runs so the index-maintenance cost on
  inserts is measured, not assumed.
- The log page / LIKE search have no aggregation to index — they stay raw
  scans and show what PRIMARY INDEX + VACUUM alone buy.

`run-breaking-point.sh` runs the untuned deep pass on both engines;
`run-tuned.sh` runs the tuned Firebolt pass at the same fill levels.
Compare the three: Firebolt floor vs Firebolt designed-for vs ClickHouse.
(A fair follow-up would tune ClickHouse too — the real CH backend already
has materialized 1m/1h metric rollups.)

## Running on Hetzner (the numbers that count)

Laptop Docker on macOS distorts this engine badly — no io_uring (VACUUM/spill
fails or crashes), unstable memory sizing after restarts, virtualized I/O.
`run-hetzner.sh` runs the whole suite on one dedicated Linux box:

```bash
export HCLOUD_TOKEN=... BENCHMARK_SSH_KEY=~/.ssh/hetzner_benchmark
SMOKE=1 ./run-hetzner.sh      # ~20 min pipeline check
./run-hetzner.sh              # full suite, ccx33, ~3-4h, ~EUR 1.50
```

One box (TIER=ccx33 default), both engines in Docker, sequential cells:
Firebolt untuned deep (30/60/100M) + concurrency, Firebolt tuned
(aggregating indexes + VACUUM), ClickHouse deep + concurrency. A failed
cell logs and continues — engine deaths are findings, not suite failures.
Results land in `benchmarks/results-firebolt-hetzner/` with a `summary.md`.
The server is deleted on exit via trap; anything labeled `bench=true` in
hcloud is safe to delete if a run is interrupted.

The end-to-end path (OTLP -> full Traceway backend -> Firebolt) is the
hardware matrix's `pgfb` mode instead: `../scripts/run-local.sh --tier ccx23
--mode pgfb` (see `benchmarks/README.md`).

## Dialect notes (vs DuckDB backend queries)

| DuckDB | Firebolt |
|---|---|
| `quantile_cont(col, p)` | `percentile_cont(p) WITHIN GROUP (ORDER BY col)` (composes with `FILTER`) |
| `time_bucket(to_seconds(N), col, TIMESTAMP '1970-01-01')` | `TO_TIMESTAMP(FLOOR(EXTRACT(EPOCH FROM col) / N) * N)` |
| `json_extract_string(tags, '$.key')` | `JSON_POINTER_EXTRACT_TEXT(tags, '/key')` |
| Appender API | multi-row `INSERT ... VALUES` over HTTP (no bulk-append client API; `COPY FROM` needs files) |

Caveats to keep in mind when reading results:

- Inserts are plain-text SQL over HTTP — payload build/parse overhead is
  real but the engine parse cost is part of what we're measuring, since an
  integration would pay it too (the Go SDK also speaks SQL over HTTP).
- The engine image is a dev build (`ghcr.io/firebolt-db/engine:dev`);
  numbers move between builds. `engineVersion` is recorded in every JSON.
- Docker Desktop on macOS virtualizes I/O — laptop numbers are for
  relative comparison and tooling validation, not publication. For real
  numbers run engine + bench on a Hetzner box like the main matrix.
