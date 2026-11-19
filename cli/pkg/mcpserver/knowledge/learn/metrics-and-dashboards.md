# Metrics and dashboards

## Metrics

A metric point is a name (`system.cpu.utilization`, `orders.created`), a numeric value, a timestamp and tags (`server_name`, `state`, `direction`, ...).

- **Where they come from**: OpenTelemetry meters in the application (counters, histograms, gauges), runtime instrumentation (Go, Node.js and others), and host metrics from the Traceway OTel Agent or an OpenTelemetry Collector with the hostmetrics receiver (`system.*` names: CPU, memory, disk, filesystem, network, processes).
- **Histograms** are stored as mergeable distributions, including OTLP exponential histograms. Explicit-bound backend histograms also retain `<name>.avg` and `<name>.count` series. Browser SDK 2.x metrics aggregate on-device; see [rum.md](rum.md).
- **Tags** come from the data point's attributes, plus an allowlist of resource attributes Traceway lifts: `host.name`, `host.id`, `os.type`, `cloud.region`, `container.name`, `k8s.cluster.name`, `k8s.pod.name`, `k8s.node.name`, `postgresql.database.name` and a few more. The `server_name` tag comes from `service.name`. The allowlist also covers `k8s.namespace.name`, `k8s.deployment.name`, `cloud.provider`, `cloud.availability_zone` and `process.*`.
- **Resolution**: on ClickHouse it follows the range so long ranges stay fast: raw points up to 6 hours, 1-minute rollups up to 72 hours, 1-hour rollups up to 30 days, daily beyond. SQLite and DuckDB always read raw points.
- **Stored types**: backend gauges, sums and histograms; summaries are dropped. Client projects accept catalog/custom delta sums and delta exponential histograms, not arbitrary gauges or cumulative series. Distribution rollups differ from point metrics: ClickHouse uses minute/hour/day storage; SQLite and DuckDB fold minutes older than six hours into hours.

Aggregations:

| Aggregation | Use |
|---|---|
| `avg`, `min`, `max`, `sum`, `count`, `last` | Gauges and per-bucket values. `max` is the honest choice when one bad series could hide behind healthy ones in an `avg`. |
| `rate` | Cumulative counters such as `system.network.io` and `system.disk.io`. Each series is differenced against its previous sample, resets are dropped, and the result is per second (`By` becomes `By/s`). Charting a counter as `avg` only climbs forever. |

A widget source can also be **inverted** (1 minus the value). The agent reports `system.cpu.utilization` per state, and `idle` inverted is the busy share.

Distribution queries support p50/p75/p90/p95/p99 and `good_ratio`; point metrics cannot supply percentiles. Merge buckets over the whole window before calculating a headline percentile, never average percentiles. Distribution `count` counts observations; delta-counter `sum` counts events. Query good ratios use 0..1; alert thresholds use 0..100. See [rum.md](rum.md) for current CLI/MCP limitations.

## Dashboards (`/dashboards`)

A dashboard is a set of metric widgets that belongs to the **organization**, not to one project. It is applied to any number of projects and shows as a tab on each of them. An edit made in one project is live in all of them.

The active tab's menu: **Rename Dashboard** (names are unique in the organization), **Edit as JSON**, **Export JSON**, **Apply to Projects**, **Remove from this Project** (it stays elsewhere), **Delete Dashboard** (everywhere, permanently).

**Adding**: ⌘K (Ctrl K) or **Add or find dashboards...** opens one search over the dashboards of every organization the user belongs to (selecting one applies it here, or copies it in from another organization), the template marketplace (selecting one installs a copy) and every metric name seen in the organization (selecting one opens the widget editor prefilled). An empty project shows **Browse Dashboards & Templates**.

**Templates**:

| Template | Metrics it expects |
|---|---|
| OTelemetry Server Agent | Host CPU, memory, disk and network from the Traceway OTel Agent |
| Go Application | Go runtime and process metrics |
| Traceway ClickHouse | `traceway.ch.*` from a monitored Traceway instance |
| Traceway DuckDB | `traceway.duckdb.*` from a monitored Traceway instance |

**Widgets**: Line Chart, Area Chart, Stacked Area, Bar Chart, Single Value, Gauge, Table, Rating Bar and Breakdown Table. Rating bars show rated distribution shares; breakdown tables compare groups over the whole range. Distribution headline values merge the full range first. Each has one or more sources: a metric name, an aggregation, optional tag filters (equals), an optional group-by that draws one series per tag value, and optionally inverted. Several sources overlay on one chart. Hover for values, drag across a chart to zoom in.

**Instance filter**: the picker in the header narrows every widget to one `server_name` seen in the range. It lives in the URL (`?server=<name>`), is not saved to the dashboard, and is where a row on the organization's Servers page leads.

**Starring**: the star on a widget pins it to the project's home page, where starred widgets can be resized and reordered. Stars are per project.

**As code**: every dashboard is one JSON document (`schemaVersion`, `name`, `widgets` in display order, each with `id`, `title`, `widgetType`, `config.sources`). Edit it in place, export it, or import a single dashboard or a whole organization bundle. The import API has an upsert mode that matches by name, so CI can keep dashboards in sync from files. Keep widget ids when editing so home page stars survive.

**Grafana import**: paste a Grafana export. Time series and graph panels become line charts, stat becomes single value, gauge stays a gauge, and Prometheus queries become metric sources. Anything that does not convert faithfully is listed as a warning, never silently dropped.

Editing dashboards and widgets needs an organization role above `readonly`. Starring, tab order and home page layout follow the effective project role. Browser projects receive Web Performance on creation; mobile projects offer Mobile Performance on the empty dashboard page. Client dashboards filter by release, OS and country (when configured), rather than server instance. Slow sessions opens retained distribution exemplars, which may have element attribution and session links.

## Reading it from an agent

| Task | CLI | MCP tool |
|---|---|---|
| A metric series | `traceway metrics query --name <metric> --aggregation avg --since 1h [--tag key=value] [--group-by tag] [--interval-minutes N]` | `query_metrics` |

- An unknown metric name returns an empty series, so probing names is safe.
- A grouped query keeps the first 200 groups and says `truncatedGroups: true` when it cut some.
- The server widens buckets so no series has more than 2000 points, and returns the bucket size it used as `intervalMinutes`.
- Agents can query `avg`, `sum`, `count`, `min`, `max`, `rate` and `last`. Inversion is a widget display setting.
- Current MCP `query_metrics` refuses percentiles; CLI accepts p50/p95/p99 and the current server computes them for distributions. Neither surface currently exposes the full RUM query contract (`wholeRange`, p75, p90, `good_ratio`, threshold). Use the dashboard for those. Inspect API query errors instead of treating empty CLI results as health.
- Use `traceway metrics discover --since 24h` for recorded names, types, units and tag keys; add `--name <metric> --key <tag>` for tag values. The MCP equivalent is `discover_metrics` with optional `name` and `key`.

Use `traceway-write dashboards import --file dashboard.json` to create and apply dashboards, or `--mode upsert` to replace by organization-wide name. Read `dashboard-building.md` in the `traceway-write` skill for chart selection from instrumented code, JSON examples and verification. Read-only `dashboards list` and `dashboards export <id>` are available in both CLIs.

MCP supports `list_dashboards`, `dashboard_library`, `export_dashboard` and, on the write server, `import_dashboards`. CLI `dashboards validate --file dashboard.json` validates offline with the same checks used by CLI and MCP imports.
