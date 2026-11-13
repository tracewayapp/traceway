<!-- GENERATED FILE: copied from cli/pkg/mcpserver/knowledge/dashboard-building.md by cli/tools/skillgen. Edit there and run just gen-skills in cli/. -->
# Build dashboards from an instrumented codebase

Use this guide when asked to build or improve observability dashboards. Dashboard authoring is available through the `traceway-write` CLI and the write MCP server (`import_dashboards`). Read commands also work in `traceway`. A write credential, organization agent-write opt-in and write access are required. Do not change instrumentation or create projects merely to fill a chart unless that work was requested.

## Find useful signals

Inspect the application's metric declarations AND recording sites: instrument name, unit, counter/gauge/histogram, cumulative versus delta semantics, attributes, recording frequency, and which success/failure paths record it. Inspect exporter/collector configuration for renaming, filtering and service names. A span or log named `checkout` does not imply a metric with that name exists.

Map the code's important operations to questions: Is work arriving? Is it completing? Are failures growing? Is a dependency slowing? Is capacity running out? For a worker, consider enqueue/completion throughput, retries, queue depth and processing duration. For an API, consider traffic, failures, duration and pool saturation. For business workflows, chart recorded outcomes, not guessed events. Select signals tied to an action someone can take; do not add every runtime metric by default.

Verify the deployment with the selected project and a representative time range:

```bash
traceway-write metrics discover --since 24h --output json
traceway-write metrics discover --name jobs.completed --key outcome --since 24h --output json
traceway-write metrics query --name jobs.completed --aggregation rate --group-by outcome --since 24h --output json
traceway-write dashboards list --output json
traceway-write dashboards library --output json
```

Discovery returns names, tag keys, stored types and units; tag-value discovery needs both `--name` and `--key`. Treat source code as intent and discovery/query results as evidence of what arrived. If empty, check the project, time range, deployment and recording path; do not present no data as zero or fabricate a metric. Report missing instrumentation separately. Record a compact mapping of each proposed chart to its source location, observed metric, aggregation, filters and operational question.

Prefer bounded dimensions such as outcome, queue, normalized route, dependency or `server_name`. Avoid request/user/order IDs and raw URLs. Grouped queries cap at 200 groups (`truncatedGroups`); narrow filters when truncated. Tags filter by exact equality. Multiple sources are independent series, not a formula language: ratios, arbitrary SQL, PromQL and joins are not supported in widgets.

## Pick the chart and aggregation

| `widgetType` | Use it for | Actual rendering behavior / limits |
|---|---|---|
| `line_chart` | Trends, spikes, latency averages, comparing instances or outcomes | One line per source/group; compare compatible units on the shared axis. Good default. |
| `area_chart` | Magnitude over time, such as backlog or memory | Filled time series; overlapping fills can obscure groups. |
| `stacked_area` | Contributions to an additive total, such as throughput by outcome | Stack mutually exclusive series with the same unit. Do not stack latency averages, percentiles, overlapping totals or unrelated gauges. |
| `bar_chart` | Comparing instances/queues at the end of the range | Horizontal bars use the latest bucket of each series, not totals over the range. |
| `single_value` | One headline measure, optionally with `showSparkline` | Only the first returned series is reduced. `avg` averages buckets, `sum`/`count` sum buckets, `min`/`max` take extrema; `last`/`rate` use the latest bucket. Avoid group-by or multiple sources here. |
| `gauge` | A bounded quantity with meaningful capacity/thresholds | Same first-series reduction as single value. Set `min`/`max` in the actual metric unit; never invent a capacity. |
| `table` | Latest/min/max/average summary of one series | Currently summarizes ONLY the first series across time; it is not a table of all groups or raw events. |

- **Cumulative counters:** `rate` computes per-second increases with reset handling. Use for requests, bytes or jobs only after verifying cumulative semantics. It needs previous samples; a newly started or sparse series may lack a rate. `sum` of cumulative snapshots is not an event total.
- **Gauges:** `avg` for typical level, `max` for worst saturation, `last` for the latest bucket value, `min` for remaining capacity floors. Understand whether aggregating across instances hides an overloaded instance; use group-by when it does.
- **Delta increments:** `sum` gives bucket totals; `count` counts stored points, not the events encoded in their values. Do not apply cumulative-counter `rate` blindly to delta values.
- **Histograms:** Traceway stores `<name>.avg` and `<name>.count`. Discover their actual names and semantics. The average is not a tail percentile; averaging averages is not necessarily a weighted request mean. Metric widgets cannot compute p50/p95/p99. Use endpoint/task analysis for latency percentiles, or report that the desired dashboard signal is unavailable.
- **Units:** preserve discovered units. A `unit` override changes display, not numeric scale. A fraction uses bounds 0..1; a percent uses 0..100. `complement: true` displays 1 minus the value (100 minus it when the returned unit is `%`), useful for CPU idle → busy. It is not an arbitrary conversion.

Start with workload health, then dependencies and capacity, then diagnostic breakdowns. Put the question and unit in clear titles, distinguish averages from latest/peak values, and use thresholds only when backed by an SLO, code limit or operator-provided capacity.

