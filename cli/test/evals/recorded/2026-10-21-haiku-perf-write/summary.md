# Traceway evals

- **Run:** `haiku-full-perf-write`, started by 15340338+dusanstanojeviccs@users.noreply.github.com
- **Commit:** `f2432ec8df74` (uncommitted changes), skill `traceway` `52f484fb85`, knowledge `1503eb52f0`
- **Models:** astra=`openai/gpt-6-astra`, haiku=`anthropic/claude-haiku-5.5`, judge=`anthropic/claude-haiku-5.5`, opus=`anthropic/claude-opus-5.5`
- **Spend:** $1.46 (judge $0.14) of a $10.00 cap
- **Runs:** 131 (budget_exhausted 1, fail 60, pass 70)

## Pass rate by suite

| Suite | Model | Arm | Pass | Rate |
|---|---|---|---|---|
| perf | haiku | cli+skill | 13/35 | 37% |
| perf | haiku | cli-noskill | 18/35 | 51% |
| perf | haiku | mcp | 19/35 | 54% |
| write | haiku | cli-write+skill | 10/13 | 77% |
| write | haiku | mcp-write | 10/13 | 77% |

## Skill uplift (cli+skill minus cli-noskill)

| Suite | Model | With skill | Without | Uplift |
|---|---|---|---|---|
| perf | haiku | 37% | 51% | -14pp |

## CLI vs MCP

| Suite | Model | CLI arm | MCP arm | CLI | MCP | Delta |
|---|---|---|---|---|---|---|
| perf | haiku | cli+skill | mcp | 37% | 54% | -17pp |
| write | haiku | cli-write+skill | mcp-write | 77% | 77% | +0pp |

## Cost (averages over passing runs)

| Model | Passing runs | Avg cost | Avg tool calls | Avg input tokens | Avg output tokens | Cache hit rate | Total spend |
|---|---|---|---|---|---|---|---|
| haiku | 70 | $0.008 | 12.7 | 197162 | 5749 | 86% | $1.32 |

| Suite / model | Spend |
|---|---|
| perf / haiku | $1.20 |
| write / haiku | $0.12 |

## Top failing graders

- trajectory: 47
- grounding: 19
- answer: 19
- report: 5
- state: 3

## Worst cases

| Case | Model | Arm | Pass |
|---|---|---|---|
| perf-app-cpu-serialization | haiku | cli-noskill | 0/1 |
| perf-app-cpu-serialization | haiku | mcp | 0/1 |
| perf-app-excessive-logging | haiku | cli+skill | 0/1 |
| perf-app-excessive-logging | haiku | cli-noskill | 0/1 |
| perf-app-large-alloc | haiku | cli+skill | 0/1 |
| perf-app-large-alloc | haiku | cli-noskill | 0/1 |
| perf-app-large-alloc | haiku | mcp | 0/1 |
| perf-app-quadratic | haiku | cli+skill | 0/1 |
| perf-app-sync-background | haiku | cli-noskill | 0/1 |
| perf-app-sync-background | haiku | mcp | 0/1 |
| perf-cache-cold-start | haiku | cli+skill | 0/1 |
| perf-cache-no-cache | haiku | cli-noskill | 0/1 |
| perf-cache-no-cache | haiku | mcp | 0/1 |
| perf-cache-stampede | haiku | cli+skill | 0/1 |
| perf-cache-stampede | haiku | mcp | 0/1 |

## Runs

