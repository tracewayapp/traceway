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
