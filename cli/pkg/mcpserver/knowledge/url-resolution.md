## Resolving Dashboard URLs

Users paste dashboard URLs (`https://<instance>/<route>`) as references in any flow. Resolve by route family:

| URL path | Identifies | How to fetch it |
|---|---|---|
| `/issues/<hash>` and `/issues/<hash>/events` | Exception group (hash = 16 hex chars) | `traceway exceptions show <hash>` |
| `/issues/<hash>/<occurrenceId>` (UUID) | One occurrence within the group | `traceway exceptions occurrence <occurrenceId> --recorded-at <t>` where `t` is the URL's `?t=` param. Direct and fast; also returns the occurrence's `sessionId` and session recording. No URL? get `recordedAt` from `traceway exceptions show <hash>` occurrences |
| `/endpoints/<endpoint>` | Endpoint group; the segment is the URL-encoded endpoint name (`GET%20%2Fapi%2Fusers%2F%3Aid` is `GET /api/users/:id`) | Decode it, then `traceway endpoints list --search "<decoded name>"` (the group has no id; `endpoints show` is for one request — next row) |
| `/endpoints/<endpoint>/<endpointId>` | One request (transaction) of that endpoint | `traceway endpoints show <endpointId> --recorded-at <t>` (`t` = the URL's `?t=` param). Returns the request, its span waterfall, and any linked exception/messages |
| `/tasks/<task>` | Background task group | No CLI for the group; for one run use the next row |
| `/tasks/<task>/<taskId>` | Single task run | `traceway tasks show <taskId> --recorded-at <t>` (`t` = the URL's `?t=` param) |
| `/sessions/<sessionId>` | Session (the exceptions that fired during it; replay stays dashboard-only) | `traceway sessions show <sessionId> --started-at <t>`. The URL has no `?t=`; use the session's start, the URL's `from=`, or a linked occurrence's `recordedAt` (it falls inside the window). Occurrences reference sessions via their `sessionId` |
| `/ai-traces/<traceName>` | AI trace group | No CLI for the group; for one trace use the next row |
| `/ai-traces/<traceName>/<traceId>` | Single AI trace | `traceway ai-traces show <traceId> --recorded-at <t>` (`t` = the URL's `?t=` param); returns token/cost stats + the conversation |
| `/logs` | Logs page (its filters are not stored in the URL) | `traceway logs query` with flags taken from the user's description |
| `/issues`, `/endpoints`, `/metrics`, `/` | List and dashboard pages | The matching `list` / `query` command |

**Time window**: most dashboard URLs carry `?preset=<p>` or `?from=<iso>&to=<iso>` (sticky across pages); honor them instead of the default window.

- `preset` values `5m 30m 60m 3h 6h 12h 24h 3d 7d` map directly to `--since`; the CLI has no month unit, so map `1M` to `--since 30d` and `3M` to `--since 90d`.
- `from`/`to` are ISO timestamps; pass via `--from`/`--to`, appending `Z` (or the correct offset) when missing, since the CLI requires RFC3339.
- No time params means the page was on its default; pick `--since` per the ground rules.

`preset`/`from`/`to` set the *window* for list/group views. Detail URLs additionally carry **`?t=<iso>`** — the single record's timestamp, URL-encoded. That `t` value is exactly what the by-id commands need as `--recorded-at` (or `--started-at` for sessions). See "Fast by-id lookups" next.
