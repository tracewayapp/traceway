# Running benchmarks locally

Step-by-step recipe for running the loadgen against a backend on your laptop —
no Hetzner, no GitHub Actions. Useful for: iterating on the loadgen itself,
checking that a backend change didn't regress single-machine throughput, or
just understanding the tooling before spending real money on Hetzner.

> Laptop numbers are not comparable to Hetzner numbers. Use this for tooling
> validation and relative regression checks, not for the published chart.

---

## Prerequisites

Install once:

```bash
# macOS
brew install docker jq go

# Confirm Docker is running
docker info >/dev/null && echo "docker: ok"
```

You also need Python 3 with matplotlib. The cleanest path on macOS (system
Python is PEP-668 locked and refuses `pip install`):

```bash
# Pick one of:

# Option A — venv in /tmp (wiped on restart, fine for ad-hoc smoke tests)
python3 -m venv /tmp/bench-venv && /tmp/bench-venv/bin/pip install -q matplotlib

# Option B — venv inside the repo (persists across restarts; gitignored)
python3 -m venv .venv-bench && .venv-bench/bin/pip install -q matplotlib
```

The rest of this doc assumes option B, so swap `/tmp/bench-venv` for
`.venv-bench` if you went with option A.

---

## Pick a backend mode

Three compose stacks, each pinned to one of the project's existing Dockerfiles:

| Mode | Compose file | Image | What it tests |
|------|--------------|-------|---------------|
| `sqlite` | `benchmarks/compose/docker-compose.sqlite.yml` | `Dockerfile.sqlite` | Single-binary backend with embedded SQLite. Fast to bring up, lower ceiling. |
| `pgch` | `benchmarks/compose/docker-compose.pgch.yml` | `Dockerfile.minimal` + clickhouse + postgres | Full prod-shape stack. Slower first build, much higher ceiling. |
| `pgfb` | `benchmarks/compose/docker-compose.pgfb.yml` | `Dockerfile.firebolt` + firebolt engine + postgres | Firebolt as the telemetry store (ClickHouse replacement candidate). Engine image via `FIREBOLT_IMAGE`, default `ghcr.io/firebolt-db/engine:dev`. |
| `managed-ch` | `benchmarks/compose/docker-compose.managed-ch.yml` | `Dockerfile.minimal` + postgres (CH is external) | Same `Dockerfile.minimal` as pgch but pointed at an external managed ClickHouse via env vars. |

All expose port **8087** on the host (override with `BENCH_PORT=8088 docker
compose -f ... up`).

First-time builds:
- `sqlite` mode: ~3–6 min (npm install + Go build).
- `pgch` mode: ~6–10 min (above + ClickHouse + Postgres image pulls).
- `managed-ch` mode: ~3–5 min (same as pgch without the CH pull).

Subsequent runs reuse the Docker layer cache and start in seconds.

### Extra setup for `managed-ch`

