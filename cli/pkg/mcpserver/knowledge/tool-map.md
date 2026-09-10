# CLI command to MCP tool map

The knowledge resources show `traceway` CLI commands in their examples. Every command maps 1:1 to an MCP tool with the same parameters (flags become snake_case params). Use this table to translate.

| CLI command | MCP tool |
|---|---|
| `traceway projects list` | `list_projects` |
| `traceway exceptions list` | `list_exceptions` |
| `traceway exceptions show <hash>` | `get_exception` |
| `traceway exceptions occurrence <id> --recorded-at <t>` | `get_exception_occurrence` |
| `traceway exceptions archive <hash> --yes` | `archive_exceptions` |
| `traceway exceptions unarchive <hash> --yes` | `unarchive_exceptions` |
| `traceway logs query` | `query_logs` |
| `traceway endpoints list` | `list_endpoints` |
| `traceway endpoints show <id> --recorded-at <t>` | `get_endpoint_request` |
| `traceway endpoints chart` | `endpoints_chart` |
| `traceway endpoints slow <endpoint>` | `get_slow_endpoint_config` |
| `traceway tasks show <id> --recorded-at <t>` | `get_task` |
| `traceway ai-traces show <id> --recorded-at <t>` | `get_ai_trace` |
| `traceway sessions show <id> --started-at <t>` | `get_session` |
| `traceway traces show <id> --recorded-at <t>` | `get_trace` |
| `traceway metrics query --name <n>` | `query_metrics` |

Flag translation: `--since/--from/--to` are `since`/`from`/`to`; `--page/--page-size` are `page`/`page_size`; `--source` is `source`; `--recorded-at` is `recorded_at`; `--started-at` is `started_at`; `--min-severity` is `min_severity`; `--search-type` is `search_type`; `--order-by` is `order_by`; `--interval-minutes` is `interval_minutes`; `--metric-type` is `metric_type`.

Differences from the CLI:

- Project selection: tools take an optional `project_id` param and fall back to the CLI's current project. There is no `projects use` tool; the default is whatever `traceway projects use` set on the host machine.
- Source selection: every telemetry tool takes an optional `source` param naming one of the profile's bound sources (`traceway sources list` on the host). Without it the tool merges every source that answers the domain and tags each record with `source`; a source that failed while others answered is reported in a second text block as a warning. `sources add | list | remove` are CLI-only.
- `query_metrics` rejects `p50/p95/p99` aggregations outright (the CLI accepts them but the server silently computes avg). Latency percentiles come from `list_endpoints`.
- No confirmation gate: `archive_exceptions`/`unarchive_exceptions` execute immediately, so only call them on an explicit user request. The MCP client's own tool-approval flow is the confirmation.
- Login, logout, and profile management are CLI-only (`traceway login` in a terminal); the MCP server uses the credentials stored by the CLI.
- Fix-agent attempts are CLI-only: `traceway agent attempts` lists the project's attempts, `traceway agent show <id>` prints one with its links and event stream, and `traceway agent report --hash <hash> --status fixed|analysis --pr <url> --report-file <path>` records a run that happened in CI as an attempt on the issue page. An agent investigating an issue must never call `agent report` about itself; the harness reports for it.
- Shell-piping recipes (`jq` filters) in the knowledge files describe CLI usage; with MCP tools, read the same fields from the structured tool result instead.
