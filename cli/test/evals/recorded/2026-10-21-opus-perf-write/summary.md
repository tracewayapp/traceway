# Traceway evals

- **Run:** `opus-full-perf-write-20261021`, started by 15340338+dusanstanojeviccs@users.noreply.github.com
- **Commit:** `69ab7bb4f927`, skill `traceway` `52f484fb85`, knowledge `1503eb52f0`
- **Models:** astra=`openai/gpt-6-astra`, haiku=`anthropic/claude-haiku-5.5`, judge=`anthropic/claude-haiku-5.5`, opus=`anthropic/claude-opus-5.5`
- **Spend:** $39.90 (judge $0.13) of a $103.00 cap
  (including $1.83 spent on runs that a resume ran again)
- **Runs:** 131 (fail 41, pass 90)

## Pass rate by suite

| Suite | Model | Arm | Pass | Rate |
|---|---|---|---|---|
| perf | opus | cli+skill | 18/35 | 51% |
| perf | opus | cli-noskill | 19/35 | 54% |
| perf | opus | mcp | 31/35 | 89% |
| write | opus | cli-write+skill | 11/13 | 85% |
| write | opus | mcp-write | 11/13 | 85% |

## Skill uplift (cli+skill minus cli-noskill)

| Suite | Model | With skill | Without | Uplift |
|---|---|---|---|---|
| perf | opus | 51% | 54% | -3pp |

## CLI vs MCP

| Suite | Model | CLI arm | MCP arm | CLI | MCP | Delta |
|---|---|---|---|---|---|---|
| perf | opus | cli+skill | mcp | 51% | 89% | -37pp |
| write | opus | cli-write+skill | mcp-write | 85% | 85% | +0pp |

## Cost (averages over passing runs)

| Model | Passing runs | Avg cost | Avg tool calls | Avg input tokens | Avg output tokens | Cache hit rate | Total spend |
|---|---|---|---|---|---|---|---|
| opus | 90 | $0.281 | 12.7 | 227101 | 4368 | 88% | $37.94 |

| Suite / model | Spend |
|---|---|
| perf / opus | $34.23 |
| write / opus | $3.71 |

## Top failing graders

- trajectory: 30
- grounding: 8
- answer: 7
- report: 1

## Worst cases

| Case | Model | Arm | Pass |
|---|---|---|---|
| perf-app-excessive-logging | opus | cli+skill | 0/1 |
| perf-app-excessive-logging | opus | cli-noskill | 0/1 |
| perf-app-large-alloc | opus | cli+skill | 0/1 |
| perf-app-large-alloc | opus | cli-noskill | 0/1 |
| perf-cache-cold-start | opus | cli-noskill | 0/1 |
| perf-cache-no-cache | opus | cli+skill | 0/1 |
| perf-cache-stampede | opus | cli+skill | 0/1 |
| perf-cache-stampede | opus | cli-noskill | 0/1 |
| perf-ctl-alert-handoff | opus | mcp | 0/1 |
| perf-ctl-healthy | opus | cli-noskill | 0/1 |
| perf-ctl-marked-slow-regressed | opus | cli-noskill | 0/1 |
| perf-ctl-onset-bisect | opus | cli-noskill | 0/1 |
| perf-db-expensive-join | opus | cli+skill | 0/1 |
| perf-db-expensive-join | opus | cli-noskill | 0/1 |
| perf-db-lock-contention | opus | cli+skill | 0/1 |

## Runs

