## Ground Rules (All Flows)

- **Reads are safe**: any `list` / `show` / `query` subcommand may run freely; they never mutate server state.
- **Writes require explicit user instruction**: `exceptions archive` / `unarchive` are the only mutating data commands; only run them when the user asks by name, with `--yes` in non-interactive contexts. "Look at this error" means read it, not archive it.
- **`agent report` is for CI, not for you**: it records a finished auto-fix run as an attempt on the issue page. When you are the agent investigating an issue, the harness reports your outcome; never call `agent report`, `agent attempts` or `agent show` about your own run.
- **Output**: piped output defaults to JSON (table on a TTY). Prefer JSON + `jq`, and `--fields a,b,c` to trim responses. Keep `--page-size` at 10 to 20 for triage.
- **Time windows**: always bound queries, default `--since 1h` for "now" questions, `--since 24h` otherwise. `--since` accepts `s`, `m`, `h`, lowercase `Nd` (no `1w`, no `7d2h`). Absolute windows via `--from` / `--to` (RFC3339).
- **Sources**: a profile always queries its own Traceway instance (the implicit `traceway` source). `traceway sources add` binds further sources by name and domain; a query then merges every source that answers its domain, tags each record with `source`, adds a `SOURCE` column to tables, and reports a source that failed as a stderr warning while the others still answer. `--source <name>` narrows a command to one source; `traceway sources list` shows what each answers.
- **Exit codes**: 0 ok, 1 generic/API, 2 usage, 3 connection, 4 auth, 5 not found, 6 rate limited, 7 server 5xx. Errors emit `{"error":"<stable_id>","message":"...","hint":"...","exit_code":N}` on stderr; branch on the `error` field.
- On exit code 4 (auth), do not run `traceway login` yourself; switch to the Login flow and let the user enter credentials.
- **Never guess a fix**: if the telemetry does not explain the failure, do not change behaviour on a hunch. Add the instrumentation that would explain it next time, and say what is still unknown. See "When the evidence is not enough" in the Debug flow.
