# Sessions, AI traces and profiles

## Sessions (`/sessions`)

Only in the sidebar of browser and mobile projects. A session is the recording of one user's visit, usually the life of one open tab: what they clicked, typed, scrolled and saw, with the console logs and network calls alongside.

- The browser SDK records sessions when `recordAllSessions: true` is set. Without it, only a short clip from just before each exception is kept, shown on the issue page.
- A session ends after 15 minutes of inactivity, at 60 minutes in total, or when the page unloads. The server does not rely on the closing message arriving: it ends the session at the last activity it saw.
- **Attributes**: the SDK stamps `url`, `path`, `referrer`, `userAgent`, `language`, `platform`, `viewport`, `screen` and `timezone`. The server adds `client.ip`. The app adds its own with `setAttributes` (for example `userId`, `email`, `tenant`).
- **The list**: search a session id, IP or attribute value. Attribute filters use equals (case sensitive), not equals, contains or not contains (case insensitive), and all filters must match. Columns: session, attributes (click one to filter), started, duration, version. Filters are kept in the URL.
- **A session**: started, duration, app version, attributes, the replay player (rrweb for web, video for Flutter) with speed and fullscreen, **Logs** and **Actions** tabs synced to the replay, and **Exceptions in this session**, each linking to its issue.
- From an issue, **View full session** opens the session the exception happened in.

Replay itself is dashboard only. Read: `traceway sessions show <id> --started-at <t>`, MCP `get_session`, which return the session and its exceptions.

## AI traces (`/ai-traces`)

Hidden for browser and mobile projects. One record per LLM call, from any OpenTelemetry span with `gen_ai.*` attributes, whether it is a root span or a child of a request or job (a span that is already an endpoint or task stays one, so model calls belong on their own child spans).

Each call records: model, provider, duration, input, output, cached, reasoning and total tokens, input, output and total cost, finish reason, operation, user id, tool calls, server, version, attributes, its spans and the full conversation (prompt and completion, rendered as a chat).

Three tabs:

| Tab | What it shows |
|---|---|
| **Traces** | Calls grouped by trace name (the agent or workflow): calls, typical and slow duration, total tokens, total cost, last seen. Sorted by cost. A trace name opens tiles (average, median and P95 duration, total cost, total tokens) and its calls (model, tokens in and out, cost, provider), and each call opens its detail. |
| **Conversations** | Calls grouped into conversations: user, turns, tools, models, tokens, cost, last seen. Conversations above the 95th percentile of cost or turns are highlighted. Filter by flagged only, model or tool, and search conversation ids, users, models, tools and flagged terms. A **Flagged terms** button edits the project's terms in place. A conversation opens a chat timeline with tool calls inline, per-turn cost, and a Raw JSON view. |
| **Users** | Searchable. Per user: conversations, calls, median turns, average cost per conversation, total cost, flagged conversations, last seen |

A call's conversation id is `gen_ai.conversation.id`, else `session.id`, else the trace id, so one agent run inside a request still groups its calls.

**Flagged content**: at ingest, prompts and completions are matched (whole word, case insensitive) against the language packs and custom terms chosen in the project settings **AI** tab. Packs: English (default), German, Spanish, French, Italian, Portuguese, Serbian. Custom terms: up to 200, one per line. Changes apply to calls ingested afterwards. Flagged calls carry the matched terms, and the **AI Flagged Content** alert rule can notify on them.

Cost alerts: **AI Trace Cost** for one expensive call, **AI Conversation Cost** for a conversation that gets expensive over 24 hours (`alerts.md`).

Read: `traceway ai-traces show <id> --recorded-at <t>`, MCP `get_ai_trace` (one call with its conversation). Lists, conversations and users are dashboard only.

## Profiles (`/profiles`)

Hidden for browser and mobile projects. Continuous profiles per service. Use them after a waterfall says the time is in the application's own code and the question is which function.

- **The list**: service, profile type, profiles, samples, total, trend, last seen.
- **A service**: profile types CPU, In-Use, Alloc, In-Use Obj, Alloc Obj, Goroutines, Mutex, Block, Wall and Lock (depending on the language). Filter by labels, group by app version or server, and show CPU as cores or per second.
- A trend chart (drag to pick a range), a **flame graph** with search and a collapse-recursion option, a function detail (sandwich) view, and **top functions** with flat and cumulative time.
- **Compare** against a baseline version or server, as a differential flame graph (hotter or cooler) or side by side. This is how to see what a release made slower.
- **Download .pprof** for local tools.

Profiles are dashboard only. There is no CLI or MCP tool.