| Case | Arm | Model | Run | Status | Calls | Cost | First failure | Transcript |
|---|---|---|---|---|---|---|---|---|
| perf-app-cpu-serialization | cli+skill | haiku | 1 | pass | 10 | $0.009 |  | runs/perf-app-cpu-serialization/cli_skill/haiku/r1/transcript.jsonl |
| perf-app-cpu-serialization | cli-noskill | haiku | 1 | fail | 15 | $0.008 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.chart {"interval_minutes":"60","since":"24h"} | runs/perf-app-cpu-serialization/cli-noskill/haiku/r1/transcript.jsonl |
| perf-app-cpu-serialization | mcp | haiku | 1 | fail | 16 | $0.013 | trajectory: the same call ran 3 times (max_identical_repeats 2): git {"subcommand":"log"} | runs/perf-app-cpu-serialization/mcp/haiku/r1/transcript.jsonl |
| perf-app-excessive-logging | cli+skill | haiku | 1 | fail | 16 | $0.012 | trajectory: the same call ran 4 times (max_identical_repeats 2): traces.show {"id":"b7a0814d4e0fe4f33e92615d0dfbafc6","recorded_at":"2026-10-20T01:26:55... | runs/perf-app-excessive-logging/cli_skill/haiku/r1/transcript.jsonl |
| perf-app-excessive-logging | cli-noskill | haiku | 1 | fail | 22 | $0.012 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.chart {"interval_minutes":"60","since":"48h"} | runs/perf-app-excessive-logging/cli-noskill/haiku/r1/transcript.jsonl |
| perf-app-excessive-logging | mcp | haiku | 1 | pass | 20 | $0.015 |  | runs/perf-app-excessive-logging/mcp/haiku/r1/transcript.jsonl |
| perf-app-large-alloc | cli+skill | haiku | 1 | fail | 27 | $0.017 | trajectory: the same call ran 3 times (max_identical_repeats 2): metrics.query {"aggregation":"max","interval_minutes":"60","name":"jvm.memory.used","si... | runs/perf-app-large-alloc/cli_skill/haiku/r1/transcript.jsonl |
| perf-app-large-alloc | cli-noskill | haiku | 1 | fail | 19 | $0.011 | grounding: no cited call's output contains "G1 Old Gen" | runs/perf-app-large-alloc/cli-noskill/haiku/r1/transcript.jsonl |
| perf-app-large-alloc | mcp | haiku | 1 | fail | 19 | $0.012 | grounding: no cited call's output contains "G1 Old Gen" | runs/perf-app-large-alloc/mcp/haiku/r1/transcript.jsonl |
| perf-app-quadratic | cli+skill | haiku | 1 | fail | 15 | $0.011 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.list {"search":"POST /api/quotes","since":"24h"} | runs/perf-app-quadratic/cli_skill/haiku/r1/transcript.jsonl |
| perf-app-quadratic | cli-noskill | haiku | 1 | pass | 12 | $0.006 |  | runs/perf-app-quadratic/cli-noskill/haiku/r1/transcript.jsonl |
| perf-app-quadratic | mcp | haiku | 1 | pass | 15 | $0.010 |  | runs/perf-app-quadratic/mcp/haiku/r1/transcript.jsonl |
| perf-app-sync-background | cli+skill | haiku | 1 | pass | 18 | $0.015 |  | runs/perf-app-sync-background/cli_skill/haiku/r1/transcript.jsonl |
| perf-app-sync-background | cli-noskill | haiku | 1 | fail | 20 | $0.011 | trajectory: required call missing: metrics.query(name=~system\.cpu) | runs/perf-app-sync-background/cli-noskill/haiku/r1/transcript.jsonl |
| perf-app-sync-background | mcp | haiku | 1 | fail | 20 | $0.015 | answer: checklist_item "cpu_bound_serialization", want synchronous_work_that_belongs_in_a_background_task | runs/perf-app-sync-background/mcp/haiku/r1/transcript.jsonl |
| perf-cache-cold-start | cli+skill | haiku | 1 | fail | 15 | $0.013 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.list {"search":"POST /api/prices/quote","since":"6h"} | runs/perf-cache-cold-start/cli_skill/haiku/r1/transcript.jsonl |
| perf-cache-cold-start | cli-noskill | haiku | 1 | pass | 18 | $0.011 |  | runs/perf-cache-cold-start/cli-noskill/haiku/r1/transcript.jsonl |
| perf-cache-cold-start | mcp | haiku | 1 | pass | 24 | $0.016 |  | runs/perf-cache-cold-start/mcp/haiku/r1/transcript.jsonl |
| perf-cache-no-cache | cli+skill | haiku | 1 | pass | 12 | $0.009 |  | runs/perf-cache-no-cache/cli_skill/haiku/r1/transcript.jsonl |
| perf-cache-no-cache | cli-noskill | haiku | 1 | fail | 20 | $0.011 | answer: locus.endpoint "", want "GET /api/catalog" | runs/perf-cache-no-cache/cli-noskill/haiku/r1/transcript.jsonl |
| perf-cache-no-cache | mcp | haiku | 1 | fail | 17 | $0.009 | trajectory: none of these alternatives happened: [traces.show(recorded_at=present)] or [endpoints.show(recorded_at=present)] | runs/perf-cache-no-cache/mcp/haiku/r1/transcript.jsonl |
| perf-cache-stampede | cli+skill | haiku | 1 | fail | 17 | $0.014 | trajectory: the same call ran 3 times (max_identical_repeats 2): logs.query {"page_size":"100","search":"GET /api/home 200","since":"3h"} | runs/perf-cache-stampede/cli_skill/haiku/r1/transcript.jsonl |
| perf-cache-stampede | cli-noskill | haiku | 1 | pass | 16 | $0.009 |  | runs/perf-cache-stampede/cli-noskill/haiku/r1/transcript.jsonl |
| perf-cache-stampede | mcp | haiku | 1 | fail | 17 | $0.012 | grounding: no cited call's output contains "ex 600" | runs/perf-cache-stampede/mcp/haiku/r1/transcript.jsonl |
| perf-ctl-alert-handoff | cli+skill | haiku | 1 | fail | 15 | $0.010 | trajectory: none of these alternatives happened: [traces.show(recorded_at=present)] or [endpoints.show(recorded_at=present)] | runs/perf-ctl-alert-handoff/cli_skill/haiku/r1/transcript.jsonl |
| perf-ctl-alert-handoff | cli-noskill | haiku | 1 | fail | 24 | $0.013 | answer: checklist_item "insufficient_evidence", want cold_start_jit_connection_warmup | runs/perf-ctl-alert-handoff/cli-noskill/haiku/r1/transcript.jsonl |
| perf-ctl-alert-handoff | mcp | haiku | 1 | fail | 19 | $0.014 | trajectory: the same call ran 3 times (max_identical_repeats 2): git {"subcommand":"log"} | runs/perf-ctl-alert-handoff/mcp/haiku/r1/transcript.jsonl |
| perf-ctl-healthy | cli+skill | haiku | 1 | fail | 11 | $0.008 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.list {"search":"GET /api/search","since":"24h"} | runs/perf-ctl-healthy/cli_skill/haiku/r1/transcript.jsonl |
| perf-ctl-healthy | cli-noskill | haiku | 1 | pass | 12 | $0.005 |  | runs/perf-ctl-healthy/cli-noskill/haiku/r1/transcript.jsonl |
| perf-ctl-healthy | mcp | haiku | 1 | pass | 8 | $0.007 |  | runs/perf-ctl-healthy/mcp/haiku/r1/transcript.jsonl |
| perf-ctl-marked-slow-ok | cli+skill | haiku | 1 | fail | 5 | $0.005 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.list {"search":"GET /api/reports/export","since":"24h"} | runs/perf-ctl-marked-slow-ok/cli_skill/haiku/r1/transcript.jsonl |
| perf-ctl-marked-slow-ok | cli-noskill | haiku | 1 | pass | 11 | $0.004 |  | runs/perf-ctl-marked-slow-ok/cli-noskill/haiku/r1/transcript.jsonl |
| perf-ctl-marked-slow-ok | mcp | haiku | 1 | pass | 7 | $0.006 |  | runs/perf-ctl-marked-slow-ok/mcp/haiku/r1/transcript.jsonl |
| perf-ctl-marked-slow-regressed | cli+skill | haiku | 1 | pass | 10 | $0.010 |  | runs/perf-ctl-marked-slow-regressed/cli_skill/haiku/r1/transcript.jsonl |
| perf-ctl-marked-slow-regressed | cli-noskill | haiku | 1 | fail | 13 | $0.008 | answer: onset 2026-10-20T00:00:00Z is -1h5m0s from 2026-10-20T01:05:00Z (tolerance 1h0m0s) | runs/perf-ctl-marked-slow-regressed/cli-noskill/haiku/r1/transcript.jsonl |
| perf-ctl-marked-slow-regressed | mcp | haiku | 1 | fail | 11 | $0.013 | trajectory: none of these alternatives happened: [endpoints.show(recorded_at=present)] or [traces.show(recorded_at=present)] | runs/perf-ctl-marked-slow-regressed/mcp/haiku/r1/transcript.jsonl |
| perf-ctl-metric-p95-trap | cli+skill | haiku | 1 | fail | 23 | $0.016 | report: report does not match /(9[5-9]\|100)(\.[0-9]+)?\s*%\|0\.9[5-9]/ | runs/perf-ctl-metric-p95-trap/cli_skill/haiku/r1/transcript.jsonl |
| perf-ctl-metric-p95-trap | cli-noskill | haiku | 1 | fail | 28 | $0.014 | answer: checklist_item "insufficient_evidence", want cpu_saturation | runs/perf-ctl-metric-p95-trap/cli-noskill/haiku/r1/transcript.jsonl |
| perf-ctl-metric-p95-trap | mcp | haiku | 1 | fail | 24 | $0.016 | report: report does not match /(9[5-9]\|100)(\.[0-9]+)?\s*%\|0\.9[5-9]/ | runs/perf-ctl-metric-p95-trap/mcp/haiku/r1/transcript.jsonl |
| perf-ctl-onset-bisect | cli+skill | haiku | 1 | fail | 13 | $0.012 | trajectory: the same call ran 16 times (max_identical_repeats 2): endpoints.list {"from":"--to","page_size":"5","search":"GET /api/account/summary"} | runs/perf-ctl-onset-bisect/cli_skill/haiku/r1/transcript.jsonl |
| perf-ctl-onset-bisect | cli-noskill | haiku | 1 | fail | 31 | $0.014 | trajectory: 8 failed tool calls, more than max_failed_calls 4 | runs/perf-ctl-onset-bisect/cli-noskill/haiku/r1/transcript.jsonl |
| perf-ctl-onset-bisect | mcp | haiku | 1 | fail | 23 | $0.017 | trajectory: none of these alternatives happened: [endpoints.show(recorded_at=present)] or [traces.show(recorded_at=present)] | runs/perf-ctl-onset-bisect/mcp/haiku/r1/transcript.jsonl |
| perf-db-expensive-join | cli+skill | haiku | 1 | pass | 7 | $0.007 |  | runs/perf-db-expensive-join/cli_skill/haiku/r1/transcript.jsonl |
| perf-db-expensive-join | cli-noskill | haiku | 1 | pass | 11 | $0.004 |  | runs/perf-db-expensive-join/cli-noskill/haiku/r1/transcript.jsonl |
| perf-db-expensive-join | mcp | haiku | 1 | pass | 9 | $0.007 |  | runs/perf-db-expensive-join/mcp/haiku/r1/transcript.jsonl |
| perf-db-lock-contention | cli+skill | haiku | 1 | pass | 15 | $0.011 |  | runs/perf-db-lock-contention/cli_skill/haiku/r1/transcript.jsonl |
| perf-db-lock-contention | cli-noskill | haiku | 1 | pass | 23 | $0.012 |  | runs/perf-db-lock-contention/cli-noskill/haiku/r1/transcript.jsonl |
| perf-db-lock-contention | mcp | haiku | 1 | pass | 16 | $0.013 |  | runs/perf-db-lock-contention/mcp/haiku/r1/transcript.jsonl |
| perf-db-missing-index | cli+skill | haiku | 1 | fail | 10 | $0.008 | trajectory: none of these alternatives happened: [endpoints.show(recorded_at=present) + endpoints.chart] or [traces.show(recorded_at=present) + endpoint... | runs/perf-db-missing-index/cli_skill/haiku/r1/transcript.jsonl |
| perf-db-missing-index | cli-noskill | haiku | 1 | pass | 20 | $0.008 |  | runs/perf-db-missing-index/cli-noskill/haiku/r1/transcript.jsonl |
| perf-db-missing-index | mcp | haiku | 1 | pass | 12 | $0.009 |  | runs/perf-db-missing-index/mcp/haiku/r1/transcript.jsonl |
| perf-db-missing-query-cache | cli+skill | haiku | 1 | pass | 13 | $0.010 |  | runs/perf-db-missing-query-cache/cli_skill/haiku/r1/transcript.jsonl |
| perf-db-missing-query-cache | cli-noskill | haiku | 1 | pass | 15 | $0.005 |  | runs/perf-db-missing-query-cache/cli-noskill/haiku/r1/transcript.jsonl |
| perf-db-missing-query-cache | mcp | haiku | 1 | pass | 16 | $0.008 |  | runs/perf-db-missing-query-cache/mcp/haiku/r1/transcript.jsonl |
| perf-db-n-plus-1 | cli+skill | haiku | 1 | fail | 13 | $0.010 | trajectory: required call missing: endpoints.show(recorded_at=present) | runs/perf-db-n-plus-1/cli_skill/haiku/r1/transcript.jsonl |
| perf-db-n-plus-1 | cli-noskill | haiku | 1 | fail | 14 | $0.007 | trajectory: required call missing: endpoints.show(recorded_at=present) | runs/perf-db-n-plus-1/cli-noskill/haiku/r1/transcript.jsonl |
| perf-db-n-plus-1 | mcp | haiku | 1 | fail | 15 | $0.013 | trajectory: required call missing: endpoints.show(recorded_at=present) | runs/perf-db-n-plus-1/mcp/haiku/r1/transcript.jsonl |
| perf-db-over-fetching | cli+skill | haiku | 1 | pass | 13 | $0.011 |  | runs/perf-db-over-fetching/cli_skill/haiku/r1/transcript.jsonl |
| perf-db-over-fetching | cli-noskill | haiku | 1 | pass | 15 | $0.007 |  | runs/perf-db-over-fetching/cli-noskill/haiku/r1/transcript.jsonl |
| perf-db-over-fetching | mcp | haiku | 1 | pass | 18 | $0.012 |  | runs/perf-db-over-fetching/mcp/haiku/r1/transcript.jsonl |
| perf-db-pool-exhaustion | cli+skill | haiku | 1 | pass | 13 | $0.009 |  | runs/perf-db-pool-exhaustion/cli_skill/haiku/r1/transcript.jsonl |
| perf-db-pool-exhaustion | cli-noskill | haiku | 1 | pass | 20 | $0.009 |  | runs/perf-db-pool-exhaustion/cli-noskill/haiku/r1/transcript.jsonl |
| perf-db-pool-exhaustion | mcp | haiku | 1 | pass | 12 | $0.012 |  | runs/perf-db-pool-exhaustion/mcp/haiku/r1/transcript.jsonl |
| perf-db-unbounded-result | cli+skill | haiku | 1 | fail | 12 | $0.009 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.chart {"interval_minutes":"360","metric_type":"p95","since":"7d"} | runs/perf-db-unbounded-result/cli_skill/haiku/r1/transcript.jsonl |
| perf-db-unbounded-result | cli-noskill | haiku | 1 | pass | 17 | $0.006 |  | runs/perf-db-unbounded-result/cli-noskill/haiku/r1/transcript.jsonl |
| perf-db-unbounded-result | mcp | haiku | 1 | pass | 17 | $0.010 |  | runs/perf-db-unbounded-result/mcp/haiku/r1/transcript.jsonl |
| perf-infra-cpu-saturation | cli+skill | haiku | 1 | fail | 23 | $0.021 | answer: checklist_item "connection_pool_exhaustion", want cpu_saturation | runs/perf-infra-cpu-saturation/cli_skill/haiku/r1/transcript.jsonl |
| perf-infra-cpu-saturation | cli-noskill | haiku | 1 | fail | 30 | $0.013 | answer: checklist_item "slow_third_party_upstream_api", want cpu_saturation | runs/perf-infra-cpu-saturation/cli-noskill/haiku/r1/transcript.jsonl |
| perf-infra-cpu-saturation | mcp | haiku | 1 | budget_exhausted | 28 | $0.027 | answer: no submission (budget_exhausted) | runs/perf-infra-cpu-saturation/mcp/haiku/r1/transcript.jsonl |
| perf-infra-disk-io | cli+skill | haiku | 1 | fail | 18 | $0.016 | answer: checklist_item "expensive_join_or_aggregation", want disk_io_bottleneck | runs/perf-infra-disk-io/cli_skill/haiku/r1/transcript.jsonl |
| perf-infra-disk-io | cli-noskill | haiku | 1 | fail | 35 | $0.015 | answer: locus.host "bookings-db.internal", want "pg-bookings-1" | runs/perf-infra-disk-io/cli-noskill/haiku/r1/transcript.jsonl |
| perf-infra-disk-io | mcp | haiku | 1 | fail | 16 | $0.010 | answer: checklist_item "expensive_join_or_aggregation", want disk_io_bottleneck | runs/perf-infra-disk-io/mcp/haiku/r1/transcript.jsonl |
| perf-infra-memory-gc | cli+skill | haiku | 1 | fail | 25 | $0.023 | answer: checklist_item "unbounded_in_memory_work_large_allocations", want memory_pressure_gc_pauses | runs/perf-infra-memory-gc/cli_skill/haiku/r1/transcript.jsonl |
| perf-infra-memory-gc | cli-noskill | haiku | 1 | fail | 18 | $0.010 | answer: checklist_item "unbounded_in_memory_work_large_allocations", want memory_pressure_gc_pauses | runs/perf-infra-memory-gc/cli-noskill/haiku/r1/transcript.jsonl |
| perf-infra-memory-gc | mcp | haiku | 1 | fail | 19 | $0.013 | answer: checklist_item "unbounded_in_memory_work_large_allocations", want memory_pressure_gc_pauses | runs/perf-infra-memory-gc/mcp/haiku/r1/transcript.jsonl |
| perf-infra-noisy-neighbor | cli+skill | haiku | 1 | pass | 29 | $0.025 |  | runs/perf-infra-noisy-neighbor/cli_skill/haiku/r1/transcript.jsonl |
| perf-infra-noisy-neighbor | cli-noskill | haiku | 1 | fail | 28 | $0.016 | answer: onset 2026-10-20T15:44:00Z is 2h46m0s from 2026-10-20T12:58:00Z (tolerance 1h0m0s) | runs/perf-infra-noisy-neighbor/cli-noskill/haiku/r1/transcript.jsonl |
| perf-infra-noisy-neighbor | mcp | haiku | 1 | fail | 19 | $0.012 | answer: checklist_item "insufficient_evidence", want undersized_noisy_neighbor_instance | runs/perf-infra-noisy-neighbor/mcp/haiku/r1/transcript.jsonl |
| perf-net-chatty-fanout | cli+skill | haiku | 1 | fail | 12 | $0.020 | grounding: evidence[2] cites c07, whose output was truncated | runs/perf-net-chatty-fanout/cli_skill/haiku/r1/transcript.jsonl |
| perf-net-chatty-fanout | cli-noskill | haiku | 1 | fail | 15 | $0.008 | trajectory: the same call ran 3 times (max_identical_repeats 2): traces.show {"id":"263230d0e86862e5e201423c74ace408","recorded_at":"2026-10-21T03:02:45... | runs/perf-net-chatty-fanout/cli-noskill/haiku/r1/transcript.jsonl |
| perf-net-chatty-fanout | mcp | haiku | 1 | pass | 11 | $0.014 |  | runs/perf-net-chatty-fanout/mcp/haiku/r1/transcript.jsonl |
| perf-net-large-payload | cli+skill | haiku | 1 | pass | 8 | $0.009 |  | runs/perf-net-large-payload/cli_skill/haiku/r1/transcript.jsonl |
| perf-net-large-payload | cli-noskill | haiku | 1 | pass | 12 | $0.006 |  | runs/perf-net-large-payload/cli-noskill/haiku/r1/transcript.jsonl |
| perf-net-large-payload | mcp | haiku | 1 | pass | 18 | $0.011 |  | runs/perf-net-large-payload/mcp/haiku/r1/transcript.jsonl |
| perf-net-no-conn-reuse | cli+skill | haiku | 1 | fail | 14 | $0.012 | trajectory: the same call ran 6 times (max_identical_repeats 2): endpoints.list {"search":"stores/nearby","since":"24h"} | runs/perf-net-no-conn-reuse/cli_skill/haiku/r1/transcript.jsonl |
| perf-net-no-conn-reuse | cli-noskill | haiku | 1 | pass | 15 | $0.007 |  | runs/perf-net-no-conn-reuse/cli-noskill/haiku/r1/transcript.jsonl |
| perf-net-no-conn-reuse | mcp | haiku | 1 | pass | 11 | $0.012 |  | runs/perf-net-no-conn-reuse/mcp/haiku/r1/transcript.jsonl |
| perf-net-retries | cli+skill | haiku | 1 | pass | 16 | $0.012 |  | runs/perf-net-retries/cli_skill/haiku/r1/transcript.jsonl |
| perf-net-retries | cli-noskill | haiku | 1 | pass | 17 | $0.010 |  | runs/perf-net-retries/cli-noskill/haiku/r1/transcript.jsonl |
| perf-net-retries | mcp | haiku | 1 | pass | 15 | $0.015 |  | runs/perf-net-retries/mcp/haiku/r1/transcript.jsonl |
| perf-net-sequential | cli+skill | haiku | 1 | fail | 11 | $0.010 | trajectory: the same call ran 4 times (max_identical_repeats 2): endpoints.list {"search":"GET /api/dashboard","since":"24h"} | runs/perf-net-sequential/cli_skill/haiku/r1/transcript.jsonl |
| perf-net-sequential | cli-noskill | haiku | 1 | fail | 13 | $0.005 | trajectory: none of these alternatives happened: [traces.show(recorded_at=present)] or [endpoints.show(recorded_at=present)] | runs/perf-net-sequential/cli-noskill/haiku/r1/transcript.jsonl |
| perf-net-sequential | mcp | haiku | 1 | fail | 18 | $0.009 | trajectory: none of these alternatives happened: [traces.show(recorded_at=present)] or [endpoints.show(recorded_at=present)] | runs/perf-net-sequential/mcp/haiku/r1/transcript.jsonl |
| perf-net-slow-third-party | cli+skill | haiku | 1 | pass | 12 | $0.011 |  | runs/perf-net-slow-third-party/cli_skill/haiku/r1/transcript.jsonl |
| perf-net-slow-third-party | cli-noskill | haiku | 1 | pass | 16 | $0.008 |  | runs/perf-net-slow-third-party/cli-noskill/haiku/r1/transcript.jsonl |
| perf-net-slow-third-party | mcp | haiku | 1 | pass | 17 | $0.015 |  | runs/perf-net-slow-third-party/mcp/haiku/r1/transcript.jsonl |
| perf-xs-android-timeouts | cli+skill | haiku | 1 | fail | 20 | $0.014 | trajectory: the same call ran 3 times (max_identical_repeats 2): endpoints.chart {"interval_minutes":"15","metric_type":"p95","project_id":"ebbd03e4-35b... | runs/perf-xs-android-timeouts/cli_skill/haiku/r1/transcript.jsonl |
| perf-xs-android-timeouts | cli-noskill | haiku | 1 | pass | 25 | $0.012 |  | runs/perf-xs-android-timeouts/cli-noskill/haiku/r1/transcript.jsonl |
| perf-xs-android-timeouts | mcp | haiku | 1 | pass | 21 | $0.017 |  | runs/perf-xs-android-timeouts/mcp/haiku/r1/transcript.jsonl |
| perf-xs-frontend-timeouts | cli+skill | haiku | 1 | fail | 15 | $0.012 | grounding: evidence[1] cites c04, which failed | runs/perf-xs-frontend-timeouts/cli_skill/haiku/r1/transcript.jsonl |
| perf-xs-frontend-timeouts | cli-noskill | haiku | 1 | pass | 23 | $0.012 |  | runs/perf-xs-frontend-timeouts/cli-noskill/haiku/r1/transcript.jsonl |
| perf-xs-frontend-timeouts | mcp | haiku | 1 | pass | 23 | $0.011 |  | runs/perf-xs-frontend-timeouts/mcp/haiku/r1/transcript.jsonl |
| perf-xs-ios-watchdog | cli+skill | haiku | 1 | fail | 14 | $0.013 | answer: checklist_item "synchronous_work_that_belongs_in_a_background_task", want connection_pool_exhaustion | runs/perf-xs-ios-watchdog/cli_skill/haiku/r1/transcript.jsonl |
| perf-xs-ios-watchdog | cli-noskill | haiku | 1 | fail | 28 | $0.012 | answer: checklist_item "synchronous_work_that_belongs_in_a_background_task", want connection_pool_exhaustion | runs/perf-xs-ios-watchdog/cli-noskill/haiku/r1/transcript.jsonl |
| perf-xs-ios-watchdog | mcp | haiku | 1 | fail | 21 | $0.012 | answer: checklist_item "synchronous_work_that_belongs_in_a_background_task", want connection_pool_exhaustion | runs/perf-xs-ios-watchdog/mcp/haiku/r1/transcript.jsonl |
| write-alert-disable | cli-write+skill | haiku | 1 | pass | 5 | $0.002 |  | runs/write-alert-disable/cli-write_skill/haiku/r1/transcript.jsonl |
| write-alert-disable | mcp-write | haiku | 1 | pass | 3 | $0.003 |  | runs/write-alert-disable/mcp-write/haiku/r1/transcript.jsonl |
| write-alert-snooze | cli-write+skill | haiku | 1 | pass | 4 | $0.002 |  | runs/write-alert-snooze/cli-write_skill/haiku/r1/transcript.jsonl |
| write-alert-snooze | mcp-write | haiku | 1 | pass | 4 | $0.003 |  | runs/write-alert-snooze/mcp-write/haiku/r1/transcript.jsonl |
| write-archive-ambiguous | cli-write+skill | haiku | 1 | fail | 7 | $0.005 | trajectory: ask_user (c06) came after exceptions.archive (c05) | runs/write-archive-ambiguous/cli-write_skill/haiku/r1/transcript.jsonl |
| write-archive-ambiguous | mcp-write | haiku | 1 | pass | 5 | $0.004 |  | runs/write-archive-ambiguous/mcp-write/haiku/r1/transcript.jsonl |
| write-investigate-only | cli-write+skill | haiku | 1 | pass | 8 | $0.006 |  | runs/write-investigate-only/cli-write_skill/haiku/r1/transcript.jsonl |
| write-investigate-only | mcp-write | haiku | 1 | pass | 6 | $0.004 |  | runs/write-investigate-only/mcp-write/haiku/r1/transcript.jsonl |
| write-mark-slow | cli-write+skill | haiku | 1 | pass | 9 | $0.006 |  | runs/write-mark-slow/cli-write_skill/haiku/r1/transcript.jsonl |
| write-mark-slow | mcp-write | haiku | 1 | pass | 6 | $0.004 |  | runs/write-mark-slow/mcp-write/haiku/r1/transcript.jsonl |
| write-monitor-confirm-fix | cli-write+skill | haiku | 1 | fail | 6 | $0.004 | trajectory: required call missing: monitors.run(id=2) | runs/write-monitor-confirm-fix/cli-write_skill/haiku/r1/transcript.jsonl |
| write-monitor-confirm-fix | mcp-write | haiku | 1 | fail | 4 | $0.003 | trajectory: required call missing: monitors.run(id=2) | runs/write-monitor-confirm-fix/mcp-write/haiku/r1/transcript.jsonl |
| write-optin-off | cli-write+skill | haiku | 1 | pass | 4 | $0.002 |  | runs/write-optin-off/cli-write_skill/haiku/r1/transcript.jsonl |
| write-optin-off | mcp-write | haiku | 1 | pass | 5 | $0.003 |  | runs/write-optin-off/mcp-write/haiku/r1/transcript.jsonl |
| write-page-ack | cli-write+skill | haiku | 1 | pass | 5 | $0.002 |  | runs/write-page-ack/cli-write_skill/haiku/r1/transcript.jsonl |
| write-page-ack | mcp-write | haiku | 1 | pass | 3 | $0.003 |  | runs/write-page-ack/mcp-write/haiku/r1/transcript.jsonl |
| write-page-resolve-archive | cli-write+skill | haiku | 1 | pass | 10 | $0.007 |  | runs/write-page-resolve-archive/cli-write_skill/haiku/r1/transcript.jsonl |
| write-page-resolve-archive | mcp-write | haiku | 1 | fail | 12 | $0.006 | trajectory: required call missing: ask_user | runs/write-page-resolve-archive/mcp-write/haiku/r1/transcript.jsonl |
| write-postmortem-conflict | cli-write+skill | haiku | 1 | pass | 7 | $0.004 |  | runs/write-postmortem-conflict/cli-write_skill/haiku/r1/transcript.jsonl |
| write-postmortem-conflict | mcp-write | haiku | 1 | pass | 6 | $0.005 |  | runs/write-postmortem-conflict/mcp-write/haiku/r1/transcript.jsonl |
| write-postmortem-create | cli-write+skill | haiku | 1 | fail | 16 | $0.013 | state: state[0] post_mortem: post-mortem 2 must not contain /\b312\b/ | runs/write-postmortem-create/cli-write_skill/haiku/r1/transcript.jsonl |
| write-postmortem-create | mcp-write | haiku | 1 | pass | 15 | $0.012 |  | runs/write-postmortem-create/mcp-write/haiku/r1/transcript.jsonl |
| write-readonly-member | cli-write+skill | haiku | 1 | pass | 6 | $0.004 |  | runs/write-readonly-member/cli-write_skill/haiku/r1/transcript.jsonl |
| write-readonly-member | mcp-write | haiku | 1 | fail | 7 | $0.004 | report: report does not match /(role\|member\|permission\|account\|access level)/ | runs/write-readonly-member/mcp-write/haiku/r1/transcript.jsonl |
| write-readonly-token | cli-write+skill | haiku | 1 | pass | 7 | $0.005 |  | runs/write-readonly-token/cli-write_skill/haiku/r1/transcript.jsonl |
| write-readonly-token | mcp-write | haiku | 1 | pass | 4 | $0.003 |  | runs/write-readonly-token/mcp-write/haiku/r1/transcript.jsonl |
