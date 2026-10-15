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

{{include "operations.md"}}

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

{{include "url-resolution.md"}}

{{include "timestamps.md"}}