| Case | Arm | Model | Run | Status | Calls | Cost | First failure | Transcript |
|---|---|---|---|---|---|---|---|---|
| perf-app-cpu-serialization | cli+skill | opus | 1 | pass | 18 | $0.376 |  | runs/perf-app-cpu-serialization/cli_skill/opus/r1/transcript.jsonl |
| perf-app-cpu-serialization | cli-noskill | opus | 1 | pass | 18 | $0.262 |  | runs/perf-app-cpu-serialization/cli-noskill/opus/r1/transcript.jsonl |
| perf-app-cpu-serialization | mcp | opus | 1 | pass | 12 | $0.252 |  | runs/perf-app-cpu-serialization/mcp/opus/r1/transcript.jsonl |
| perf-app-excessive-logging | cli+skill | opus | 1 | fail | 19 | $0.399 | answer: locus.service "", want "listings-api" | runs/perf-app-excessive-logging/cli_skill/opus/r1/transcript.jsonl |
| perf-app-excessive-logging | cli-noskill | opus | 1 | fail | 20 | $0.332 | trajectory: the same call ran 4 times (max_identical_repeats 2): endpoints.chart {"interval_minutes":"240","since":"72h"} | runs/perf-app-excessive-logging/cli-noskill/opus/r1/transcript.jsonl |
| perf-app-excessive-logging | mcp | opus | 1 | pass | 17 | $0.399 |  | runs/perf-app-excessive-logging/mcp/opus/r1/transcript.jsonl |
| perf-app-large-alloc | cli+skill | opus | 1 | fail | 12 | $0.370 | trajectory: the same call ran 3 times (max_identical_repeats 2): metrics.query {"aggregation":"max","interval_minutes":"60","name":"jvm.gc.duration.avg"... | runs/perf-app-large-alloc/cli_skill/opus/r1/transcript.jsonl |
| perf-app-large-alloc | cli-noskill | opus | 1 | fail | 24 | $0.409 | trajectory: the same call ran 3 times (max_identical_repeats 2): traces.show {"id":"740f369c11dce2bdaa6b7ac5b86aaa45","recorded_at":"2026-10-20T19:56:07... | runs/perf-app-large-alloc/cli-noskill/opus/r1/transcript.jsonl |
| perf-app-large-alloc | mcp | opus | 1 | pass | 14 | $0.306 |  | runs/perf-app-large-alloc/mcp/opus/r1/transcript.jsonl |
| perf-app-quadratic | cli+skill | opus | 1 | pass | 11 | $0.265 |  | runs/perf-app-quadratic/cli_skill/opus/r1/transcript.jsonl |
| perf-app-quadratic | cli-noskill | opus | 1 | pass | 12 | $0.195 |  | runs/perf-app-quadratic/cli-noskill/opus/r1/transcript.jsonl |
| perf-app-quadratic | mcp | opus | 1 | pass | 13 | $0.286 |  | runs/perf-app-quadratic/mcp/opus/r1/transcript.jsonl |
| perf-app-sync-background | cli+skill | opus | 1 | pass | 11 | $0.317 |  | runs/perf-app-sync-background/cli_skill/opus/r1/transcript.jsonl |
| perf-app-sync-background | cli-noskill | opus | 1 | pass | 16 | $0.308 |  | runs/perf-app-sync-background/cli-noskill/opus/r1/transcript.jsonl |
| perf-app-sync-background | mcp | opus | 1 | pass | 16 | $0.371 |  | runs/perf-app-sync-background/mcp/opus/r1/transcript.jsonl |
| perf-cache-cold-start | cli+skill | opus | 1 | pass | 15 | $0.398 |  | runs/perf-cache-cold-start/cli_skill/opus/r1/transcript.jsonl |
| perf-cache-cold-start | cli-noskill | opus | 1 | fail | 23 | $0.332 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.chart {"interval_minutes":"15","metric_type":"p99","since":"30d"} | runs/perf-cache-cold-start/cli-noskill/opus/r1/transcript.jsonl |
| perf-cache-cold-start | mcp | opus | 1 | pass | 16 | $0.409 |  | runs/perf-cache-cold-start/mcp/opus/r1/transcript.jsonl |
| perf-cache-no-cache | cli+skill | opus | 1 | fail | 11 | $0.255 | grounding: evidence[0] cites c02, which failed | runs/perf-cache-no-cache/cli_skill/opus/r1/transcript.jsonl |
| perf-cache-no-cache | cli-noskill | opus | 1 | pass | 18 | $0.263 |  | runs/perf-cache-no-cache/cli-noskill/opus/r1/transcript.jsonl |
| perf-cache-no-cache | mcp | opus | 1 | pass | 13 | $0.242 |  | runs/perf-cache-no-cache/mcp/opus/r1/transcript.jsonl |
| perf-cache-stampede | cli+skill | opus | 1 | fail | 15 | $0.323 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.chart {"interval_minutes":"1","metric_type":"p99","since":"1h"} | runs/perf-cache-stampede/cli_skill/opus/r1/transcript.jsonl |
| perf-cache-stampede | cli-noskill | opus | 1 | fail | 20 | $0.318 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.chart {"interval_minutes":"1","metric_type":"p99","since":"1h"} | runs/perf-cache-stampede/cli-noskill/opus/r1/transcript.jsonl |
| perf-cache-stampede | mcp | opus | 1 | pass | 9 | $0.307 |  | runs/perf-cache-stampede/mcp/opus/r1/transcript.jsonl |
| perf-ctl-alert-handoff | cli+skill | opus | 1 | pass | 18 | $0.447 |  | runs/perf-ctl-alert-handoff/cli_skill/opus/r1/transcript.jsonl |
| perf-ctl-alert-handoff | cli-noskill | opus | 1 | pass | 23 | $0.366 |  | runs/perf-ctl-alert-handoff/cli-noskill/opus/r1/transcript.jsonl |
| perf-ctl-alert-handoff | mcp | opus | 1 | fail | 18 | $0.438 | trajectory: the same call ran 3 times (max_identical_repeats 2): git {"subcommand":"show"} | runs/perf-ctl-alert-handoff/mcp/opus/r1/transcript.jsonl |
| perf-ctl-healthy | cli+skill | opus | 1 | pass | 7 | $0.193 |  | runs/perf-ctl-healthy/cli_skill/opus/r1/transcript.jsonl |
| perf-ctl-healthy | cli-noskill | opus | 1 | fail | 12 | $0.161 | trajectory: the same call ran 7 times (max_identical_repeats 2): endpoints.list {"from":"--to","search":"/api/search"} | runs/perf-ctl-healthy/cli-noskill/opus/r1/transcript.jsonl |
| perf-ctl-healthy | mcp | opus | 1 | pass | 9 | $0.191 |  | runs/perf-ctl-healthy/mcp/opus/r1/transcript.jsonl |
| perf-ctl-marked-slow-ok | cli+skill | opus | 1 | pass | 3 | $0.143 |  | runs/perf-ctl-marked-slow-ok/cli_skill/opus/r1/transcript.jsonl |
| perf-ctl-marked-slow-ok | cli-noskill | opus | 1 | pass | 11 | $0.141 |  | runs/perf-ctl-marked-slow-ok/cli-noskill/opus/r1/transcript.jsonl |
| perf-ctl-marked-slow-ok | mcp | opus | 1 | pass | 6 | $0.137 |  | runs/perf-ctl-marked-slow-ok/mcp/opus/r1/transcript.jsonl |
| perf-ctl-marked-slow-regressed | cli+skill | opus | 1 | pass | 13 | $0.330 |  | runs/perf-ctl-marked-slow-regressed/cli_skill/opus/r1/transcript.jsonl |
| perf-ctl-marked-slow-regressed | cli-noskill | opus | 1 | fail | 20 | $0.286 | trajectory: the same call ran 15 times (max_identical_repeats 2): endpoints.list {"from":"--to","search":"exports"} | runs/perf-ctl-marked-slow-regressed/cli-noskill/opus/r1/transcript.jsonl |
| perf-ctl-marked-slow-regressed | mcp | opus | 1 | pass | 11 | $0.498 |  | runs/perf-ctl-marked-slow-regressed/mcp/opus/r1/transcript.jsonl |
| perf-ctl-metric-p95-trap | cli+skill | opus | 1 | pass | 15 | $0.354 |  | runs/perf-ctl-metric-p95-trap/cli_skill/opus/r1/transcript.jsonl |
| perf-ctl-metric-p95-trap | cli-noskill | opus | 1 | pass | 11 | $0.174 |  | runs/perf-ctl-metric-p95-trap/cli-noskill/opus/r1/transcript.jsonl |
| perf-ctl-metric-p95-trap | mcp | opus | 1 | pass | 14 | $0.522 |  | runs/perf-ctl-metric-p95-trap/mcp/opus/r1/transcript.jsonl |
| perf-ctl-onset-bisect | cli+skill | opus | 1 | pass | 18 | $0.377 |  | runs/perf-ctl-onset-bisect/cli_skill/opus/r1/transcript.jsonl |
| perf-ctl-onset-bisect | cli-noskill | opus | 1 | fail | 21 | $0.267 | grounding: evidence[1] cites c07, which failed | runs/perf-ctl-onset-bisect/cli-noskill/opus/r1/transcript.jsonl |
| perf-ctl-onset-bisect | mcp | opus | 1 | pass | 23 | $0.393 |  | runs/perf-ctl-onset-bisect/mcp/opus/r1/transcript.jsonl |
| perf-db-expensive-join | cli+skill | opus | 1 | fail | 8 | $0.231 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.chart {"interval_minutes":"180","metric_type":"p95","since":"24h"} | runs/perf-db-expensive-join/cli_skill/opus/r1/transcript.jsonl |
| perf-db-expensive-join | cli-noskill | opus | 1 | fail | 13 | $0.205 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.chart {"interval_minutes":"720","since":"7d"} | runs/perf-db-expensive-join/cli-noskill/opus/r1/transcript.jsonl |
| perf-db-expensive-join | mcp | opus | 1 | pass | 8 | $0.206 |  | runs/perf-db-expensive-join/mcp/opus/r1/transcript.jsonl |
| perf-db-lock-contention | cli+skill | opus | 1 | fail | 16 | $0.349 | trajectory: the same call ran 3 times (max_identical_repeats 2): logs.query {"min_severity":"13","page_size":"100","service_name":"inventory-api","since... | runs/perf-db-lock-contention/cli_skill/opus/r1/transcript.jsonl |
| perf-db-lock-contention | cli-noskill | opus | 1 | pass | 16 | $0.320 |  | runs/perf-db-lock-contention/cli-noskill/opus/r1/transcript.jsonl |
| perf-db-lock-contention | mcp | opus | 1 | pass | 15 | $0.348 |  | runs/perf-db-lock-contention/mcp/opus/r1/transcript.jsonl |
| perf-db-missing-index | cli+skill | opus | 1 | pass | 11 | $0.268 |  | runs/perf-db-missing-index/cli_skill/opus/r1/transcript.jsonl |
| perf-db-missing-index | cli-noskill | opus | 1 | pass | 16 | $0.234 |  | runs/perf-db-missing-index/cli-noskill/opus/r1/transcript.jsonl |
| perf-db-missing-index | mcp | opus | 1 | pass | 13 | $0.260 |  | runs/perf-db-missing-index/mcp/opus/r1/transcript.jsonl |
| perf-db-missing-query-cache | cli+skill | opus | 1 | fail | 10 | $0.246 | answer: locus.endpoint "GET /api/me (and every other route)", want "GET /api/me" | runs/perf-db-missing-query-cache/cli_skill/opus/r1/transcript.jsonl |
| perf-db-missing-query-cache | cli-noskill | opus | 1 | fail | 12 | $0.158 | answer: locus.endpoint "GET /api/me (and every other route via the global APP_GUARD)", want "GET /api/me" | runs/perf-db-missing-query-cache/cli-noskill/opus/r1/transcript.jsonl |
| perf-db-missing-query-cache | mcp | opus | 1 | fail | 16 | $0.283 | answer: locus.endpoint "GET /api/me (and every route, via global APP_GUARD)", want "GET /api/me" | runs/perf-db-missing-query-cache/mcp/opus/r1/transcript.jsonl |
| perf-db-n-plus-1 | cli+skill | opus | 1 | fail | 11 | $0.272 | trajectory: required call missing: endpoints.show(recorded_at=present) | runs/perf-db-n-plus-1/cli_skill/opus/r1/transcript.jsonl |
| perf-db-n-plus-1 | cli-noskill | opus | 1 | fail | 14 | $0.205 | trajectory: required call missing: endpoints.show(recorded_at=present) | runs/perf-db-n-plus-1/cli-noskill/opus/r1/transcript.jsonl |
| perf-db-n-plus-1 | mcp | opus | 1 | pass | 14 | $0.384 |  | runs/perf-db-n-plus-1/mcp/opus/r1/transcript.jsonl |
| perf-db-over-fetching | cli+skill | opus | 1 | fail | 17 | $0.345 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.chart {"interval_minutes":"720","metric_type":"p50","since":"14d"} | runs/perf-db-over-fetching/cli_skill/opus/r1/transcript.jsonl |
| perf-db-over-fetching | cli-noskill | opus | 1 | pass | 17 | $0.222 |  | runs/perf-db-over-fetching/cli-noskill/opus/r1/transcript.jsonl |
| perf-db-over-fetching | mcp | opus | 1 | pass | 10 | $0.285 |  | runs/perf-db-over-fetching/mcp/opus/r1/transcript.jsonl |
| perf-db-pool-exhaustion | cli+skill | opus | 1 | fail | 11 | $0.282 | trajectory: the same call ran 4 times (max_identical_repeats 2): endpoints.chart {"interval_minutes":"60","metric_type":"p95","since":"24h"} | runs/perf-db-pool-exhaustion/cli_skill/opus/r1/transcript.jsonl |
| perf-db-pool-exhaustion | cli-noskill | opus | 1 | pass | 13 | $0.247 |  | runs/perf-db-pool-exhaustion/cli-noskill/opus/r1/transcript.jsonl |
| perf-db-pool-exhaustion | mcp | opus | 1 | pass | 8 | $0.220 |  | runs/perf-db-pool-exhaustion/mcp/opus/r1/transcript.jsonl |
| perf-db-unbounded-result | cli+skill | opus | 1 | fail | 12 | $0.262 | trajectory: none of these alternatives happened: [endpoints.show(recorded_at=present) + endpoints.chart] or [traces.show(recorded_at=present) + endpoint... | runs/perf-db-unbounded-result/cli_skill/opus/r1/transcript.jsonl |
| perf-db-unbounded-result | cli-noskill | opus | 1 | pass | 15 | $0.189 |  | runs/perf-db-unbounded-result/cli-noskill/opus/r1/transcript.jsonl |
| perf-db-unbounded-result | mcp | opus | 1 | pass | 12 | $0.236 |  | runs/perf-db-unbounded-result/mcp/opus/r1/transcript.jsonl |
| perf-infra-cpu-saturation | cli+skill | opus | 1 | pass | 24 | $0.701 |  | runs/perf-infra-cpu-saturation/cli_skill/opus/r1/transcript.jsonl |
| perf-infra-cpu-saturation | cli-noskill | opus | 1 | fail | 27 | $0.492 | answer: checklist_item "connection_pool_exhaustion", want cpu_saturation | runs/perf-infra-cpu-saturation/cli-noskill/opus/r1/transcript.jsonl |
| perf-infra-cpu-saturation | mcp | opus | 1 | fail | 27 | $0.683 | trajectory: the same call ran 3 times (max_identical_repeats 2): git {"subcommand":"log"} | runs/perf-infra-cpu-saturation/mcp/opus/r1/transcript.jsonl |
| perf-infra-disk-io | cli+skill | opus | 1 | fail | 15 | $0.360 | answer: locus.host "", want "pg-bookings-1" | runs/perf-infra-disk-io/cli_skill/opus/r1/transcript.jsonl |
| perf-infra-disk-io | cli-noskill | opus | 1 | pass | 23 | $0.341 |  | runs/perf-infra-disk-io/cli-noskill/opus/r1/transcript.jsonl |
| perf-infra-disk-io | mcp | opus | 1 | pass | 22 | $0.463 |  | runs/perf-infra-disk-io/mcp/opus/r1/transcript.jsonl |
| perf-infra-memory-gc | cli+skill | opus | 1 | fail | 18 | $0.405 | trajectory: the same call ran 3 times (max_identical_repeats 2): metrics.query {"aggregation":"max","interval_minutes":"60","name":"mem.used","since":"2... | runs/perf-infra-memory-gc/cli_skill/opus/r1/transcript.jsonl |
| perf-infra-memory-gc | cli-noskill | opus | 1 | fail | 17 | $0.280 | answer: locus.service "", want "geo-api" | runs/perf-infra-memory-gc/cli-noskill/opus/r1/transcript.jsonl |
| perf-infra-memory-gc | mcp | opus | 1 | pass | 15 | $0.441 |  | runs/perf-infra-memory-gc/mcp/opus/r1/transcript.jsonl |
| perf-infra-noisy-neighbor | cli+skill | opus | 1 | pass | 16 | $0.380 |  | runs/perf-infra-noisy-neighbor/cli_skill/opus/r1/transcript.jsonl |
| perf-infra-noisy-neighbor | cli-noskill | opus | 1 | pass | 20 | $0.306 |  | runs/perf-infra-noisy-neighbor/cli-noskill/opus/r1/transcript.jsonl |
| perf-infra-noisy-neighbor | mcp | opus | 1 | pass | 18 | $0.426 |  | runs/perf-infra-noisy-neighbor/mcp/opus/r1/transcript.jsonl |
| perf-net-chatty-fanout | cli+skill | opus | 1 | fail | 17 | $0.400 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.list {"order_by":"p95","page_size":"10","since":"24h"} | runs/perf-net-chatty-fanout/cli_skill/opus/r1/transcript.jsonl |
| perf-net-chatty-fanout | cli-noskill | opus | 1 | fail | 15 | $0.228 | trajectory: the same call ran 4 times (max_identical_repeats 2): traces.show {"id":"263230d0e86862e5e201423c74ace408","recorded_at":"2026-10-21T04:32:45... | runs/perf-net-chatty-fanout/cli-noskill/opus/r1/transcript.jsonl |
| perf-net-chatty-fanout | mcp | opus | 1 | pass | 9 | $0.534 |  | runs/perf-net-chatty-fanout/mcp/opus/r1/transcript.jsonl |
| perf-net-large-payload | cli+skill | opus | 1 | pass | 11 | $0.285 |  | runs/perf-net-large-payload/cli_skill/opus/r1/transcript.jsonl |
| perf-net-large-payload | cli-noskill | opus | 1 | fail | 13 | $0.185 | grounding: evidence[4] cites c12, which failed | runs/perf-net-large-payload/cli-noskill/opus/r1/transcript.jsonl |
| perf-net-large-payload | mcp | opus | 1 | pass | 14 | $0.306 |  | runs/perf-net-large-payload/mcp/opus/r1/transcript.jsonl |
| perf-net-no-conn-reuse | cli+skill | opus | 1 | pass | 11 | $0.234 |  | runs/perf-net-no-conn-reuse/cli_skill/opus/r1/transcript.jsonl |
| perf-net-no-conn-reuse | cli-noskill | opus | 1 | fail | 18 | $0.268 | trajectory: 5 failed tool calls, more than max_failed_calls 4 | runs/perf-net-no-conn-reuse/cli-noskill/opus/r1/transcript.jsonl |
| perf-net-no-conn-reuse | mcp | opus | 1 | pass | 10 | $0.330 |  | runs/perf-net-no-conn-reuse/mcp/opus/r1/transcript.jsonl |
| perf-net-retries | cli+skill | opus | 1 | pass | 12 | $0.315 |  | runs/perf-net-retries/cli_skill/opus/r1/transcript.jsonl |
| perf-net-retries | cli-noskill | opus | 1 | pass | 20 | $0.266 |  | runs/perf-net-retries/cli-noskill/opus/r1/transcript.jsonl |
| perf-net-retries | mcp | opus | 1 | fail | 9 | $0.217 | trajectory: none of these alternatives happened: [traces.show(recorded_at=present)] or [endpoints.show(recorded_at=present)] | runs/perf-net-retries/mcp/opus/r1/transcript.jsonl |
| perf-net-sequential | cli+skill | opus | 1 | pass | 12 | $0.295 |  | runs/perf-net-sequential/cli_skill/opus/r1/transcript.jsonl |
| perf-net-sequential | cli-noskill | opus | 1 | pass | 11 | $0.171 |  | runs/perf-net-sequential/cli-noskill/opus/r1/transcript.jsonl |
| perf-net-sequential | mcp | opus | 1 | pass | 9 | $0.355 |  | runs/perf-net-sequential/mcp/opus/r1/transcript.jsonl |
| perf-net-slow-third-party | cli+skill | opus | 1 | fail | 14 | $0.331 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.chart {"interval_minutes":"60","metric_type":"p99","since":"7d"} | runs/perf-net-slow-third-party/cli_skill/opus/r1/transcript.jsonl |
| perf-net-slow-third-party | cli-noskill | opus | 1 | pass | 25 | $0.350 |  | runs/perf-net-slow-third-party/cli-noskill/opus/r1/transcript.jsonl |
| perf-net-slow-third-party | mcp | opus | 1 | pass | 16 | $0.439 |  | runs/perf-net-slow-third-party/mcp/opus/r1/transcript.jsonl |
| perf-xs-android-timeouts | cli+skill | opus | 1 | fail | 24 | $0.515 | trajectory: the same call ran 5 times (max_identical_repeats 2): exceptions.show {"hash":"e57268eeaeb2c15f"} | runs/perf-xs-android-timeouts/cli_skill/opus/r1/transcript.jsonl |
| perf-xs-android-timeouts | cli-noskill | opus | 1 | fail | 19 | $0.419 | grounding: evidence[0] cites c03, whose output was truncated | runs/perf-xs-android-timeouts/cli-noskill/opus/r1/transcript.jsonl |
| perf-xs-android-timeouts | mcp | opus | 1 | pass | 15 | $0.462 |  | runs/perf-xs-android-timeouts/mcp/opus/r1/transcript.jsonl |
| perf-xs-frontend-timeouts | cli+skill | opus | 1 | pass | 15 | $0.322 |  | runs/perf-xs-frontend-timeouts/cli_skill/opus/r1/transcript.jsonl |
| perf-xs-frontend-timeouts | cli-noskill | opus | 1 | pass | 20 | $0.295 |  | runs/perf-xs-frontend-timeouts/cli-noskill/opus/r1/transcript.jsonl |
| perf-xs-frontend-timeouts | mcp | opus | 1 | pass | 23 | $0.423 |  | runs/perf-xs-frontend-timeouts/mcp/opus/r1/transcript.jsonl |
| perf-xs-ios-watchdog | cli+skill | opus | 1 | fail | 25 | $0.580 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.chart {"interval_minutes":"60","metric_type":"p95","project_id":"ab07bd54-1d6... | runs/perf-xs-ios-watchdog/cli_skill/opus/r1/transcript.jsonl |
| perf-xs-ios-watchdog | cli-noskill | opus | 1 | pass | 24 | $0.517 |  | runs/perf-xs-ios-watchdog/cli-noskill/opus/r1/transcript.jsonl |
| perf-xs-ios-watchdog | mcp | opus | 1 | pass | 25 | $0.539 |  | runs/perf-xs-ios-watchdog/mcp/opus/r1/transcript.jsonl |
| write-alert-disable | cli-write+skill | opus | 1 | pass | 4 | $0.078 |  | runs/write-alert-disable/cli-write_skill/opus/r1/transcript.jsonl |
| write-alert-disable | mcp-write | opus | 1 | pass | 3 | $0.103 |  | runs/write-alert-disable/mcp-write/opus/r1/transcript.jsonl |
| write-alert-snooze | cli-write+skill | opus | 1 | pass | 4 | $0.069 |  | runs/write-alert-snooze/cli-write_skill/opus/r1/transcript.jsonl |
| write-alert-snooze | mcp-write | opus | 1 | pass | 4 | $0.121 |  | runs/write-alert-snooze/mcp-write/opus/r1/transcript.jsonl |
| write-archive-ambiguous | cli-write+skill | opus | 1 | fail | 4 | $0.087 | trajectory: required call missing: ask_user | runs/write-archive-ambiguous/cli-write_skill/opus/r1/transcript.jsonl |
| write-archive-ambiguous | mcp-write | opus | 1 | fail | 4 | $0.133 | trajectory: required call missing: ask_user | runs/write-archive-ambiguous/mcp-write/opus/r1/transcript.jsonl |
| write-investigate-only | cli-write+skill | opus | 1 | fail | 11 | $0.198 | trajectory: 4 failed tool calls, more than max_failed_calls 3 | runs/write-investigate-only/cli-write_skill/opus/r1/transcript.jsonl |
| write-investigate-only | mcp-write | opus | 1 | pass | 10 | $0.203 |  | runs/write-investigate-only/mcp-write/opus/r1/transcript.jsonl |
| write-mark-slow | cli-write+skill | opus | 1 | pass | 5 | $0.079 |  | runs/write-mark-slow/cli-write_skill/opus/r1/transcript.jsonl |
| write-mark-slow | mcp-write | opus | 1 | pass | 6 | $0.132 |  | runs/write-mark-slow/mcp-write/opus/r1/transcript.jsonl |
| write-monitor-confirm-fix | cli-write+skill | opus | 1 | pass | 5 | $0.079 |  | runs/write-monitor-confirm-fix/cli-write_skill/opus/r1/transcript.jsonl |
| write-monitor-confirm-fix | mcp-write | opus | 1 | pass | 6 | $0.120 |  | runs/write-monitor-confirm-fix/mcp-write/opus/r1/transcript.jsonl |
| write-optin-off | cli-write+skill | opus | 1 | pass | 4 | $0.068 |  | runs/write-optin-off/cli-write_skill/opus/r1/transcript.jsonl |
| write-optin-off | mcp-write | opus | 1 | pass | 3 | $0.099 |  | runs/write-optin-off/mcp-write/opus/r1/transcript.jsonl |
| write-page-ack | cli-write+skill | opus | 1 | pass | 7 | $0.094 |  | runs/write-page-ack/cli-write_skill/opus/r1/transcript.jsonl |
| write-page-ack | mcp-write | opus | 1 | pass | 3 | $0.104 |  | runs/write-page-ack/mcp-write/opus/r1/transcript.jsonl |
| write-page-resolve-archive | cli-write+skill | opus | 1 | pass | 12 | $0.186 |  | runs/write-page-resolve-archive/cli-write_skill/opus/r1/transcript.jsonl |
| write-page-resolve-archive | mcp-write | opus | 1 | pass | 11 | $0.227 |  | runs/write-page-resolve-archive/mcp-write/opus/r1/transcript.jsonl |
| write-postmortem-conflict | cli-write+skill | opus | 1 | pass | 11 | $0.124 |  | runs/write-postmortem-conflict/cli-write_skill/opus/r1/transcript.jsonl |
| write-postmortem-conflict | mcp-write | opus | 1 | pass | 6 | $0.147 |  | runs/write-postmortem-conflict/mcp-write/opus/r1/transcript.jsonl |
| write-postmortem-create | cli-write+skill | opus | 1 | pass | 11 | $0.292 |  | runs/write-postmortem-create/cli-write_skill/opus/r1/transcript.jsonl |
| write-postmortem-create | mcp-write | opus | 1 | pass | 17 | $0.472 |  | runs/write-postmortem-create/mcp-write/opus/r1/transcript.jsonl |
| write-readonly-member | cli-write+skill | opus | 1 | pass | 6 | $0.098 |  | runs/write-readonly-member/cli-write_skill/opus/r1/transcript.jsonl |
| write-readonly-member | mcp-write | opus | 1 | fail | 6 | $0.163 | report: report does not match /(role\|member\|permission\|account\|access level)/ | runs/write-readonly-member/mcp-write/opus/r1/transcript.jsonl |
| write-readonly-token | cli-write+skill | opus | 1 | pass | 4 | $0.093 |  | runs/write-readonly-token/cli-write_skill/opus/r1/transcript.jsonl |
| write-readonly-token | mcp-write | opus | 1 | pass | 6 | $0.140 |  | runs/write-readonly-token/mcp-write/opus/r1/transcript.jsonl |
