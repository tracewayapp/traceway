# traceway-cli

A Go CLI for the [Traceway](https://github.com/tracewayapp/traceway) observability platform — exceptions, logs, endpoints, metrics. Designed to be **first-class for both LLMs (invoking via shell tools) and humans** (with `gh`-style ergonomics).

## Install

Prebuilt binaries are published with every release at
[github.com/tracewayapp/traceway/releases](https://github.com/tracewayapp/traceway/releases)
under the matching `CLI vX.Y.Z` tag — the CLI version tracks the backend release.
Download the archive for your platform (`traceway_<version>_<os>_<arch>.tar.gz`,
or `.zip` on Windows), extract it, and put `traceway` on your `PATH`.

Or build from source — this repo ships a Nix dev shell with Go 1.26, `just`, `gotestsum`, `golangci-lint`, `govulncheck`, and `gh`:

```bash
nix develop
just build         # produces ./bin/traceway
```

Or vanilla Go:

```bash
go build -o bin/traceway ./cmd/traceway
```

Check an installed binary with `traceway version` (or `traceway --version`). Source builds report `dev`.

## Quick start

```bash
# 1. log in (creates ~/.config/traceway/config.json + ~/.local/state/traceway/state.json)
#    Default: browser device login — prints a URL + short code, you approve in the
#    browser, and the CLI receives a token that auto-refreshes.
traceway login --url https://cloud.traceway.com

# 2. pick a project (one-time; future calls use it implicitly)
traceway projects list
traceway projects use <project-id>

# 3. ask questions
traceway exceptions list --since 24h
traceway logs query --since 1h --search "OutOfMemory"
traceway endpoints list --since 1h
traceway metrics query --name http.server.duration --aggregation avg --since 1h
```

## Commands

| Command | Purpose |
|---|---|
| `traceway login` | Authenticate and store a token. Three modes: default browser **device flow** (auto-refreshing); `--password` (email + password, or `--password-stdin`; passing `--username` implies this mode); `--token` / `--token-stdin` (store a personal access token). The device flow needs an interactive terminal — in CI use `--token` or `--password`, or pass `--no-browser` to drive it manually. |
| `traceway logout` | Revoke the session server-side (device logins) and forget the stored credentials for a profile |
| `traceway profiles {list,use}` | Manage multiple Traceway accounts/instances |
| `traceway projects {list,use}` | List or select the active project |
| `traceway exceptions list` | Recent grouped exceptions |
| `traceway exceptions show <hash>` | A single exception group + occurrences |
| `traceway exceptions occurrence <id> --recorded-at <t>` | A single occurrence by id (+ `sessionId` and recording) |
| `traceway exceptions archive <hash>...` | Archive one or more groups (mutating; needs `--yes` non-interactively) |
| `traceway exceptions unarchive <hash>...` | Unarchive (mutating; needs `--yes` non-interactively) |
| `traceway logs query` | Query logs with severity / service / search filters |
| `traceway endpoints list` | Per-endpoint p50/p95/p99 stats |
| `traceway endpoints show <id> --recorded-at <t>` | A single request (transaction) by id: spans + linked errors |
| `traceway tasks show <id> --recorded-at <t>` | A single background task run by id |
| `traceway ai-traces show <id> --recorded-at <t>` | A single AI trace by id + its conversation |
| `traceway sessions show <id> --started-at <t>` | A single session by id + the exceptions that fired in it |
| `traceway traces show <id> --recorded-at <t>` | A distributed trace: every service node sharing the id |
| `traceway metrics query` | Time-series metric queries |
| `traceway mcp` | Serve the MCP server over stdio (see "MCP server" below) |

Run `traceway <command> --help` for full per-command flags.

### By-id detail lookups are time-bounded

The `show` / `occurrence` commands take a UUID plus a **required** timestamp flag
(`--recorded-at`, or `--started-at` for `sessions`). Telemetry tables are
partitioned by day; the timestamp bounds the query to a window around the record
so the lookup prunes partitions instead of scanning all of them. Get the id and
its timestamp together — from a dashboard URL's `?t=` param, a notification's
`Occurred at`, or a list/`exceptions show` row's `recordedAt`. Omitting the flag
exits 2 (`usage_error`); a non-RFC3339 value exits 2 (`invalid_timestamp`).

## Profiles

Multiple Traceway instances or accounts coexist via profiles. Configuration (URL, username) lives in `$XDG_CONFIG_HOME/traceway/config.json` so it can be checked in or managed declaratively (e.g. NixOS); credentials and the active project live in `$XDG_STATE_HOME/traceway/state.json`.

```bash
traceway login --url https://traceway.example.com --profile work
traceway profiles list
traceway profiles use work
traceway --profile personal exceptions list   # one-off override
```

## Output formats

The `--output` flag picks the format. The default is `table` on a TTY and `json` otherwise — i.e. piping always gets machine-readable output.

| Format | Use |
|---|---|
| `table` | Human-friendly columns (default on TTY) |
| `json` | Compact JSON, one record per line. Default when stdout isn't a TTY |
| `yaml` | YAML rendering of the same data |

`--fields a,b,c` projects list responses to just those keys (the `pagination` wrapper passes through unchanged):

```bash
traceway exceptions list --output json --fields exceptionHash,count,lastSeen
```

## Errors

Every error writes a stable JSON envelope to stderr (in `json` / `yaml` modes — prose in `table` mode) and exits with one of:

| Exit | Meaning |
|---|---|
| 0 | Success |
| 1 | Generic / API error |
| 2 | Usage error (bad flags, missing confirmation, invalid time range) |
| 3 | Connection failure |
| 4 | Auth failure (`not_authenticated`, `token_expired`, `forbidden`) |
| 5 | Not found |
| 6 | Rate limited |
| 7 | Server (5xx) |

Envelope shape:

```json
{"error":"token_expired","message":"session expired or invalid","hint":"traceway login","exit_code":4}
```

The `error` field is a stable snake_case identifier. LLMs/scripts can branch on it.

## Mutations + confirmation

`exceptions archive` and `exceptions unarchive` require explicit consent because they change server state:

- Pass `--yes` to skip the prompt.
- Or set `TRACEWAY_ASSUME_YES=1` in the environment.
- Or run interactively and answer the `Continue? [y/N]` prompt.

Calling a mutating command from a non-TTY context (script, LLM tool call) without one of the opt-ins fails immediately with `usage_error` (exit 2) — no hung prompts.

## MCP server

`traceway mcp` serves the whole query/debug surface as an MCP server on stdio,
for MCP clients like Claude Code, Claude Desktop, and Cursor:

```bash
claude mcp add traceway -- traceway mcp
```

It reuses the CLI session (credentials, token refresh, current project); run
`traceway login` and `traceway projects use` first. Without a login it exits
immediately with instructions (a stdio server has no TTY to prompt on). For
headless use set `TRACEWAY_URL` + `TRACEWAY_TOKEN` (a `twp_` personal access
token), plus optional `TRACEWAY_PROJECT`, and no CLI session is needed.

Each CLI query command maps 1:1 to a tool (`exceptions list` becomes
`list_exceptions`, and so on); only `archive_exceptions`/`unarchive_exceptions`
mutate, and they are annotated accordingly. The server also exposes the
Traceway debugging playbooks as MCP resources (`traceway://knowledge/*`) and
prompts (`debug_issue`, `investigate_performance`, `whats_broken`,
`resolve_notification`).

Every tool declares an output schema, so results come back as validated
`structuredContent`. The same server is also mounted by the backend itself as
a remote MCP server at `https://<instance>/mcp` (streamable HTTP, OAuth
authorization-code + PKCE with dynamic client registration, or a `twp_` PAT
as the bearer token) - `claude mcp add --transport http traceway
https://<instance>/mcp` needs no local install at all.

The implementation lives in `pkg/mcpserver` (importable, transport-agnostic;
the stdio command is a thin wrapper). The playbook markdown in
`pkg/mcpserver/knowledge/` is the canonical source for the published
`skills/traceway` Claude Code skill: edit the chunks, then run
`just gen-skills` (drift is CI-checked).

## Smoke testing

Unit/CLI tests run by default and never touch a network:

```bash
just test
```

End-to-end tests against a real Traceway instance are gated behind a build tag and read connection info from env vars:

```bash
export TRACEWAY_SMOKE_URL=https://traceway.example.com
export TRACEWAY_SMOKE_USERNAME=you@example.com
export TRACEWAY_SMOKE_PASSWORD=...
export TRACEWAY_SMOKE_PROJECT_ID=...
just smoke-test
```

If any of those vars is missing, the smoke tests skip cleanly rather than fail.

## Contributing

```bash
just check   # lint + test + vulncheck
```

The library at `pkg/client` deliberately has zero CLI dependencies — it's importable directly by other Go programs. The MCP server (`pkg/mcpserver`) is its first such consumer and must stay out of `pkg/client`'s dependency graph.
