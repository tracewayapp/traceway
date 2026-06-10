# benchmarks/

Hardware-vs-throughput benchmark for the Traceway backend. Provisions
Hetzner Cloud servers, runs OTLP ingest in realistic batch shapes (one signal
per matrix entry), and produces per-signal charts of sustainable throughput
per hardware tier and DB mode.

## What it answers

> On hardware tier **X** with DB config **Y**, you can sustain **N** spans/sec,
> **M** metric data points/sec, and **P** log records/sec via OTLP under
> collector-shaped batch traffic (gzipped protobuf, batches up to 8192).

Three signals are tested in separate matrix entries:
- **spans** → `POST /api/otel/v1/traces` (`ExportTraceServiceRequest`)
- **metrics** → `POST /api/otel/v1/metrics` (`ExportMetricsServiceRequest`, Gauge data points)
- **logs** → `POST /api/otel/v1/logs` (`ExportLogsServiceRequest`)

Three DB modes are supported:
- **sqlite** — single-binary Traceway with embedded SQLite (`Dockerfile.sqlite`).
- **pgch** — full ClickHouse + Postgres stack, all in Docker on the SUT (`Dockerfile.minimal`).
- **managed-ch** — `Dockerfile.minimal` pointed at an externally-hosted ClickHouse (ClickHouse Cloud, Aiven, Altinity, etc.) via env vars. Postgres still runs locally in the SUT's Docker. See [Running against managed ClickHouse](#running-against-managed-clickhouse).

Four hardware tiers, all Hetzner CCX (dedicated vCPU) so neighbor noise doesn't
pollute the latency signal:

| Tier  | vCPU | RAM   | Disk        |
|-------|------|-------|-------------|
| CCX13 | 2    | 8 GB  | 80 GB NVMe  |
| CCX23 | 4    | 16 GB | 160 GB NVMe |
| CCX33 | 8    | 32 GB | 240 GB NVMe |
| CCX43 | 16   | 64 GB | 360 GB NVMe |

## Two scenarios

Two distinct questions; one matrix-entry script answers either depending on
the `--scenario` flag.

- **`throughput` (default).** A three-phase ingest ramp (Phase 1: batch
  size; Phase 2: collector-shape rate; Phase 3: SDK-fleet-shape rate).
  Results land in `benchmarks/results-throughput/`. Answers "how fast can
  the SUT swallow OTLP data without erroring or rejecting?"
- **`read-probe`.** Fills the table to a sequence of row counts
  (`100k, 1M, 10M, 100M`) and probes one dashboard read at each level,
  failing the step when the read exceeds `--read-threshold-ms` (default
  5 s). Results land in `benchmarks/results-probe/`. Answers "how big
  can the table grow before the dashboard read on this endpoint cliffs?"

The two scenarios write to sibling folders so one never overwrites the
other. Each folder is wiped on each dispatch of its scenario.

Don't read the throughput number as "the dashboard stays usable at N
items/sec" — that's what read-probe is for.

## Two insert paths: sync vs async ClickHouse

Every backend `PrepareBatch` call funnels through `chdb.BatchCtx()`, which
reads `CH_ASYNC_INSERT` once at startup. **This is a per-dispatch toggle**
— a single matrix entry runs one insert mode; comparison requires two
dispatches.

|                          | `CH_ASYNC_INSERT=0` (default) | `CH_ASYNC_INSERT=1` |
|--------------------------|-------------------------------|----------------------|
| Driver flag              | `clickhouse.WithAsync(false)` | `clickhouse.WithAsync(true)` |
| Part-per-request         | 1 part per insert (worst case for MergeTree) | Batched server-side, parts amortised |
| What it measures         | Pessimistic ceiling — what you get if you point a stock OTel collector at the default Traceway config | Realistic production ceiling — what a tuned deployment gets |
| Result filename          | `<tier>-<mode>-<signal>-<scenario>.json` | `<tier>-<mode>-<signal>-<scenario>-async.json` |
| Headline bar in charts   | Solid | Solid (sibling bar in chart-headline-<signal>.png when both passes are present) |

Quote both numbers when reporting. The sync number tells you the floor your
users hit before tuning; the async number tells you the headroom.

**Triggering async-insert locally:**

