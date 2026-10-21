---
name: traceway-write
description: 'Operate a Traceway instance with write access. Use when the user wants an agent to change something in Traceway: archive or unarchive issues, acknowledge or resolve on-call pages, snooze, enable or disable alert rules, run a monitor now, create or edit a post-mortem, or mark an endpoint as expected-slow. Requires the separate traceway-write CLI or the /mcp/write server, a write credential, and an organization admin who turned on agent writes.'
---
<!-- GENERATED FILE: assembled from cli/pkg/mcpserver/knowledge by cli/tools/skillgen. Edit the chunks there and run just gen-skills in cli/. -->

# Traceway with write access

This skill drives the `traceway-write` CLI (or the write MCP server) to make the routine changes an on-call engineer makes: archive issues, acknowledge and resolve pages, snooze or toggle alert rules, run monitors, write post-mortems and mark slow endpoints.

To explain how a part of Traceway works before or after a change (what acknowledging a page does to escalation, how a snooze differs from disabling a rule, how a monitor opens an incident), use the `traceway-learn` skill. It is the full product guide. Over MCP the same guide is the `traceway://knowledge/learn` resources.

For pure investigation (debugging an issue, latency analysis), the read-only `traceway` skill has the deeper playbooks. Its commands work the same way here: every read command of `traceway` also exists in `traceway-write`.

> If a `traceway-write` MCP server is connected (stdio `traceway-write mcp`, or `https://<instance>/mcp/write`), prefer its tools over shelling out. Its resources carry this skill's knowledge, and its tool names are listed in the table below.

## Setup

### 1. Install the write CLI

`traceway-write` is a separate binary with its own login, so a read-only `traceway` install can never be upgraded to write in place.

```bash
curl -fsSL https://cli.tracewayapp.com/install.sh | sh -s -- --write
traceway-write version
```

