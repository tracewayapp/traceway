## Fast by-id lookups (always pass the timestamp)

The by-id detail commands — `exceptions occurrence`, `endpoints show`, `tasks show`, `ai-traces show`, `sessions show`, `traces show` — **require** the record's timestamp (`--recorded-at`, or `--started-at` for sessions). Telemetry tables are partitioned by day: with the timestamp the lookup is bounded to a small window and ClickHouse prunes to a few partitions; without it the server scans every partition (slow cold load). The flag is mandatory for exactly this reason — never omit it. It can be approximate (within ±24h), and you can recover or estimate it when it isn't handed to you; see "When you don't have the timestamp" below.

Where the timestamp comes from, in order of preference:

1. **A dashboard URL** — the `?t=<iso>` param is the record's `recordedAt`; URL-decode it and pass it verbatim. (Sessions have no `t`; use the session start, `from=`, or a linked occurrence's `recordedAt`.)
2. **A list/group you already fetched** — every `exceptions show` occurrence carries `recordedAt`. Capture the id and its `recordedAt` together, then drill in.
3. **A notification** — see below.

Query order when you hold an id: resolve its `recordedAt` first (URL, group, or notification), then call the by-id command with it.

### When you don't have the timestamp

The flag is required, so you must supply *something* — but it can be approximate. The lookup window is **±24h** around what you pass (**±48h** for `traces show`), and if the record isn't in that window the server falls back to an unbounded scan. So a timestamp within a day of the truth stays fast; a wrong guess still returns the right record, just slower. Resolve it in this order:

1. **Recover it from an API.** For an occurrence whose hash you know (e.g. `/issues/<hash>/<occurrenceId>` pasted without `?t=`), run `traceway exceptions show <hash>` and read that occurrence's `recordedAt` — the hash endpoint needs no timestamp. A group's `firstSeen`/`lastSeen` from `exceptions list` bound when its occurrences happened (`lastSeen` ≈ the most recent one).
2. **Estimate from context.** A notification's send time, the issue's `firstSeen`/`lastSeen`, or the URL's `preset`/`from` window all put you inside ±24h — good enough for a fast lookup.
3. **Ask the user.** If nothing pins it down (e.g. a bare occurrence/endpoint id with no hash, no time, and no list to recover from), ask roughly when it happened — "around when did this fire? within a day is enough" — and pass that. Don't invent a placeholder like "now" when the issue is old; that defeats the pruning and can miss the ±24h window entirely.