```bash
# Default (sync) — no env var needed
./benchmarks/scripts/run-local.sh --tier ccx13 --mode pgch --signal spans

# Async — set CH_ASYNC_INSERT=1 in the parent env. run-matrix-entry.sh
# propagates it via the 7th positional arg "async" to sut-bootstrap.sh,
# which exports it into the SUT's docker-compose env so chdb.BatchCtx()
# reads the right value at startup.
CH_ASYNC_INSERT=1 ./benchmarks/scripts/run-local.sh --tier ccx13 --mode pgch --signal spans --async
```

`run-local.sh --async` is sugar that just sets `CH_ASYNC_INSERT=1` and
threads the `async` arg into `run-matrix-entry.sh`. Direct script call:

```bash
./benchmarks/scripts/run-matrix-entry.sh ccx13 pgch spans 30m benchmarks/results-throughput "" async
#                                         ^tier ^mode ^sig ^dur ^out-dir            ^smoke ^async
```

**Triggering async-insert in CI:**

In the GitHub Actions `workflow_dispatch` form, flip the `async_insert`
boolean to `true`. The workflow propagates it through to
`run-matrix-entry.sh` and tags both the artifact and the committed JSON
filenames with `-async`. Dispatch twice (once off, once on) and the
aggregate job's chart renderer paints sibling bars in
`chart-headline-<signal>.png`.

`managed-ch` mode also honours `CH_ASYNC_INSERT` — useful for measuring
how much of the managed offering's headroom is currently being left on
the table by the conservative sync default.

## How a run works

Per matrix entry (one tier × one mode × one signal):

1. `hetzner-up.sh` provisions a SUT box + a CAX11 loadgen box on a private network (override with `LOADGEN_TIER`).
2. `sut-bootstrap.sh` rsyncs the repo, installs Docker, brings up the right
   `docker-compose.<mode>.yml`, waits for `/health`.
3. `seed-project.sh` registers a fresh user + project, captures the project
   bearer token.
4. `loadgen-bootstrap.sh` cross-compiles the loadgen, pushes it to the
   loadgen box, runs it with `--signal <spans|metrics|logs>` against the
   SUT's *private* IP.
5. The loadgen runs a three-phase ramp. After every step it GETs
   `/api/health/deep` and embeds a `ch` block in the step's JSON
   (`partsCount`, `partsByTable`, `activeMerges`, `longestMergeSec`,
   `errorsRecent`, `memoryUsageBytes`, `uptimeSec`). This is how the
   bench sees ClickHouse-side pressure — the HTTP-only signal alone
   can't distinguish "SUT cliffed on CPU" from "MergeTree threw
   `Too many parts`".
   - **Phase 1 — batch-size ramp.** Single client at a fixed 5 req/sec.
     Batch sizes step through `256,1024,4096,8192,16384`. Each step holds for
     `--step-duration` (default 2 min). Stops at the first failing step.
   - **Inter-phase merge-idle wait.** Phase 1's last step typically runs the
     SUT near saturation; CH merge horizons are minutes, not seconds. The
     loadgen polls `/api/health/deep` every 5 s until `activeMerges == 0`
     and `partsCount` is stable (within ±5%) for two consecutive polls, or
     `--max-merge-idle-wait` (default 5 m) expires. The old fixed
     `--inter-phase-cooldown-seconds` flag still works (additive pre-wait)
     but defaults to 0 — it's deprecated in favour of polling CH directly.
     Without this gate, pgch runs reliably produced "0 OK / 0 errors"
     Phase 2 because new requests sat on the SUT-side TCP backlog while
     CH was still merging Phase 1's parts.
   - **Phase 2 — request-rate ramp (collector shape).** Batch size fixed at
     `min(Phase 1 winner, --phase2-batch-cap=16384)`. Request rates step
     through `1,5,25,100,400`. When the coarse ramp finds the first failing
     step, up to `--phase2-bisect-max-steps` (default 3) bisection steps run
     between the last passing rate and the failing rate to pin the cliff
     within `--phase2-bisect-tolerance` (default 20%). So `5→25` (a 5× jump)
     gets refined into something like `5, 15, 10, 12` until the gap is
     <20% of the last passing rate.
   - **Merge-idle wait** before Phase 3 too.
   - **Phase 3 — request-rate ramp (SDK-fleet shape).** Batch size fixed at
     `--phase3-batch-size` (default **100**, matching typical language-SDK
     `BatchSpanProcessor` output rather than the collector's 8192). Request
     rates step through `--phase3-request-rates` (default
     `10,100,1000,5000,10000` — much higher than Phase 2 because each
     request is much cheaper at batch=100). Same bisection logic as Phase 2.
     Measures the SUT under "thousands of small batches/sec from a real
     SDK fleet" load, which stresses the HTTP/auth/decode/queue path more
     than the raw insert path Phase 2 measures.
6. A step "fails" when **either**:
   - combined error rate (HTTP failures + OTLP `PartialSuccess` rejected items)
     exceeds `--ingest-err-threshold` (default 5%), **or**
   - the *achieved* request rate falls below `--soft-cliff-ratio` × target rate
     (default 70%) — meaning the workers can't keep up with the limiter, the
     SUT has cliffed on latency, and we'd be erroring out one step later anyway.

   The headline `maxSustainableItemsPerSec` is the highest `actualItemsPerSec`
   recorded across passing steps from **any** of Phase 1, Phase 2, or Phase 3
   — measured from real OK responses, not the formula `batchSize × targetRate`
   (which over-reports when workers saturate before the limiter does).
   Different phases probe different shapes (collector fat-batch vs SDK
   small-batch fleet) and the SUT's best-shape ceiling is what the headline
   reports; per-phase numbers stay in the JSON for shape-specific analysis.
7. `hetzner-down.sh` deletes everything via a bash `trap` — even on Ctrl-C.

### SUT crash recovery & restart detection

`pgch` and `managed-ch` containers declare `restart: on-failure:3` in their
compose files. When ClickHouse OOMs under Phase 1's heaviest step (typical
on `ccx33` + `pgch` once Phase 2 climbs past `batch=16384 × rate=100`), Docker
auto-restarts the crashed service within seconds. After each merge-idle
wait the loadgen polls `GET <target>/health` for up to
`--sut-health-timeout-seconds` (default 60 s) to confirm the SUT actually
came back. If `/health` never returns 200, the loadgen skips the remaining
phases and writes the partial JSON via the existing per-step checkpoint —
no data already collected is lost. The bounded `on-failure:3` retry count
prevents a permanently-broken SUT from restart-looping forever; if the
third restart also crashes, the container stays down and the health-poll
fires the clean-abort path.

