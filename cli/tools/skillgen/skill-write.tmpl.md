---
name: traceway-write
description: 'Configure Traceway alerting, SLOs, on-call routing, personal notifications, dashboards and browser collection, issue shared dashboard links and embed dashboards in another application, or perform incident actions through the write CLI or MCP server. Use when the user asks to change Traceway configuration or operational state, or to share or embed a dashboard. Requires a write credential and organization Agent Access.'
---
<!-- GENERATED FILE: assembled from cli/pkg/mcpserver/knowledge by cli/tools/skillgen. Edit the chunks there and run just gen-skills in cli/. -->

# Write to Traceway

Use the connected write MCP server when available, otherwise `traceway-write`. Every read command also exists in the write CLI. For investigation use the read skill; for product explanations use `traceway-learn`. Load only the reference needed for the requested change.

| Area | Read this reference | What can change |
|---|---|---|
| Setup | [write-setup.md](write-setup.md) | Install, authenticate and understand permissions |
| Alerting | [alert-configuration.md](alert-configuration.md) | Channels, delivery tests and project alert rules |
| SLOs | [slo-configuration.md](slo-configuration.md) | Service level objectives: create, preview, change, pause and delete, plus their burn-rate alerts |
| On-call routing | [oncall-configuration.md](oncall-configuration.md) | Teams, ownership, schedules, overrides and escalation policies |
| Personal delivery | [personal-notifications.md](personal-notifications.md) | Own contact methods and high/low urgency notification chains |
| Incident response | [incident-actions.md](incident-actions.md) | Archive issues, acknowledge/resolve pages, snooze rules, run monitors, write post-mortems, mark expected latency |
| Dashboards | [dashboard-building.md](dashboard-building.md) | Discover recorded metrics and build useful charts from instrumentation |
| Sharing and embedding | [dashboard-embedding.md](dashboard-embedding.md) | Issue an expiring read-only dashboard link, and add an embedded dashboard to the user's own application |
| RUM | [rum.md](rum.md) | Browser metrics, release alerts, frontend SLO semantics and dashboard-only capabilities |
| Browser and Flutter collection | [sdk-configuration.md](sdk-configuration.md) | OpAMP rules, sampling, sessions, replay, and recording triggers (widget selectors and route names in Flutter) |

## How the concepts interact

Instrumentation supplies telemetry. Dashboards visualize it; project alert rules watch it and send to a channel. An SLO measures the share of good events against a target; it alerts only through the burn-rate and budget rules that reference it. An ordinary channel delivers directly. An escalation channel opens a page using an organization escalation policy. Policy steps target schedules (one on-call person), teams (all members), individual users or ordinary channels. A schedule's layers rotate responders; overrides temporarily replace them. Each responder's personal contact methods and urgency chain decide how to reach them. Acknowledging the page stops escalation and queued personal deliveries; resolving it allows a new page for the same condition. Changing a policy affects future pages, not an already-open page's policy snapshot.

{{include "operations.md"}}