Prebuilt archives are on the [releases page](https://github.com/tracewayapp/traceway/releases) under `cli/vX.Y.Z` tags as `traceway-write_<version>_<os>_<arch>.tar.gz`. From source: `cd traceway/cli && go build -tags write -o bin/traceway-write ./cmd/traceway`.

### 2. Log in with write access

Ask the user to run the browser login themselves (in Claude Code: `! traceway-write login --url https://<instance>`). The approval page says the login asks for read and write access.

```bash
traceway-write login --url https://<instance>
```

Headless alternative: the user creates a **Read and write** personal access token (Account > Personal Access Tokens, prefix `tww_`, expires within 90 days) and stores it without echoing it:

```bash
printf '%s' "$TRACEWAY_WRITE_TOKEN" | traceway-write login --url https://<instance> --token-stdin
```

A read token (`twp_`) stored in `traceway-write` still cannot write: the server refuses every write with "This credential is read-only".

### 3. Organization opt-in

An organization owner or admin must turn on **Settings > Agent Access > Allow agent writes**. Until then writes answer 403 "Agent write access is turned off for this organization". Do not try to work around it: tell the user who can turn it on.

### 4. Select a project

```bash
traceway-write projects list
traceway-write projects use <project-id>
```

### MCP instead of the CLI

- Local stdio: `claude mcp add traceway-write -- traceway-write mcp` (uses the `traceway-write login`).
- Remote: add `https://<instance>/mcp/write` as an HTTP MCP server. The client runs OAuth and the consent page asks for read and write access. The plain `/mcp` server never exposes write tools, even to a write token.

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
- **Ask when the request could mean more than the user knows.** When a write would cover several records and they differ in a way the user may not know (one is still occurring, has a different cause, or belongs to another team), list them, say how they differ, and ask which ones to change before writing anything. Do not settle it yourself, even by leaving the odd one out.
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

## Command reference

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

## Resolving Dashboard URLs

Users paste dashboard URLs (`https://<instance>/<route>`) as references in any flow. Resolve by route family:

| URL path | Identifies | How to fetch it |
|---|---|---|
| `/issues/<hash>` and `/issues/<hash>/events` | Exception group (hash = 16 hex chars) | `traceway exceptions show <hash>` |
| `/issues/<hash>/<occurrenceId>` (UUID) | One occurrence within the group | `traceway exceptions occurrence <occurrenceId> --recorded-at <t>` where `t` is the URL's `?t=` param. Direct and fast; also returns the occurrence's `sessionId` and session recording. No URL? get `recordedAt` from `traceway exceptions show <hash>` occurrences |
| `/endpoints/<endpoint>` | Endpoint group; the segment is the URL-encoded endpoint name (`GET%20%2Fapi%2Fusers%2F%3Aid` is `GET /api/users/:id`) | Decode it, then `traceway endpoints list --search "<decoded name>"` (the group has no id; `endpoints show` is for one request, see the next row) |
| `/endpoints/<endpoint>/<endpointId>` | One request (transaction) of that endpoint | `traceway endpoints show <endpointId> --recorded-at <t>` (`t` = the URL's `?t=` param). Returns the request, its span waterfall, and any linked exception/messages |
| `/tasks/<task>` | Background task group | No CLI for the group; for one run use the next row |
| `/tasks/<task>/<taskId>` | Single task run | `traceway tasks show <taskId> --recorded-at <t>` (`t` = the URL's `?t=` param) |
| `/sessions/<sessionId>` | Session (the exceptions that fired during it; replay stays dashboard-only) | `traceway sessions show <sessionId> --started-at <t>`. The URL has no `?t=`; use the session's start, the URL's `from=`, or a linked occurrence's `recordedAt` (it falls inside the window). Occurrences reference sessions via their `sessionId` |
| `/ai-traces/<traceName>` | AI trace group | No CLI for the group; for one trace use the next row |
| `/ai-traces/<traceName>/<traceId>` | Single AI trace | `traceway ai-traces show <aiTraceId> --recorded-at <t>` (the UUID in the URL, not a 32 hex trace id) (`t` = the URL's `?t=` param); returns token/cost stats + the conversation |
| `/logs` | Logs page (its filters are not stored in the URL) | `traceway logs query` with flags taken from the user's description |
| `/issues`, `/endpoints`, `/metrics`, `/` | List and dashboard pages | The matching `list` / `query` command |

**Time window**: most dashboard URLs carry `?preset=<p>` or `?from=<iso>&to=<iso>` (sticky across pages); honor them instead of the default window.

- `preset` values `5m 30m 60m 3h 6h 12h 24h 3d 7d` map directly to `--since`; the CLI has no month unit, so map `1M` to `--since 30d` and `3M` to `--since 90d`.
- `from`/`to` are ISO timestamps; pass via `--from`/`--to`, appending `Z` (or the correct offset) when missing, since the CLI requires RFC3339.
- No time params means the page was on its default; pick `--since` per the ground rules.

`preset`/`from`/`to` set the *window* for list/group views. Detail URLs additionally carry **`?t=<iso>`**, the single record's timestamp, URL-encoded. That `t` value is exactly what the by-id commands need as `--recorded-at` (or `--started-at` for sessions). See "Fast by-id lookups" next.

## Fast by-id lookups (always pass the timestamp)

The by-id detail commands (`exceptions occurrence`, `endpoints show`, `tasks show`, `ai-traces show`, `sessions show`, `traces show`) **require** the record's timestamp (`--recorded-at`, or `--started-at` for sessions). Telemetry tables are partitioned by day: with the timestamp the lookup is bounded to a small window and ClickHouse prunes to a few partitions; without it the server scans every partition (slow cold load). The flag is mandatory for exactly this reason, so never omit it. It can be approximate (within ±24h), and you can recover or estimate it when it isn't handed to you; see "When you don't have the timestamp" below.

Where the timestamp comes from, in order of preference:

1. **A dashboard URL**: the `?t=<iso>` param is the record's `recordedAt`; URL-decode it and pass it verbatim. (Sessions have no `t`; use the session start, `from=`, or a linked occurrence's `recordedAt`.)
2. **A list/group you already fetched**: every `exceptions show` occurrence carries `recordedAt`. Capture the id and its `recordedAt` together, then drill in.
3. **A notification**: see below.

Query order when you hold an id: resolve its `recordedAt` first (URL, group, or notification), then call the by-id command with it.

### When you don't have the timestamp

The flag is required, so you must supply *something*, but it can be approximate. The lookup window is **±24h** around what you pass (**±48h** for `traces show`), and if the record isn't in that window the server falls back to an unbounded scan. So a timestamp within a day of the truth stays fast; a wrong guess still returns the right record, just slower. Resolve it in this order:

1. **Recover it from an API.** For an occurrence whose hash you know (e.g. `/issues/<hash>/<occurrenceId>` pasted without `?t=`), run `traceway exceptions show <hash>` and read that occurrence's `recordedAt`. The hash endpoint needs no timestamp. A group's `firstSeen`/`lastSeen` from `exceptions list` bound when its occurrences happened (`lastSeen` ≈ the most recent one).
2. **Estimate from context.** A notification's send time, the issue's `firstSeen`/`lastSeen`, or the URL's `preset`/`from` window all put you inside ±24h. That is good enough for a fast lookup.
3. **Ask the user.** If nothing pins it down (e.g. a bare occurrence/endpoint id with no hash, no time, and no list to recover from), ask roughly when it happened ("around when did this fire? within a day is enough") and pass that. Don't invent a placeholder like "now" when the issue is old; that defeats the pruning and can miss the ±24h window entirely.