**Detecting silent CH restarts.** Compose's `restart` policy used to mean
a crashed-and-recovered ClickHouse was invisible: the SUT came back up and
the bench kept running, producing a "passing" report that hid the
incident. The bench now tracks CH's `uptime()` across every step. If
step N's uptime is lower than step N-1's uptime plus the elapsed step
window, ClickHouse restarted mid-step. The step is marked failed with
`failReason: CH restarted mid-step (...)`, `chRestarted: true` is set on
the top-level JSON, the run skips remaining phases, and the loadgen
exits non-zero. The chart renderer hatches the affected headline bar
in red with a `CH↻` annotation so the dispatch report flags the run as
invalid rather than burying the restart in a passing number.

After all matrix entries finish, `chart.py` renders the full chart suite
(headline bars, Phase 1 / 2 / 3 ramps, Pareto, tier scaling, cliff grid,
batch efficiency, signal mix — plus the read-probe equivalents in the
read-probe folder) and a combined `summary.md`. See
[charts.md](charts.md) for a guide to reading each chart.

## Running from your laptop

### One-time setup

1. **Install tooling**: `hcloud`, `jq`, `python3`, Go 1.25+, `rsync`. On macOS:
   `brew install hcloud jq rsync go`.
2. **Install matplotlib in a venv** (system Python is usually PEP 668 locked):
   ```bash
   python3 -m venv .venv
   source .venv/bin/activate
   pip install matplotlib
   ```
3. **Generate an SSH key** specifically for benchmarks and upload its public
   half to Hetzner under the name `benchmark-key`:
   ```bash
   ssh-keygen -t ed25519 -f ~/.ssh/hetzner_benchmark -C benchmark-key
   chmod 600 ~/.ssh/hetzner_benchmark
   hcloud ssh-key create --name benchmark-key --public-key-from-file ~/.ssh/hetzner_benchmark.pub
   ```
4. **Export creds** (use `direnv` or a sourced `.envrc.local`, never commit):
   ```bash
   export HCLOUD_TOKEN=...
   export BENCHMARK_SSH_KEY=~/.ssh/hetzner_benchmark
   ```

### Smoke (cheap — ~5 min, ~€0.02)

```bash
./benchmarks/scripts/run-local.sh --smoke
```

One tier (ccx13), one mode (sqlite), one signal (spans), short steps. If this
works, the full matrix works.

