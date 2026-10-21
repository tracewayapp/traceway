# Traceway evals

- **Run:** `opus-full-perf-write-20261021`, started by 15340338+dusanstanojeviccs@users.noreply.github.com
- **Commit:** `49c921398af2`, skill `traceway` `52f484fb85`, knowledge `1503eb52f0`
- **Models:** astra=`openai/gpt-6-astra`, haiku=`anthropic/claude-haiku-5.5`, judge=`anthropic/claude-haiku-5.5`, opus=`anthropic/claude-opus-5.5`
- **Spend:** $15.75 (judge $0.05) of a $75.00 cap
- **Runs:** 131 (fail 11, infra_error 83, pass 37)

## Pass rate by suite

| Suite | Model | Arm | Pass | Rate |
|---|---|---|---|---|
| perf | opus | cli+skill | 10/14 | 71% |
| perf | opus | cli-noskill | 7/13 | 54% |
| perf | opus | mcp | 12/13 | 92% |
| write | opus | cli-write+skill | 5/5 | 100% |
| write | opus | mcp-write | 3/3 | 100% |

## Skill uplift (cli+skill minus cli-noskill)

| Suite | Model | With skill | Without | Uplift |
|---|---|---|---|---|
| perf | opus | 69% | 54% | +15pp |

## CLI vs MCP

| Suite | Model | CLI arm | MCP arm | CLI | MCP | Delta |
|---|---|---|---|---|---|---|
| perf | opus | cli+skill | mcp | 69% | 92% | -23pp |
| write | opus | cli-write+skill | mcp-write | 100% | 100% | +0pp |

## Cost (averages over passing runs)

| Model | Passing runs | Avg cost | Avg tool calls | Avg input tokens | Avg output tokens | Cache hit rate | Total spend |
|---|---|---|---|---|---|---|---|
| opus | 37 | $0.277 | 12.4 | 215640 | 4312 | 85% | $15.70 |

| Suite / model | Spend |
|---|---|
| perf / opus | $14.07 |
| write / opus | $1.63 |

## Top failing graders