Before running `--mode managed-ch`, export the ClickHouse credentials in your
shell (preflight will fail loudly if they're missing):

```bash
export CLICKHOUSE_SERVER='your-cluster.region.aws.clickhouse.cloud:9440'
export CLICKHOUSE_USERNAME='bench'
export CLICKHOUSE_PASSWORD='••••••'
# Optional:
export CLICKHOUSE_DATABASE='traceway'         # default: traceway
export BENCH_CH_HTTPS_PORT='8443'             # default: 8443 (CH Cloud); some hosts 8123
```

The orchestrator wipes (`DROP DATABASE IF EXISTS` + `CREATE DATABASE`) the
configured database before each matrix entry — **use a dedicated database**
or other tables in the same DB will be lost.

---

## The 6-step run

These are the exact commands. Open a terminal at the repo root:

```bash
cd /Users/dusanstanojevic/Documents/workspace/traceway
```

### 1. Bring up the backend on a clean volume

```bash
# Always tear down with -v first. Reusing a volume across runs means the next
# loadgen run starts with old data, AND can hit "database disk image is
# malformed" if the previous container was killed mid-write.
docker compose -f benchmarks/compose/docker-compose.sqlite.yml down -v 2>/dev/null
docker compose -f benchmarks/compose/docker-compose.sqlite.yml up -d --build
```

For `pgch` mode swap the filename to `docker-compose.pgch.yml` here and in the
remaining steps.

### 2. Wait for `/health` to return 200

```bash
until curl -sf http://localhost:8087/health >/dev/null; do echo "waiting..."; sleep 2; done
echo "ready"
```

If this loops for more than ~3 min, something is wrong. Inspect:

```bash
docker compose -f benchmarks/compose/docker-compose.sqlite.yml logs --tail=80 traceway
```

### 3. Register a fresh user + project; capture the project token

```bash
eval "$(benchmarks/scripts/seed-project.sh http://localhost:8087 \
  | jq -r '"export PROJECT_TOKEN=\(.projectToken)"')"
echo "PROJECT_TOKEN=$PROJECT_TOKEN"
```

`seed-project.sh` hits `/api/register`. On self-hosted backends only one
organization is allowed, so this only works on a **fresh** DB — step 1's
`down -v` is what guarantees that. The throughput scenario only needs the
project token (OTLP ingestion is bearer-authed); the read-probe scenario
also needs the JWT + project ID for the dashboard endpoints — see
[Running the read-probe scenario locally](#running-the-read-probe-scenario-locally)
below.

### 4. Build and run the loadgen, one signal at a time

```bash
(cd benchmarks/loadgen && go build -o loadgen .)
mkdir -p /tmp/bench-localhost && rm -f /tmp/bench-localhost/*.json
```

Pick numbers appropriate for the mode you booted in step 1. The throughput
scenario runs a three-phase ramp:
1. **Phase 1** grows batch size at a fixed low request rate (saturates one
   client; finds the per-request decode/insert ceiling).
2. **Phase 2** grows request rate at the winning batch size capped at
   `--phase2-batch-cap` (collector shape: fat batches, low req rate).
3. **Phase 3** grows request rate at batch=100 (SDK-fleet shape: small
   batches, high req rate — stresses HTTP/auth/decode rather than insert).

The examples below trim Phase 3 to keep laptop runs short; remove the
`--phase3-*` overrides to exercise it.

**SQLite (laptop) — quick smoke per signal:**

```bash
for sig in spans metrics logs; do
  benchmarks/loadgen/loadgen \
    --target http://localhost:8087 \
    --token "$PROJECT_TOKEN" \
    --signal "$sig" \
    --duration 5m --step-duration 30s \
    --phase1-batch-sizes 256,1024,4096 \
    --phase2-request-rates 1,5,20 \
    --phase3-request-rates 10,100 \
    --report-out "/tmp/bench-localhost/local-sqlite-${sig}-throughput.json" \
    --tier local --mode sqlite
done
```

**pgch (laptop):**

```bash
for sig in spans metrics logs; do
  benchmarks/loadgen/loadgen \
    --target http://localhost:8087 \
    --token "$PROJECT_TOKEN" \
    --signal "$sig" \
    --duration 8m --step-duration 45s \
    --phase1-batch-sizes 256,1024,4096,8192 \
    --phase2-request-rates 1,5,20,50 \
    --phase3-request-rates 10,100,500 \
    --report-out "/tmp/bench-localhost/local-pgch-${sig}-throughput.json" \
    --tier local --mode pgch
done
```

The `-throughput` suffix on the filename matches the scenario folder convention
(`benchmarks/results-throughput/`); use `-read-probe` when running
`--scenario read-probe` (see [Running the read-probe scenario locally](#running-the-read-probe-scenario-locally) below).

On stderr you'll see per-phase progress like:

```
phase1 step 1: batch=256 rate=5.0 items/s=1280 p99=12ms err=0.00% passed=true
phase1 step 2: batch=1024 rate=5.0 items/s=5120 p99=18ms err=0.00% passed=true
phase1 step 3: batch=4096 rate=5.0 items/s=20480 p99=42ms err=0.00% passed=true
phase2 step 1: batch=4096 rate=1.0 items/s=4096 p99=15ms err=0.00% passed=true
phase2 step 2: batch=4096 rate=5.0 items/s=20480 p99=21ms err=0.00% passed=true
phase2 step 3: batch=4096 rate=20.0 items/s=81920 p99=180ms err=0.00% passed=true
phase3 step 1: batch=100  rate=10.0 items/s=1000 p99=8ms err=0.00% passed=true
phase3 step 2: batch=100  rate=100.0 items/s=10000 p99=22ms err=0.00% passed=true
wrote /tmp/bench-localhost/local-sqlite-spans-throughput.json: signal=spans max sustainable spans/sec = 81920
```

A run finishes when either (a) a step fails the error threshold, (b) all
configured steps pass, or (c) `--duration` expires.

### 5. Render the charts + summary

```bash
.venv-bench/bin/python benchmarks/scripts/chart.py /tmp/bench-localhost/
open /tmp/bench-localhost/chart-spans.png            # headline bars per tier × mode
open /tmp/bench-localhost/chart-phase2-rate-spans.png  # collector-shape rate ramp
open /tmp/bench-localhost/chart-pareto-spans.png       # latency–throughput trade-off
open /tmp/bench-localhost/chart-cliff-spans.png        # step-by-step pass/fail grid
cat /tmp/bench-localhost/summary.md
```

The full chart suite (~14 PNG types per scenario folder, headline +
phase-by-phase ramps + Pareto + tier scaling + cliff grids + batch
efficiency + signal mix, plus read-probe equivalents) is described in
[charts.md](charts.md). `chart.py` auto-detects which scenarios are in
the folder and only renders the relevant charts.

A one-entry result renders fine; it just won't have multiple bars/lines to
compare against. Old pre-OTLP-split JSONs (without a `signal` field) are
skipped with a log line.

### 6. Tear down

```bash
docker compose -f benchmarks/compose/docker-compose.sqlite.yml down -v
```

The `-v` deletes the named volume so the next run is guaranteed clean.

---

## Reading the output

Each throughput result JSON has up to three phases plus a headline:

```json
{
  "tier": "local", "mode": "sqlite", "signal": "spans",
  "scenario": "throughput",
  "startedAt": "2026-05-15T...",  "endedAt": "...",
  "phase1": {
    "kind": "batch-size-ramp",
    "fixedRequestRate": 5,
    "steps": [
      {
        "step": 1, "batchSize": 256, "requestRate": 5,
        "attemptedItemsPerSec": 1280, "actualItemsPerSec": 1280, "rejected": 0,
        "ingest": { "p50": 8, "p95": 14, "p99": 22, "ok": 150, "errors": 0, "errRate": 0 },
        "passed": true
      }
    ],
    "maxBatchSize": 8192
  },
  "phase2": {
    "kind": "request-rate-ramp",
    "fixedBatchSize": 8192,
    "steps": [ ... ],
    "maxRequestRate": 100
  },
  "phase3": {
    "kind": "small-batch-rate-ramp",
    "fixedBatchSize": 100,
    "steps": [ ... ],
    "maxRequestRate": 1000
  },
  "maxSustainableItemsPerSec": 819200
}
```

**A step passes** when the combined error rate (HTTP failures + OTLP
`PartialSuccess` rejected items) is ≤ `--ingest-err-threshold` (default 5%)
AND the achieved request rate is ≥ `--soft-cliff-ratio × target` (default
70%). The second check catches saturated-but-not-erroring soft cliffs.

`maxSustainableItemsPerSec` is the **highest `actualItemsPerSec` recorded
across any passing step from any phase** — measured from real OK responses,
not `batchSize × targetRate` (which over-reports when the workers saturate
before the limiter does). Different phases probe different shapes
(collector fat-batch vs SDK small-batch); the headline reports the best
shape's ceiling and per-phase numbers stay in the JSON for shape-specific
analysis.

Read-probe JSONs have a different shape — see [Running the read-probe
scenario locally](#running-the-read-probe-scenario-locally) below.

---

## What an "item" is

Each signal counts a different unit:

| Signal | Unit | OTLP request |
|---|---|---|
| `spans` | one span (one row in `spans` / one trace fan-out) | `POST /api/otel/v1/traces` with `ExportTraceServiceRequest` |
| `metrics` | one Gauge data point (one row in `metric_points`) | `POST /api/otel/v1/metrics` with `ExportMetricsServiceRequest` |
| `logs` | one `LogRecord` (one row in `log_records`) | `POST /api/otel/v1/logs` with `ExportLogsServiceRequest` |

All bodies are protobuf, gzipped. The loadgen is OTLP-only — no `/api/report`.

### Data variety

Everything is uniform random within bounded ranges — deliberately flat:

- **Spans:** 13 endpoint paths, status codes 200/201/404/500, durations 10–1000 ms
- **Metrics:** 10 metric names (`bench.metric.{cpu,mem,qps,lat,errs,disk,net,heap,gc,fd}`), Gauge type, low-cardinality `host` attr
- **Logs:** severity INFO/WARN/ERROR (weighted toward INFO), random 120-char body, 5 attrs incl. synthetic trace_id

This isolates raw throughput from traffic-shape noise. It is **not**
representative of real production telemetry; don't read the numbers as "this
is what users will experience" — they're "this is the floor the backend can
sustain on synthetic, evenly-distributed load."

---

## Tuning the run

| Flag | Default | When to change |
|------|---------|----------------|
| `--signal` | (required) | One of `spans`, `metrics`, `logs`. |
| `--scenario` | `throughput` | `throughput` (three-phase ingest ramp) or `read-probe` (fill the table, probe a read at each level). |
| `--phase1-batch-sizes` | `256,1024,4096,8192,16384` | Drop the lower steps on beefy boxes to save time; raise the cap to find the per-request decode/insert ceiling. |
| `--phase2-request-rates` | `1,5,25,100,400` | Raise the cap to find the request-rate ceiling on big boxes; lower for cheap smoke tests. |
| `--phase3-request-rates` | `10,100,1000,5000,10000` | The SDK-fleet shape ramp at batch=100. Lower the cap for fast smoke tests; drop the flag entirely to use the default range. |
| `--phase1-fixed-rate` | 5 | Per-second request rate held constant during Phase 1. Small enough that batch size is the only variable. |
| `--phase2-batch-cap` | 16384 | Cap on Phase 2 batch size. Phase 2 uses `min(this, Phase 1 winner)`. Bumped past the OTel collector default of 8192 because pgch SUTs can usefully exceed it. |
| `--phase3-batch-size` | 100 | Fixed batch size for Phase 3 — matches typical language-SDK `BatchSpanProcessor` output rather than the collector's 8192. |
| `--soft-cliff-ratio` | 0.70 | A step also fails when achieved req-rate falls below this fraction of target — catches saturated-but-not-erroring cliffs. Set to 0 to disable. |
| `--step-duration` | 2m | Shorten for smoke checks (15–30s), keep at 2m for real measurements. |
| `--duration` | 30m | Hard cap on total wall time. |
| `--ingest-err-threshold` | 0.05 | Combined HTTP error + OTLP rejected rate that fails a step. |
| `--fill-levels` | `100000,1000000,10000000,100000000` | (read-probe only) Row counts to fill the table to before each probe. |
| `--read-threshold-ms` | 5000 | (read-probe only) A probe fails if the read exceeds this. |

The ramp is geometric (256 → 1024 → 4096 → …), so cliff resolution is coarse
by design — for a precise number, run a second pass with a narrower band of
values around the known cliff.

---

## Common pitfalls

- **`database disk image is malformed (11)`** in the container logs.
  Previous run was killed mid-write and the SQLite WAL didn't recover. Run
  step 1 again with `down -v`.

- **`actualItemsPerSec` is much lower than `batchSize × requestRate`.**
  Items are being dropped — either HTTP failures or OTLP `PartialSuccess`
  rejections. Check `rejected` and `ingest.errors` for the culprit.

- **`seed-project.sh` returns `409 Conflict: organization already exists`.**
  The DB isn't fresh — go back to step 1 with `down -v`.

- **Port 8087 already in use.**
  Pick another: `BENCH_PORT=8088 docker compose -f ... up -d --build`, then
  use `http://localhost:8088` everywhere.

- **`/tmp/bench-localhost/` gone after a restart.**
  macOS empties `/tmp` on reboot. Use `~/bench-localhost/` or a path inside
  the repo if you want results to survive.

- **Loadgen run was killed; some Docker resources still up.**
  The compose stack is yours to manage — it won't auto-tear-down. Run step 6.

---

## Running the read-probe scenario locally

The throughput scenario above answers "how fast can the SUT swallow OTLP
data?" The read-probe scenario answers "how big can the table grow before
the dashboard read on this endpoint cliffs?" Each fill level is reached by
ingesting OTLP until the row count is hit, then the loadgen waits
`--settle-seconds` and issues one read against the signal's dashboard
endpoint (`/api/endpoints/grouped` for spans, `/api/metrics/application`
for metrics, `/api/logs` for logs). The step fails when the read takes
longer than `--read-threshold-ms` (default 5 s).

Read-probe needs a JWT + project ID, not just a project token, because the
dashboard endpoints are JWT-authenticated. `seed-project.sh` returns both:

```bash
seed_json=$(benchmarks/scripts/seed-project.sh http://localhost:8087)
export PROJECT_TOKEN=$(echo "${seed_json}" | jq -r .projectToken)
export PROJECT_ID=$(echo "${seed_json}" | jq -r .projectId)
export JWT=$(echo "${seed_json}" | jq -r .jwt)
```

Then run the loadgen with `--scenario read-probe`. Small fill levels keep
the laptop honest:

```bash
mkdir -p /tmp/bench-localhost && rm -f /tmp/bench-localhost/*.json
for sig in spans metrics logs; do
  benchmarks/loadgen/loadgen \
    --target http://localhost:8087 \
    --token "$PROJECT_TOKEN" --jwt "$JWT" --project-id "$PROJECT_ID" \
    --signal "$sig" --scenario read-probe \
    --fill-levels 10000,100000,1000000 \
    --settle-seconds 5s \
    --report-out "/tmp/bench-localhost/local-pgch-${sig}-read-probe.json" \
    --tier local --mode pgch
done
.venv-bench/bin/python benchmarks/scripts/chart.py /tmp/bench-localhost/
open /tmp/bench-localhost/chart-readprobe-headline-spans.png
open /tmp/bench-localhost/chart-readprobe-spans.png
open /tmp/bench-localhost/chart-readprobe-cliff-spans.png
```

`chart.py` against a folder mixing throughput and read-probe JSONs renders
both suites — the wrong-scenario renderers are no-ops. In production, the
CI workflow keeps them separate by writing to `benchmarks/results-throughput/`
vs `benchmarks/results-probe/` based on the `--scenario` flag.

Read-probe JSONs have a different shape from throughput:

```json
{
  "tier": "local", "mode": "pgch", "signal": "spans",
  "scenario": "read-probe",
  "readProbe": {
    "readPath": "/api/endpoints/grouped",
    "readThresholdMs": 5000,
    "settleSeconds": 5,
    "fillBatchSize": 8192,
    "fillRequestRate": 100,
    "steps": [
      {
        "fillLevelTarget": 10000,
        "rowsIngested": 10123,
        "ingestSecondsElapsed": 3.2,
        "readLatencyMs": 180,
        "readOk": true,
        "passed": true
      }
    ],
    "maxFillLevelPassed": 1000000
  },
  "maxFillLevelPassed": 1000000
}
```

The headline number is `maxFillLevelPassed` — the largest row count at
which the read came back under threshold. The matching read latency lives
in the last passing step's `readLatencyMs`.

---

## Going beyond local

Once the local run works end-to-end, move to the Hetzner ladder:

```bash
# Validate Hetzner credentials, no money spent
./benchmarks/scripts/run-local.sh --dry-run

# ~5 min, ~€0.02 — one tier, one mode, one signal, on real Hetzner
./benchmarks/scripts/run-local.sh --smoke

# Full matrix — 4 tiers x 2 modes x 3 signals, ~3 h, ~€3.60
./benchmarks/scripts/run-local.sh

# Subset: one signal only across all tiers/modes
./benchmarks/scripts/run-local.sh --signal spans
```

Prereqs and detail in [README.md](./README.md). The same `run-matrix-entry.sh`
script powers both the laptop path and the GitHub Actions workflow.