### Full matrix (~3 h, ~€3.60)

```bash
./benchmarks/scripts/run-local.sh
```

4 tiers × 2 modes × 3 signals, default `--duration 30m`. Output goes to
`benchmarks/results-throughput/` (or `benchmarks/results-probe/` when
running `--scenario read-probe`), which is wiped at the start of every run
so each dispatch's output stands alone — no silent cross-run mixing that
the old date-folder layout produced. The two scenario folders are
independent: a read-probe run never touches `results-throughput/` and
vice-versa.

### Other useful invocations

```bash
# Validate environment without provisioning anything (free)
./benchmarks/scripts/run-local.sh --dry-run

# Re-run just one tier/mode across all signals
./benchmarks/scripts/run-local.sh --tier ccx23 --mode pgch

# One signal only across all tiers/modes
./benchmarks/scripts/run-local.sh --signal spans

# A single matrix cell
./benchmarks/scripts/run-local.sh --tier ccx13 --mode sqlite --signal logs

# Override the per-entry runtime
./benchmarks/scripts/run-local.sh --tier ccx13 --duration 10m

# Switch to the read-probe scenario (writes to benchmarks/results-probe/)
./benchmarks/scripts/run-local.sh --scenario read-probe

# Async-insert pass — same matrix but with CH_ASYNC_INSERT=1 on the SUT.
# Output filenames pick up a -async suffix so paired bars render in
# chart-headline-<signal>.png. Dispatch the default (sync) pass first,
# then this one, into the same results folder.
./benchmarks/scripts/run-local.sh --tier ccx13 --mode pgch --signal spans --async
```

## Running from GitHub Actions

`.github/workflows/benchmark-hardware.yml`, `workflow_dispatch` only. Inputs
mirror the local flags. The workflow YAML is a thin wrapper around the same
`run-matrix-entry.sh` script the local path uses — if it works locally, it
works in CI.

Required GitHub secrets:
- `HCLOUD_TOKEN`
- `BENCHMARK_SSH_PRIVATE_KEY` — the private key matching the Hetzner-side
  `benchmark-key`.

After the matrix completes, an `aggregate` job downloads all artifacts, runs
`chart.py`, and commits the matching scenario folder
(`benchmarks/results-throughput/` for throughput runs,
`benchmarks/results-probe/` for read-probe runs) to `main` via a bot commit
(`git add -A`, so files from a prior dispatch that aren't in this one get
staged for deletion). No PR — it's a generated artifact. **Each scenario
folder always reflects exactly one dispatch of that scenario**; a
throughput dispatch never modifies `results-probe/` and vice-versa.
Comparing to previous runs is `git log -- benchmarks/results-throughput/`
or `git show <sha>:benchmarks/results-throughput/summary.md`. Dated
historical folders (`benchmarks/results/2026-05-15/` etc.) from before
this layout change are kept in git for reference but are no longer written
to.

## Running against managed ClickHouse

Setting `modes=managed-ch` in the workflow dispatch (or `--mode managed-ch`
locally) points the SUT's Traceway container at an externally-hosted
ClickHouse. Postgres still runs locally in the SUT's Docker — this benchmark
is about ClickHouse characteristics.

Required GitHub repository secrets (Settings → Secrets and variables → Actions):

| Secret | Example | Notes |
|---|---|---|
| `BENCH_CH_SERVER` | `your-cluster.us-east-1.aws.clickhouse.cloud:9440` | Native TCP endpoint with TLS port (usually `9440`) |
| `BENCH_CH_USERNAME` | `default` | A dedicated bench user is wiser than `default` |
| `BENCH_CH_PASSWORD` | `••••••` | |
| `BENCH_CH_DATABASE` | `traceway` | Optional, defaults to `traceway` |
| `BENCH_CH_HTTPS_PORT` | `8443` | Optional, defaults to `8443` (CH Cloud); some hosts use `8123` for plain HTTP |

The bench user needs `DROP DATABASE` and `CREATE DATABASE` privileges — between
every matrix entry the orchestrator runs `reset-managed-ch.sh`, which wipes and
recreates the bench database via the HTTPS interface so each (tier × signal ×
scenario) cell starts on an empty cluster. ~5–10s of overhead per entry. If
you're running on a shared cluster, **point the bench at a dedicated database**
or you will lose other data.