- trajectory: 9
- grounding: 4
- answer: 1

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
| perf-ctl-onset-bisect | cli-noskill | opus | 1 | infra_error | 11 | $0.130 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-ctl-onset-bisect/cli-noskill/opus/r1/transcript.jsonl |
| perf-ctl-onset-bisect | mcp | opus | 1 | infra_error | 8 | $0.166 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-ctl-onset-bisect/mcp/opus/r1/transcript.jsonl |
| perf-db-expensive-join | cli+skill | opus | 1 | infra_error | 2 | $0.116 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-expensive-join/cli_skill/opus/r1/transcript.jsonl |
| perf-db-expensive-join | cli-noskill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-expensive-join/cli-noskill/opus/r1/transcript.jsonl |
| perf-db-expensive-join | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-expensive-join/mcp/opus/r1/transcript.jsonl |
| perf-db-lock-contention | cli+skill | opus | 1 | infra_error | 1 | $0.024 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-lock-contention/cli_skill/opus/r1/transcript.jsonl |
| perf-db-lock-contention | cli-noskill | opus | 1 | infra_error | 7 | $0.091 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-lock-contention/cli-noskill/opus/r1/transcript.jsonl |
| perf-db-lock-contention | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-lock-contention/mcp/opus/r1/transcript.jsonl |
| perf-db-missing-index | cli+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-missing-index/cli_skill/opus/r1/transcript.jsonl |
| perf-db-missing-index | cli-noskill | opus | 1 | infra_error | 6 | $0.079 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-missing-index/cli-noskill/opus/r1/transcript.jsonl |
| perf-db-missing-index | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-missing-index/mcp/opus/r1/transcript.jsonl |
| perf-db-missing-query-cache | cli+skill | opus | 1 | infra_error | 1 | $0.089 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-missing-query-cache/cli_skill/opus/r1/transcript.jsonl |
| perf-db-missing-query-cache | cli-noskill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-missing-query-cache/cli-noskill/opus/r1/transcript.jsonl |
| perf-db-missing-query-cache | mcp | opus | 1 | infra_error | 2 | $0.073 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-missing-query-cache/mcp/opus/r1/transcript.jsonl |
| perf-db-n-plus-1 | cli+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-n-plus-1/cli_skill/opus/r1/transcript.jsonl |
| perf-db-n-plus-1 | cli-noskill | opus | 1 | infra_error | 2 | $0.028 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-n-plus-1/cli-noskill/opus/r1/transcript.jsonl |
| perf-db-n-plus-1 | mcp | opus | 1 | infra_error | 6 | $0.086 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-n-plus-1/mcp/opus/r1/transcript.jsonl |
| perf-db-over-fetching | cli+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-over-fetching/cli_skill/opus/r1/transcript.jsonl |
| perf-db-over-fetching | cli-noskill | opus | 1 | infra_error | 6 | $0.065 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-over-fetching/cli-noskill/opus/r1/transcript.jsonl |
| perf-db-over-fetching | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-over-fetching/mcp/opus/r1/transcript.jsonl |
| perf-db-pool-exhaustion | cli+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-pool-exhaustion/cli_skill/opus/r1/transcript.jsonl |
| perf-db-pool-exhaustion | cli-noskill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-pool-exhaustion/cli-noskill/opus/r1/transcript.jsonl |
| perf-db-pool-exhaustion | mcp | opus | 1 | infra_error | 2 | $0.075 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-pool-exhaustion/mcp/opus/r1/transcript.jsonl |
| perf-db-unbounded-result | cli+skill | opus | 1 | infra_error | 3 | $0.091 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-unbounded-result/cli_skill/opus/r1/transcript.jsonl |
| perf-db-unbounded-result | cli-noskill | opus | 1 | infra_error | 1 | $0.023 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-unbounded-result/cli-noskill/opus/r1/transcript.jsonl |
| perf-db-unbounded-result | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-db-unbounded-result/mcp/opus/r1/transcript.jsonl |
| perf-infra-cpu-saturation | cli+skill | opus | 1 | infra_error | 1 | $0.089 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-infra-cpu-saturation/cli_skill/opus/r1/transcript.jsonl |
| perf-infra-cpu-saturation | cli-noskill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-infra-cpu-saturation/cli-noskill/opus/r1/transcript.jsonl |
| perf-infra-cpu-saturation | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-infra-cpu-saturation/mcp/opus/r1/transcript.jsonl |
| perf-infra-disk-io | cli+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-infra-disk-io/cli_skill/opus/r1/transcript.jsonl |
| perf-infra-disk-io | cli-noskill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-infra-disk-io/cli-noskill/opus/r1/transcript.jsonl |
| perf-infra-disk-io | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-infra-disk-io/mcp/opus/r1/transcript.jsonl |
| perf-infra-memory-gc | cli+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-infra-memory-gc/cli_skill/opus/r1/transcript.jsonl |
| perf-infra-memory-gc | cli-noskill | opus | 1 | infra_error | 1 | $0.022 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-infra-memory-gc/cli-noskill/opus/r1/transcript.jsonl |
| perf-infra-memory-gc | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-infra-memory-gc/mcp/opus/r1/transcript.jsonl |
| perf-infra-noisy-neighbor | cli+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-infra-noisy-neighbor/cli_skill/opus/r1/transcript.jsonl |
| perf-infra-noisy-neighbor | cli-noskill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-infra-noisy-neighbor/cli-noskill/opus/r1/transcript.jsonl |
| perf-infra-noisy-neighbor | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-infra-noisy-neighbor/mcp/opus/r1/transcript.jsonl |
| perf-net-chatty-fanout | cli+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-chatty-fanout/cli_skill/opus/r1/transcript.jsonl |
| perf-net-chatty-fanout | cli-noskill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-chatty-fanout/cli-noskill/opus/r1/transcript.jsonl |
| perf-net-chatty-fanout | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-chatty-fanout/mcp/opus/r1/transcript.jsonl |
| perf-net-large-payload | cli+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-large-payload/cli_skill/opus/r1/transcript.jsonl |
| perf-net-large-payload | cli-noskill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-large-payload/cli-noskill/opus/r1/transcript.jsonl |
| perf-net-large-payload | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-large-payload/mcp/opus/r1/transcript.jsonl |
| perf-net-no-conn-reuse | cli+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-no-conn-reuse/cli_skill/opus/r1/transcript.jsonl |
| perf-net-no-conn-reuse | cli-noskill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-no-conn-reuse/cli-noskill/opus/r1/transcript.jsonl |
| perf-net-no-conn-reuse | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-no-conn-reuse/mcp/opus/r1/transcript.jsonl |
| perf-net-retries | cli+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-retries/cli_skill/opus/r1/transcript.jsonl |
| perf-net-retries | cli-noskill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-retries/cli-noskill/opus/r1/transcript.jsonl |
| perf-net-retries | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-retries/mcp/opus/r1/transcript.jsonl |
| perf-net-sequential | cli+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-sequential/cli_skill/opus/r1/transcript.jsonl |
| perf-net-sequential | cli-noskill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-sequential/cli-noskill/opus/r1/transcript.jsonl |
| perf-net-sequential | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-sequential/mcp/opus/r1/transcript.jsonl |
| perf-net-slow-third-party | cli+skill | opus | 1 | infra_error | 1 | $0.023 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-slow-third-party/cli_skill/opus/r1/transcript.jsonl |
| perf-net-slow-third-party | cli-noskill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-slow-third-party/cli-noskill/opus/r1/transcript.jsonl |
| perf-net-slow-third-party | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-net-slow-third-party/mcp/opus/r1/transcript.jsonl |
| perf-xs-android-timeouts | cli+skill | opus | 1 | infra_error | 2 | $0.093 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-xs-android-timeouts/cli_skill/opus/r1/transcript.jsonl |
| perf-xs-android-timeouts | cli-noskill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-xs-android-timeouts/cli-noskill/opus/r1/transcript.jsonl |
| perf-xs-android-timeouts | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-xs-android-timeouts/mcp/opus/r1/transcript.jsonl |
| perf-xs-frontend-timeouts | cli+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-xs-frontend-timeouts/cli_skill/opus/r1/transcript.jsonl |
| perf-xs-frontend-timeouts | cli-noskill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-xs-frontend-timeouts/cli-noskill/opus/r1/transcript.jsonl |
| perf-xs-frontend-timeouts | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-xs-frontend-timeouts/mcp/opus/r1/transcript.jsonl |
| perf-xs-ios-watchdog | cli+skill | opus | 1 | infra_error | 8 | $0.257 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-xs-ios-watchdog/cli_skill/opus/r1/transcript.jsonl |
| perf-xs-ios-watchdog | cli-noskill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-xs-ios-watchdog/cli-noskill/opus/r1/transcript.jsonl |
| perf-xs-ios-watchdog | mcp | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/perf-xs-ios-watchdog/mcp/opus/r1/transcript.jsonl |
| write-alert-disable | cli-write+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-alert-disable/cli-write_skill/opus/r1/transcript.jsonl |
| write-alert-disable | mcp-write | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-alert-disable/mcp-write/opus/r1/transcript.jsonl |
| write-alert-snooze | cli-write+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-alert-snooze/cli-write_skill/opus/r1/transcript.jsonl |
| write-alert-snooze | mcp-write | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-alert-snooze/mcp-write/opus/r1/transcript.jsonl |
| write-archive-ambiguous | cli-write+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-archive-ambiguous/cli-write_skill/opus/r1/transcript.jsonl |
| write-archive-ambiguous | mcp-write | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-archive-ambiguous/mcp-write/opus/r1/transcript.jsonl |
| write-investigate-only | cli-write+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-investigate-only/cli-write_skill/opus/r1/transcript.jsonl |
| write-investigate-only | mcp-write | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-investigate-only/mcp-write/opus/r1/transcript.jsonl |
| write-mark-slow | cli-write+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-mark-slow/cli-write_skill/opus/r1/transcript.jsonl |
| write-mark-slow | mcp-write | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-mark-slow/mcp-write/opus/r1/transcript.jsonl |
| write-monitor-confirm-fix | cli-write+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-monitor-confirm-fix/cli-write_skill/opus/r1/transcript.jsonl |
| write-monitor-confirm-fix | mcp-write | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-monitor-confirm-fix/mcp-write/opus/r1/transcript.jsonl |
| write-optin-off | cli-write+skill | opus | 1 | pass | 4 | $0.068 |  | runs/write-optin-off/cli-write_skill/opus/r1/transcript.jsonl |
| write-optin-off | mcp-write | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-optin-off/mcp-write/opus/r1/transcript.jsonl |
| write-page-ack | cli-write+skill | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-page-ack/cli-write_skill/opus/r1/transcript.jsonl |
| write-page-ack | mcp-write | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-page-ack/mcp-write/opus/r1/transcript.jsonl |
| write-page-resolve-archive | cli-write+skill | opus | 1 | pass | 12 | $0.186 |  | runs/write-page-resolve-archive/cli-write_skill/opus/r1/transcript.jsonl |
| write-page-resolve-archive | mcp-write | opus | 1 | pass | 11 | $0.227 |  | runs/write-page-resolve-archive/mcp-write/opus/r1/transcript.jsonl |
| write-postmortem-conflict | cli-write+skill | opus | 1 | pass | 11 | $0.124 |  | runs/write-postmortem-conflict/cli-write_skill/opus/r1/transcript.jsonl |
| write-postmortem-conflict | mcp-write | opus | 1 | pass | 6 | $0.147 |  | runs/write-postmortem-conflict/mcp-write/opus/r1/transcript.jsonl |
| write-postmortem-create | cli-write+skill | opus | 1 | infra_error | 5 | $0.107 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-postmortem-create/cli-write_skill/opus/r1/transcript.jsonl |
| write-postmortem-create | mcp-write | opus | 1 | pass | 17 | $0.472 |  | runs/write-postmortem-create/mcp-write/opus/r1/transcript.jsonl |
| write-readonly-member | cli-write+skill | opus | 1 | pass | 6 | $0.098 |  | runs/write-readonly-member/cli-write_skill/opus/r1/transcript.jsonl |
| write-readonly-member | mcp-write | opus | 1 | infra_error | 3 | $0.102 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-readonly-member/mcp-write/opus/r1/transcript.jsonl |
| write-readonly-token | cli-write+skill | opus | 1 | pass | 4 | $0.093 |  | runs/write-readonly-token/cli-write_skill/opus/r1/transcript.jsonl |
| write-readonly-token | mcp-write | opus | 1 | infra_error | 0 | $0.000 | openrouter HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry afte... | runs/write-readonly-token/mcp-write/opus/r1/transcript.jsonl |
