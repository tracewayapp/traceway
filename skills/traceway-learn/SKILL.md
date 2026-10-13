---
name: traceway-learn
description: 'The full guide to Traceway, the observability platform. Explains every part of the product and how it is meant to be used: issues and exception grouping, endpoints and the impact score, tasks, spans and traces, logs, metrics and dashboards, sessions, AI traces, profiles, monitors, incidents, status pages, post-mortems, alert rules and channels, on-call teams, schedules, overrides, escalation policies and pages, contact methods, roles, organization views, settings and agent access. Use when the user asks how a Traceway page or feature works, where to find something, what a term means, or how to set up a workflow such as paging or a status page.'
---
<!-- GENERATED FILE: assembled from cli/pkg/mcpserver/knowledge/learn by cli/tools/skillgen. Edit the chunks there and run just gen-skills in cli/. -->

# Learn Traceway

Traceway is an observability platform. Applications send telemetry: OpenTelemetry over OTLP/HTTP from backends, and the Traceway SDKs from browser and mobile apps. Traceway turns it into issues, request and task analytics, traces, logs, metrics and dashboards, session replay, AI cost tracking and profiles. On top of that it runs uptime monitors, public status pages, alerting and on-call paging.

This guide explains every part of the product: what each page shows, the terms it uses, how teams are meant to use it, and how an agent reads or changes it through the `traceway` CLI or the MCP server.

## How to use this guide

- Answer "how does X work", "where do I find X" and "what does X mean" from the topic files below. Read only the one or two the question needs.
- Quote the exact names the dashboard uses (tab names, buttons, rule types), so the user can find them.
- When the user wants live data, switch to the `traceway` skill (or the MCP tools): it holds the debugging and performance playbooks.
- When the user wants something changed, the `traceway-write` skill covers the changes agents are allowed to make. Everything else is done in the dashboard, and the topic files say so.
- When the user wants an application instrumented, use the `traceway-setup` skill.

## Core ideas

- **Organization** owns people, projects, dashboards, on-call and status pages. **Project** is one application, and almost every page is scoped to it. A trace can cross several projects of one organization.
- **Roles**: `owner`, `admin`, `user`, `readonly`, with per-project overrides for `user` and `readonly` members.
- **Time windows**: every telemetry page is bounded by a range kept in the URL (`?preset=24h`, or `?from=&to=`). Presets: 5m, 30m, 60m, 3h, 6h, 12h, 24h, 3d, 7d, 1M, 3M. CLI and MCP reads take `--since` or `--from/--to`.
- **Spans are the primitive.** Endpoints, tasks, AI traces and issues are spans promoted at ingest. One **trace id** links a request to its downstream calls, logs and exceptions.
- **Alert, page, incident** are different things. An **alert** is a rule sending a notification. A **page** is an on-call escalation that keeps going until someone acknowledges it. An **incident** is a period when a monitor was down (or one recorded by hand), with an optional public story on a status page.

## Topics

| Topic | File | MCP resource | Covers |
|---|---|---|---|
| Issues | [issues.md](issues.md) | `traceway://knowledge/learn/issues` | Exceptions, fingerprinting, the issue and occurrence pages, archiving, regressions |
| Endpoints and tasks | [endpoints-and-tasks.md](endpoints-and-tasks.md) | `traceway://knowledge/learn/endpoints-and-tasks` | P50, P95, P99, impact score, Apdex, expected-performance marks, requests, background tasks |
| Spans, traces and logs | [traces-and-logs.md](traces-and-logs.md) | `traceway://knowledge/learn/traces-and-logs` | The span explorer, whole-trace waterfalls, distributed traces, log search, live tail |
| Metrics and dashboards | [metrics-and-dashboards.md](metrics-and-dashboards.md) | `traceway://knowledge/learn/metrics-and-dashboards` | Metric names and tags, aggregations and rate, dashboards, widgets, templates, starring |
| Sessions, AI traces, profiles | [sessions-and-ai.md](sessions-and-ai.md) | `traceway://knowledge/learn/sessions-and-ai` | Session replay, LLM calls, conversations, flagged content, flame graphs |
| Monitors and status pages | [monitors.md](monitors.md) | `traceway://knowledge/learn/monitors` | HTTP, TCP and browser checks, runs, incidents, public status pages, post-mortems |
| Alerts | [alerts.md](alerts.md) | `traceway://knowledge/learn/alerts` | Every rule type and its settings, channels, cooldown, snooze, history |
| On-call | [on-call.md](on-call.md) | `traceway://knowledge/learn/on-call` | Teams, schedules, layers, overrides, escalation policies, pages, contact methods |
| Organization and access | [organization.md](organization.md) | `traceway://knowledge/learn/organization` | Projects, roles, the home page, organization views, Connection, Account, Settings, agent access |

## Page map

| Page | Path | Topic |
|---|---|---|
| Dashboard (home) | `/` | organization.md |
| Issues | `/issues`, `/issues/<hash>` | issues.md |
| Logs | `/logs` | traces-and-logs.md |
| Endpoints | `/endpoints`, `/endpoints/<endpoint>` | endpoints-and-tasks.md |
| Tasks | `/tasks` | endpoints-and-tasks.md |
| Spans | `/spans`, `/spans/<traceId>` | traces-and-logs.md |
| Profiles | `/profiles` | sessions-and-ai.md |
| Sessions | `/sessions` (browser and mobile projects) | sessions-and-ai.md |
| AI Traces | `/ai-traces` | sessions-and-ai.md |
| Dashboards | `/dashboards` | metrics-and-dashboards.md |
| Monitors | `/monitors` (tabs Monitors, Status Pages, Post-Mortems) | monitors.md |
| Public status page | `/status/<slug>` | monitors.md |
| Alerts | `/notifications` (tabs Channels, Rules, History) | alerts.md |
| On-Call | `/on-call` (tabs Pages, Overview, Teams, Schedules, Policies) | on-call.md |
| Connection | `/connection` | organization.md |
| Account | `/account` | organization.md, on-call.md |
| Settings | `/settings` (owners and admins) | organization.md |
| Organization | `/organization` (Servers, Issues, Monitors, On-Call, Projects) | organization.md |
| Project setup | `/setup` | organization.md |

Browser and mobile projects only show Issues, Sessions, Alerts, On-Call and Connection.

## Common questions

| Question | Answer in |
|---|---|
| Why did two errors land in the same issue, or one error split in two? | issues.md, Fingerprinting |
| What does the impact score mean and why is this endpoint Critical? | endpoints-and-tasks.md |
| What happens when I acknowledge a page? Who gets paged next? | on-call.md, Pages and Escalation policies |
| Who is on call right now, and how do I swap a shift? | on-call.md, Schedules and Overrides |
| Snooze or disable a noisy alert? | alerts.md, Disable versus snooze |
| Why is my monitor down after one failure, or not down after several? | monitors.md, States and runs |
| How do I publish uptime to customers? | monitors.md, Status pages |
| Where do I write up the incident? | monitors.md, Post-mortems |
| Why is a span missing from the waterfall? | traces-and-logs.md |
| Why does my counter metric only go up? | metrics-and-dashboards.md, Aggregations |
| Can an agent change things in Traceway? | organization.md, How agents get access |