Locally: export the same vars (`CLICKHOUSE_SERVER`, `CLICKHOUSE_USERNAME`,
`CLICKHOUSE_PASSWORD`, optional `CLICKHOUSE_DATABASE`, `BENCH_CH_HTTPS_PORT`)
before invoking `run-local.sh --mode managed-ch`. Preflight will fail early if
they're missing.

### Caveats specific to managed CH

- **Network RTT dominates.** Hetzner `nbg1` to ClickHouse Cloud `us-east-1` is
  ~100ms each way; that's a wall the cluster can't climb. Match regions
  (`nbg1`/`fsn1`/`hel1` → CH Cloud EU; `ash`/`hil` → CH Cloud US) for numbers
  comparable to local-CH `pgch`.
- **Bandwidth matters too.** At 8192-span gzipped OTLP batches × 400 req/sec
  you push ~30–80 MB/s outbound from the SUT. Hetzner's egress is generous;
  the managed cluster's ingress quota is the more likely cap.
- **Read-probe is the more interesting scenario on managed CH.** Throughput
  often gets bottlenecked on the SUT→cluster pipe before the cluster's actual
  ingest path. Read-probe surfaces the cluster's read scaling, which is what
  you actually buy from a managed offering.

## Symbolicator benchmarks

Two additional workflows benchmark the JS symbolicator. They answer different
questions and run on different infrastructure:

| Workflow | What it measures | Where it runs | Cost |
|---|---|---|---|
| `benchmark-symbolicator` | Bundle parser throughput: goja vs oxc (`go test -bench`) | GitHub runner | free, ~10 min |
| `benchmark-symbolicator-cache` | Resolver cache architecture: in-memory LRU vs mmap disk cache | One Hetzner box | under 1 EUR, ~2 h |

Both are `workflow_dispatch` only. The workflow file must exist on `main` to be
dispatchable; select the feature branch in the run dropdown to execute that
branch's scripts and code.

### Parser benchmark

`scripts/benchmark-symbolicator.sh` (repo root `scripts/`, not this folder)
builds the oxc Rust shim, runs the symbolicator test suite as a gate, then runs
`BenchmarkBundleParsers` over the bundler fixtures (simple, inlining, webpack,
metro, preact) plus 1MB/5MB synthetic bundles, with both parsers. Results land
in the job step summary. Expected shape: oxc 2-4.5x faster than goja with
roughly 99% fewer allocations; `BenchmarkOpenTW` in the hundreds of
nanoseconds.

### Cache benchmark (cachebench)

> With a corpus of **N** uploaded bundles (more than RAM can hold), a hot set
> of ~30 active exceptions, and **R**% of lookups hitting the long tail, which
> resolver cache is cheaper: parsed-in-memory or mmap'd `.tw` files on disk,
> and at what corpus size does one break?

The harness is `backend/tools/cachebench` (it imports backend internals, so it
lives in the backend module): `generate` writes a corpus of synthetic
bundle + VLQ source map + `.tw` triples, `run` executes one cell
(mode x entries x cold-ratio) in a fresh process and emits a JSON row with
throughput (rps), weighted p50/p95/p99 latency, peak RSS sampled at 200ms, GC
totals, and cache internals (hits, builds, disk hits, store hits, evictions),
`table` merges the rows into a markdown summary that marks the crossover
corpus size and any OOM break point.

Run it three ways:

```bash
# Locally, no Hetzner (small corpus; RSS reads 0 on macOS, Linux is the real measurement)
./benchmarks/scripts/cachebench-local.sh

# From a laptop against Hetzner (same env vars as the hardware benchmark)
export HCLOUD_TOKEN=... BENCHMARK_SSH_KEY=~/.ssh/benchmark-key
SMOKE=1 ./benchmarks/scripts/cachebench-entry.sh   # ~5 min pipeline check
./benchmarks/scripts/cachebench-entry.sh           # full sweep

# From GitHub Actions: dispatch benchmark-symbolicator-cache, smoke=true first
```

Defaults: ccx13 (2 vCPU / 8 GB), corpus 1000 to 12000 bundles (~1 MB resolver
each against a 5.6 GB bounded memory budget, 70% of RAM), cold ratios
0 / 0.05 / 0.25, plus an unbounded in-memory variant per cell to find the OOM
break point. Each cell gets a wiped tw-cache and dropped page caches. A cell
killed by the OOM killer is recorded as `BREAK (OOM)` instead of failing the
sweep; ssh drops and timeouts are recorded as `ERROR`/`TIMEOUT` so they cannot
masquerade as cache conclusions.

