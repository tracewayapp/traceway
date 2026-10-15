## Operating with write access

Write access lets an agent take the routine actions an on-call engineer takes during and after an incident. It is opt-in at three levels, and every level has to agree before a write goes through:

1. **The credential.** Only write credentials can write: a `tww_` personal access token, a `traceway-write login`, or an MCP client connected to the `/mcp/write` server. A `twp_` token, a `traceway login` and the `/mcp` server are read only, whatever the person's role.
2. **The organization.** An admin has to turn on Settings > Agent Access. Until then every agent write is refused with a 403 that says so.
3. **The person.** The write runs with the role of the person who created the credential, under the same rules as the dashboard. A `readonly` member's write token cannot archive issues, change alerts, run monitors or edit post-mortems. Acknowledging and resolving pages needs no write role, so any member's write token can do that.

Every write a write credential makes in the organization, accepted or refused because Agent Access is off, is recorded in the organization's agent activity log with the person, the credential, the project and the result.

### What an agent can change

| Action | CLI (`traceway-write`) | MCP tool | Reversible |
|---|---|---|---|
| Archive or unarchive issues | `exceptions archive <hash>...` / `unarchive` | `archive_exceptions` / `unarchive_exceptions` | Yes |
| Acknowledge a page | `pages ack <id>` | `acknowledge_page` | No, but harmless: it only stops escalation |
| Resolve a page | `pages resolve <id> [--archive-issue]` | `resolve_page` | No |
| Enable or disable an alert rule | `alerts enable <id>` / `alerts disable <id>` | `set_notification_rule_enabled` | Yes |
| Snooze an alert rule | `alerts snooze <id> --minutes N` (0 clears) | `snooze_notification_rule` | Yes, it expires on its own |
| Run a monitor now | `monitors run <id>` | `run_check` | Nothing to undo |
| Write a post-mortem | `post-mortems create` / `update <id>` | `create_post_mortem` / `update_post_mortem` | Edits are kept in the activity log |
| Mark an endpoint expected-slow | `endpoints mark-slow <endpoint> --offset-ms N --reason ...` (0 removes) | `set_slow_endpoint` | Yes |

Everything else (creating rules and channels, editing schedules and policies, status pages and public incident updates, members, tokens, project settings, deleting anything) stays in the dashboard. Those routes refuse agent credentials outright.

### Rules for every write

- **Only on request.** Investigating, debugging, summarizing or "taking a look" never implies a write. The user has to ask for the change.
- **Say it before doing it.** One line naming the action, the record and the new value: "Snoozing rule 12 (Endpoint P95 on GET /api/search) for 60 minutes." The CLI asks for confirmation on a TTY and needs `--yes` otherwise. MCP clients run their own approval step.
- **Prefer the reversible option.** Snooze rather than disable. Acknowledge rather than resolve while the problem is still live. Archive an issue rather than resolving its page with `--archive-issue` if you are unsure the page is done.
- **Report what changed and how to undo it.**
- **Treat a 403 as an answer.** "This credential is read-only" or "Agent write access is turned off" is a policy decision. Do not retry, switch credentials or look for another route. Tell the user what it said and who can change it.

### Playbooks

#### Someone got paged

1. `pages list` (status `active`) to find the page, then `pages show <id>` for the rule, subject and who was notified.
2. Investigate with the read tools. For an issue page, the page carries the exception hash: follow the debug flow.
3. When the user says they are on it: `pages ack <id>`. Escalation stops for everyone.
4. When the cause is fixed and confirmed: `pages resolve <id>`, with `--archive-issue` only if the user wants the issue archived too.

#### An alert is flapping during a known incident

1. `alerts list` to find the rule id and confirm it is the noisy one.
2. `alerts snooze <id> --minutes 60` (or as long as the user asks). The rule is muted and comes back on its own.
3. Tell the user when the snooze ends. Disable a rule only when the user wants it off until someone turns it back on.

#### Confirming a fix

1. `monitors run <id>` for the affected monitor.
2. Wait about one interval, then `monitors show <id>`: the latest run and whether the open incident resolved.
3. For an issue, `exceptions show <hash>` and check that no occurrence is newer than the deploy. Archive it only when the user agrees it is fixed.

#### Writing a post-mortem

1. Gather facts with the read tools: the incident window from `monitors show`, the issues and their first and last occurrences, the pages and who acknowledged them, the slow endpoints and the trace of a failing request.
2. Draft the document in markdown with these sections: Summary, Impact (who and how much, with numbers), Timeline (UTC, from first signal to resolution), Root cause, Resolution, Follow-up actions (each with an owner if known). Only state what the telemetry or the user confirmed, and mark open questions as open.
3. Show the draft to the user. Create it only after they approve: `post-mortems create --title "..." --content-file draft.md --tag <tag> --incident <id>`.
4. Later edits: `post-mortems show <id>` first, then `post-mortems update <id> --content-file revised.md --expected-updated-at <updatedAt from that show>` with the full revised body. If someone edited it in between, the update fails with a conflict: show it again and reapply the change. `--unlink-incident` removes a wrong incident link.

#### An endpoint is slow by design

1. Confirm with the user that the latency is accepted, and get the reason.
2. Check the current p95 with `endpoints list --search "<endpoint>"`.
3. `endpoints mark-slow "<endpoint>" --offset-ms <a bit above p95> --reason "<why>"`. Raw percentiles do not change. Apdex and impact stop penalizing up to the offset.
