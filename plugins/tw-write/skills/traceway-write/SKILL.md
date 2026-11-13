---
name: traceway-write
description: 'Configure Traceway alerting, on-call routing, personal notifications, dashboards and browser collection, or perform incident actions through the write CLI or MCP server. Use when the user asks to change Traceway configuration or operational state. Requires a write credential and organization Agent Access.'
---
<!-- GENERATED FILE: assembled from cli/pkg/mcpserver/knowledge by cli/tools/skillgen. Edit the chunks there and run just gen-skills in cli/. -->

# Write to Traceway

Use the connected write MCP server when available, otherwise `traceway-write`. Every read command also exists in the write CLI. For investigation use the read skill; for product explanations use `traceway-learn`. Load only the reference needed for the requested change.

| Area | Read this reference | What can change |
|---|---|---|
| Setup | [write-setup.md](write-setup.md) | Install, authenticate and understand permissions |
| Alerting | [alert-configuration.md](alert-configuration.md) | Channels, delivery tests and project alert rules |
| On-call routing | [oncall-configuration.md](oncall-configuration.md) | Teams, ownership, schedules, overrides and escalation policies |
| Personal delivery | [personal-notifications.md](personal-notifications.md) | Own contact methods and high/low urgency notification chains |
| Incident response | [incident-actions.md](incident-actions.md) | Archive issues, acknowledge/resolve pages, snooze rules, run monitors, write post-mortems, mark expected latency |
| Dashboards | [dashboard-building.md](dashboard-building.md) | Discover recorded metrics and build useful charts from instrumentation |
| Browser collection | [sdk-configuration.md](sdk-configuration.md) | OpAMP rules, sampling, sessions and replay |

## How the concepts interact

Instrumentation supplies telemetry. Dashboards visualize it; project alert rules watch it and send to a channel. An ordinary channel delivers directly. An escalation channel opens a page using an organization escalation policy. Policy steps target schedules (one on-call person), teams (all members), individual users or ordinary channels. A schedule's layers rotate responders; overrides temporarily replace them. Each responder's personal contact methods and urgency chain decide how to reach them. Acknowledging the page stops escalation and queued personal deliveries; resolving it allows a new page for the same condition. Changing a policy affects future pages, not an already-open page's policy snapshot.

## Operating with write access

Use a write credential and an organization whose admin enabled Agent Access. Project writes require the caller's project write role; organization configuration requires owner/admin. Any member can create schedule overrides; deleting one requires its creator, covering user or an admin. Personal contacts and notification chains belong only to the authenticated person. Acknowledging and resolving pages need project read access, but agent calls still need a write credential and the organization opt-in.

Only make changes the user requested. Inspect the current configuration, preserve unrelated settings, state the intended change, then apply it. Existing authorization covers the requested work; do not repeatedly ask for permission. In noninteractive CLI calls use `--yes`. Testing a channel or contact sends a real notification; testing an escalation channel opens a real page. Do not test delivery merely to verify that configuration saved.

Read back the result and report the effect and any remaining verification. A 403 is a policy decision: explain whether the credential, organization opt-in or caller role blocked it; do not bypass it. Agent writes are audited. Treat channel/contact configuration as sensitive: do not echo tokens or webhook URLs into summaries.

Read the relevant resource before configuring an area:

- `traceway://knowledge/write-setup`: installation, login and permission layers.
- `traceway://knowledge/alert-configuration`: channels and project alert rules.
- `traceway://knowledge/oncall-configuration`: teams, schedules, overrides and escalation policies.
- `traceway://knowledge/personal-notifications`: own contacts and high/low urgency notification chains.
- `traceway://knowledge/incident-actions`: pages, issues, monitor runs, post-mortems and expected latency.
- `traceway://knowledge/dashboard-building`: dashboard charts and instrumentation discovery.
- `traceway://knowledge/sdk-configuration`: browser collection, replay and OpAMP rules.

For product behavior and tradeoffs use `traceway://knowledge/learn/alerts`, `traceway://knowledge/learn/on-call` and the other learn resources.