What to expect in the summary table (one section per cold ratio):

- **Cold 0%:** memory wins or ties every row; with only the hot set in play
  nothing ever misses. Baseline sanity.
- **Cold 5% / 25%:** memory wins while the corpus fits its budget, then
  degrades once it does not (every cold lookup pulls the `.tw` artifact from
  storage into the heap, churning the LRU and the GC) while disk holds sub-ms
  p99 via mmap re-opens. The table prints the crossover row.
- **memory-unbounded:** RSS climbs with corpus size until `BREAK (OOM)`,
  producing the "file-based is the only option beyond here" line. Disk RSS
  stays flat (only hot pages resident).

Results are not committed back to the repo: every dispatch uploads
`benchmarks/results-cachebench/` (per-cell JSON plus `summary.md`) as a run
artifact, and the summary table is rendered in the job step summary.

## Layout

```
benchmarks/
  compose/                       # SUT-side docker compose, one per mode
    docker-compose.sqlite.yml
    docker-compose.pgch.yml
    docker-compose.managed-ch.yml  # External CH; Postgres still local
  loadgen/                       # The Go binary that generates load (OTLP-only)
    main.go                      # CLI + orchestration + merge-idle wait
    ingest.go                    # Worker pool driving the selected signal sender
    otlp_common.go               # Shared OTLP helpers + signalSender interface
    otlp_spans.go                # ExportTraceServiceRequest builder
    otlp_metrics.go              # ExportMetricsServiceRequest builder (Gauge)
    otlp_logs.go                 # ExportLogsServiceRequest builder
    ramp.go                      # Three-phase ramp + CH-restart detection
    ch_snapshot.go               # GET /api/health/deep -> per-step ch{} block
    stats.go                     # Latency tracker (percentiles via sort)
    util.go, log.go              # Misc helpers
  scripts/
    run-local.sh                 # ★ Laptop entry point
    run-matrix-entry.sh          # One matrix cycle (used by local + CI)
    preflight.sh                 # Validates env before any provisioning
    hetzner-up.sh                # hcloud server create
    hetzner-down.sh              # hcloud server delete (idempotent)
    sut-bootstrap.sh             # Installs Docker, brings up backend
    reset-managed-ch.sh          # Wipes the managed CH DB between matrix entries
    loadgen-bootstrap.sh         # Cross-compiles + runs loadgen
    seed-project.sh              # /api/register -> JWT + project token JSON
    chart.py                     # matplotlib renderer (throughput + read-probe charts)
    _ssh.sh                      # Shared ssh/rsync helpers + retry_eof
    cachebench-local.sh          # Symbolicator cache sweep on this machine
    cachebench-entry.sh          # Symbolicator cache sweep on one Hetzner box
    _cachebench.sh               # Shared matrix/stub helpers for the two above
  results-throughput/            # Committed throughput results (wiped per dispatch)
  results-probe/                 # Committed read-probe results (wiped per dispatch)
  results-cachebench/            # Symbolicator cache results; CI uploads these as a run artifact, never committed
  results/                       # Historical dated folders (not written to anymore)
  charts.md                      # Reading guide for every chart chart.py emits
```

## Failure modes & debugging

- **A run leaves Hetzner servers behind.** `hetzner-down.sh` is idempotent and
  callable directly: `./benchmarks/scripts/hetzner-down.sh <run-id>`. Run IDs
  appear in the orchestrator's stderr at the start of each matrix entry. Also
  visible in `hcloud server list` — anything tagged `bench=true` is safe to
  delete.
- **`preflight.sh` complains about `benchmark-key`.** The Hetzner-side SSH key
  resource is named `benchmark-key` regardless of what you named the file
  locally. Re-upload via `hcloud ssh-key create --name benchmark-key
  --public-key-from-file <your-pub-key>`.
- **Docker compose build is slow on tiny tiers.** First-time `docker compose
  up --build` on a CCX13 takes 5–8 minutes (npm install + Go build).
  `sut-bootstrap.sh` waits up to 10 minutes for `/health` before giving up.
  Subsequent runs reuse the Docker layer cache via the persistent volumes.
- **All steps pass on the highest tier.** Raise the upper end of
  `--phase2-request-rates` (default `1,5,25,100,400`) — e.g. add `1000,2000` —
  if you want to find the ceiling on big boxes. The default is meant to keep
  one-tier runs bounded.
