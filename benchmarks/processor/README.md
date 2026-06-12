# benchmarks/processor

Head-to-head benchmark of the Traceway source map symbolicator processor (oxc build)
against Honeycomb's reference sourcemapprocessor, measuring sustained symbolication
throughput and resident memory while ramping load to the breaking point.

## Topology

```
loadgen (Go) ----OTLP/HTTP----> collector under test ----OTLP/HTTP----> drain (Rust)
   |                            (one symbolicator,                        |
   |                             file store, no batch,                    |
   |                             sync export)                             |
   +---- ramps concurrency      RSS sampled every 1s        verifies the stacktrace
         until saturation                                   was actually symbolicated
```

The drain is a Rust server that gunzips each export request, scans for the
original-source marker (`../src/inventory.js`) and the minified marker (`.mjs:1:`),
and counts symbolicated vs unsymbolicated requests. It does no protobuf parsing,
so it sustains far more load than either collector can produce.

The export path is synchronous (no sending queue, no retry, no batch processor),
so one accepted loadgen request equals one fully symbolicated export delivered to
the drain. Loadgen ok-rate times spans-per-request is the end-to-end
stacktraces/sec; the drain's symbolicated percentage is the correctness check.

## Implementations

| impl | binary | parser | cache |
|------|--------|--------|-------|
| `honeycomb` | otelcol-bench-honeycomb | symbolic (cgo) | RAM LRU, entry-count bound |
| `traceway-oxc-mem` | otelcol-bench-traceway | oxc | in-memory resolvers only |
| `traceway-oxc-disk` | otelcol-bench-traceway | oxc | mmap'd `.tw` disk tier |
| `traceway-goja-mem` | otelcol-bench-traceway | goja | in-memory resolvers only |
| `traceway-goja-disk` | otelcol-bench-traceway | goja | mmap'd `.tw` disk tier |

One traceway binary (built with `-tags oxc`) serves all four variants; parser
and cache mode are runtime config (`parser`, `cache_dir`).

## Scenarios

- `hot`: one bundle, always cache-warm. Pure resolve throughput ramp.
- `churn`: 512 bundles (default) against a 128-entry resolver cache.
  Honeycomb re-parses through Sentry symbolic on every eviction; Traceway disk
  variants re-open compiled `.tw` files. The goja-vs-oxc choice matters here,
  since the parser only runs on cache misses.
- `oom`: 4096 bundles (default) with 1MB bundle padding, 1MB of
  sourcesContent padding, AND 1MB of synthetic VLQ mappings per map (so
  traceway's token table grows too and the corpus is realistic, not rigged), cache entry limit raised to corpus size,
  fixed 32-connection load until the corpus is fully resident or the collector
  dies. Honeycomb retains the raw map JSON plus the minified bundle on the C
  heap per entry, so RSS grows with corpus bytes. Traceway discards bundles and
  sourcesContent after compiling the compact `.tw` token table; the mem
  variants keep resolvers on the Go heap, the disk variants only mmap handles.
  The oom run defaults to a small SUT (ccx13, 8 GB) so the breaking point
  arrives quickly.

Corpus entries are the real minified node-app bundle padded with `--pad-kb`
of valid JS (default 256 KB) so scope-analysis parse cost is realistic. The
sourcemap stays valid because frames sit on line 1 before the padding.

## Memory methodology

RSS is sampled once per second from outside the process (`rss.csv` per run).
Go heap numbers would be misleading here: Honeycomb's parsed maps live on the
C heap inside symbolic (invisible to Go), and Traceway's mmap'd `.tw` pages
are kernel-reclaimable (inflate RSS but are evictable). Compare the full RSS
timeline, not a single number.

## Run locally

```
./run-local.sh
IMPLS="traceway-oxc traceway-goja honeycomb" SCENARIOS=churn CONNECTIONS=4,16,64 ./run-local.sh
```

Needs go, cargo, node, jq. Builds both collectors (the Traceway one with
`-tags oxc` after `scripts/build-oxc-shim.sh`), drain, loadgen, corpusgen,
then runs the matrix on localhost and prints a summary table. Results land
in `results/<impl>-<scenario>/` as `loadgen.json`, `drain.json`, `rss.csv`,
`collector.log`.

## Run on Hetzner

```
export HCLOUD_TOKEN=...
./run-hetzner.sh traceway-oxc
./run-hetzner.sh honeycomb
```

Provisions two dedicated-vCPU servers per invocation (SUT default ccx33,
loadgen+drain box default ccx23, same location), pushes prebuilt linux
artifacts from `artifacts/`, runs the scenarios, pulls results, and deletes
the servers on exit. The loadgen box hosts the drain so the SUT runs nothing
but the collector.

## GitHub Action

`benchmark-processor` (workflow_dispatch) builds all artifacts on the runner,
then runs the two implementations as parallel matrix entries, each on its own
Hetzner server pair. Needs the `HCLOUD_TOKEN` secret. Results are uploaded as
`results-traceway-oxc` and `results-honeycomb` artifacts.

## Knobs

| Env | Default | Meaning |
|-----|---------|---------|
| `IMPLS` | `traceway-oxc honeycomb` | run-local.sh only; `traceway-goja` selects the goja parser in the same oxc-built binary |
| `SCENARIOS` | `hot churn` | |
| `CHURN_ENTRIES` | `512` | corpus size for churn |
| `PAD_KB` | `256` | padding per bundle |
| `CONNECTIONS` | ramp | comma list of concurrency steps |
| `STEP_DURATION` | `30s` local, `60s` hetzner | time per step |
| `SPANS_PER_REQUEST` | `20` | exception spans per OTLP request |
| `SUT_TYPE` / `LDG_TYPE` | `ccx33` / `ccx23` | hetzner server types |
