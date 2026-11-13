<!-- GENERATED FILE: copied from cli/pkg/mcpserver/knowledge/incident-actions.md by cli/tools/skillgen. Edit there and run just gen-skills in cli/. -->
# Incident actions


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
3. If the user requested a saved post-mortem, create the reviewed document: `post-mortems create --title "..." --content-file draft.md --tag <tag> --incident <id>`.
4. Later edits: `post-mortems show <id>` first, then `post-mortems update <id> --content-file revised.md --expected-updated-at <updatedAt from that show>` with the full revised body. If someone edited it in between, the update fails with a conflict: show it again and reapply the change. `--unlink-incident` removes a wrong incident link.

#### An endpoint is slow by design

1. Confirm with the user that the latency is accepted, and get the reason.
2. Check the current p95 with `endpoints list --search "<endpoint>"`.
3. `endpoints mark-slow "<endpoint>" --offset-ms <a bit above p95> --reason "<why>"`. Raw percentiles do not change. Apdex and impact stop penalizing up to the offset.


## Command mapping


Every `traceway` read command works in `traceway-write` (`exceptions list|show|occurrence`, `logs query`, `endpoints list|chart|show|slow`, `tasks show`, `ai-traces show`, `sessions show`, `traces show`, `metrics query`, `pages list|show`, `alerts list`, `monitors list|show`, `post-mortems list|show`). The write commands:

| Command | MCP tool | What it changes |
|---|---|---|
| `traceway-write exceptions archive <hash>... [--yes]` | `archive_exceptions` | Hides issues from the default list |
| `traceway-write exceptions unarchive <hash>... [--yes]` | `unarchive_exceptions` | Brings them back |
| `traceway-write pages ack <id>` | `acknowledge_page` | Stops escalation for a page |
| `traceway-write pages resolve <id> [--archive-issue]` | `resolve_page` | Closes a page, optionally archiving its issue |
| `traceway-write alerts enable <id>` / `disable <id>` | `set_notification_rule_enabled` | Turns a rule on or off |
| `traceway-write alerts snooze <id> --minutes N` | `snooze_notification_rule` | Silences a rule for N minutes; 0 clears |
| `traceway-write monitors run <id>` | `run_check` | Queues an immediate monitor run |
| `traceway-write post-mortems create --title T [--content-file F] [--tag X] [--incident N]` | `create_post_mortem` | New post-mortem |
| `traceway-write post-mortems update <id> [--title] [--content-file --expected-updated-at] [--tag] [--incident \| --unlink-incident]` | `update_post_mortem` | Edits only the flags passed. A new body needs the `updatedAt` it was based on |
| `traceway-write endpoints mark-slow <endpoint> --offset-ms N --reason R` | `set_slow_endpoint` | Accepted latency for an endpoint; 0 removes |

Every write command prints what it is about to do and asks for confirmation on a TTY. In scripts and agents pass `--yes` (or set `TRACEWAY_ASSUME_YES=1`) only after the user asked for that exact change. Output, flags, exit codes and error envelopes are the same as the read CLI: JSON when piped, `--fields` to trim, exit code 4 for auth and permission errors.