## JSON contract and example

Import a single document or `{ "schemaVersion": 1, "dashboards": [ ...documents ] }`. Each document has `schemaVersion: 1`, `name`, optional `description`, and `widgets` in display order. Use stable unique widget IDs of at most 20 letters/digits/underscores/hyphens; preserve existing IDs so home-page stars survive. Maximum 100 widgets per dashboard, 32 KiB per config.

Each widget has `id`, `title`, `widgetType`, and `config`. `config.sources` contains `{ "type": "metric", "name": "...", "aggregation": "..." }` with optional `tagFilters`, `groupBy`, `label`, `complement`. Layout uses `colSpan` (1, 2 or 3) and `size` (`sm`, `md`, `lg`); visual options include `showLegend`, `yAxisLabel`, `unit`, `showSparkline`, `min`, `max`, `baseColor`, and `thresholds: [{ "value": 80, "color": "#f59e0b" }]`.

This example assumes discovery confirmed a cumulative `jobs.completed` counter with `outcome`, a `jobs.queue.depth` gauge with `queue`, and `jobs.duration.avg` in seconds. Replace these with verified names; it is not a universal template.

```json
{
  "schemaVersion": 1,
  "name": "Worker health",
  "description": "Completion throughput, backlog and mean processing duration",
  "widgets": [
    {
      "id": "jobs_throughput",
      "title": "Completed jobs per second by outcome",
      "widgetType": "stacked_area",
      "config": {
        "colSpan": 2,
        "showLegend": true,
        "sources": [{"type": "metric", "name": "jobs.completed", "aggregation": "rate", "groupBy": "outcome"}]
      }
    },
    {
      "id": "queue_depth",
      "title": "Queue depth by queue (peak per bucket)",
      "widgetType": "line_chart",
      "config": {
        "sources": [{"type": "metric", "name": "jobs.queue.depth", "aggregation": "max", "groupBy": "queue"}]
      }
    },
    {
      "id": "job_duration",
      "title": "Mean processing duration (seconds)",
      "widgetType": "line_chart",
      "config": {
        "sources": [{"type": "metric", "name": "jobs.duration.avg", "aggregation": "avg"}]
      }
    }
  ]
}
```

## MCP equivalents

Use `discover_metrics` (optional `name` + `key` for tag values), `query_metrics`, `list_dashboards`, `dashboard_library`, and `export_dashboard`. These are read-only tools on both servers. `import_dashboards` exists only on the write server. Pass `project_id` when the server has no default project, plus `data` containing the document or bundle as a JSON object and optional `mode` (`create` or `upsert`). Example tool arguments:

```json
{"project_id":"<project-uuid>","mode":"create","data":{"schemaVersion":1,"name":"Worker health","widgets":[{"id":"queue_depth","title":"Queue depth","widgetType":"line_chart","config":{"sources":[{"type":"metric","name":"jobs.queue.depth","aggregation":"max"}]}}]}}
```

Both CLI import and MCP import validate document shape, names, chart types, metric sources, supported aggregations and size limits before sending a write. `dashboards validate --file file.json` runs the same checks offline without login. Validation does not prove a metric exists or that permissions allow saving; query the sources and verify the saved export.

## Save and verify

```bash
traceway-write dashboards validate --file worker-health.json
traceway-write dashboards import --file worker-health.json --yes --output json
traceway-write dashboards export 42 --output json > worker-health.saved.json
```

Import defaults to `--mode create`: it creates copies, suffixing duplicate names with ` (imported)`, and applies them to the selected project. It is not idempotent; after an ambiguous timeout, list dashboards before retrying. The response includes created/updated counts and dashboard IDs. Use the returned ID rather than the illustrative `42`.

Use `dashboards library` to inspect organization-wide names and `appliedProjectIds` before upsert; `dashboards list` only shows the selected project's applied dashboards. For an existing dashboard, export it first, preserve unrelated widgets and their IDs, edit the document, then use:

```bash
traceway-write dashboards import --file worker-health.json --mode upsert --yes --output json
```

Upsert matches names case-insensitively across the selected project's organization and replaces the ENTIRE matching definition and description. Dashboards are shared: all projects using that dashboard see the edit. It also applies the dashboard to the selected project. There is no optimistic-concurrency protection: re-export before saving if someone may have edited it, and reconcile changes. Keep the original export for restoration by upsert; creating a separately named copy is appropriate when the user requested a project-specific variant. Removing an assignment or deleting a dashboard still uses the UI.

Pass `--yes` only for a user-requested change. State the dashboard name, selected project and shared edit scope before writing. Agent imports cannot override `organizationId` or `applyToProjectIds`; the CLI sends only the document and mode. A 403 is a policy decision, not a reason to try another route.

After saving, export and compare the saved sources/configuration, and query every source with its exact aggregation, filters and grouping. The CLI query does not apply visual `complement`; account for that when checking values. Check representative traffic and quiet periods, cardinality, units, and whether a chart is empty. Report created/updated IDs, questions the charts answer, any unverified signals, and how to restore the previous export. Successful JSON import alone does not prove that the charts have useful data.
