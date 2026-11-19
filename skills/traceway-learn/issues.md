<!-- GENERATED FILE: copied from cli/pkg/mcpserver/knowledge/learn/issues.md by cli/tools/skillgen. Edit there and run just gen-skills in cli/. -->
# Issues and exceptions

An **issue** is a group of exceptions (or captured messages) that share one fingerprint. The issues list ranks root causes, not repetitions: a bug in a hot path that throws ten thousand times is one row with a count of ten thousand.

## What creates an issue

- An unhandled exception or panic during a traced request or task.
- An error recorded on a span as an OpenTelemetry `exception` event.
- An exception or a message captured explicitly through a Traceway SDK (browser, mobile).

There are two kinds:

- **Errors** have a stack trace.
- **Messages** are captured strings with no exception. They group by their exact text, so variable data belongs in attributes. `"payment failed for order 123"` makes a new issue per order.

## Fingerprinting

Every error gets a **fingerprint**: a 16-hex-character hash of its normalized stack trace. The hash is the issue's identity. It is in the URL (`/issues/<hash>`), and archiving, regressions and alerts all key on it.

Before hashing, values that change between occurrences of the same bug are removed: the error message (only the type is kept), ids, UUIDs, hex addresses, IPs, emails, long numbers, goroutine numbers, absolute paths (only `file:line` stays), URL origins, module versions, JVM line numbers, columns (except on line 1, where minified bundles need them), and resolved JavaScript function names.

So the same failure keeps one issue across servers, environments, versions and users. A new issue appears when the failure changes: a different error type or a different code path. Line numbers count for most languages, so moving code inside a file can split an issue after a deploy (JVM frames are the exception). Uploading source maps or debug symbols for the first time rewrites the frames, so later occurrences form a new issue.

## The issues page (`/issues`)

- **Columns**: Issue (error type and first line), Trend (hourly sparkline), Events (count), Last Seen. Sort by Events or Last Seen (the default). Sort by last seen to see what is happening now.
- **Search**: a case-insensitive substring of the stack trace, scoped to **All**, **Issues** (errors only) or **Messages**.
- **Time range**: defaults to 24 hours. An issue is listed when it occurred in the range.
- **Archived issues** are hidden. The page has no toggle to show them.
- **Bulk archive**: select rows (up to 100) and **Archive**. When the selected issues have open on-call pages, the dialog offers to resolve them too (checked by default).

## The issue page (`/issues/<hash>`)

- The stack trace with **Formatted** and **Raw** views. Formatted folds library and system frames (JavaScript, Flutter, iOS, Java, Go). A **Message** badge marks captured messages.
- First seen, last seen, total occurrences, and the **Archive** button.
- The on-call team that owns the project and who is on call now.
- **Last Event**: when, server, version, trace id, attributes, and the **related endpoint, task or AI trace** it happened in, with its status, duration and a link. Browser issues show the replay clip from just before the error, or a **View full session** button.
- A **Distributed Trace** card when other endpoints, tasks, AI traces or exceptions share the trace id.
- On browser issues, **Session Context**: the console logs and user actions around the error, synced to the replay.
- The first 10 events, then **View all N events** (`/issues/<hash>/events`, 50 per page). Each event opens its own occurrence page.

**Occurrence** pages (`/issues/<hash>/<exceptionId>`) show one occurrence with the same event card. When debugging, fix the **last** occurrence, not the group: it is the one with the current code and data.

## Lifecycle

1. **Active**: it occurred.
2. **Archived**: someone marked it fixed or accepted. It is hidden from the list.
3. **Back**: an archived issue that occurs again after its archive time shows in the list again by itself. It stays archived until someone unarchives it, so it hides again only if it is archived again. On that first re-occurrence the **New Issue** rule fires once. The **Error Regression** rule fires on it and on every later occurrence while the issue is still archived, limited by its cooldown.

Archive an issue once it is fixed, so that a regression alerts. Unarchiving has no button in the dashboard. Agents with write access can do it, and the issue also comes back by itself if it occurs again.

## Attributes

An occurrence carries the key-value attributes in scope when it happened. Under a trace it also carries the span's attributes. Request URL, query, body and headers can be attached when the SDK is configured to record them.

## Reading and changing it from an agent

| Task | CLI | MCP tool |
|---|---|---|
| List issues | `traceway exceptions list --since 24h [--search S] [--include-archived]` | `list_exceptions` |
| One issue with occurrences | `traceway exceptions show <hash>` | `get_exception` |
| One occurrence | `traceway exceptions occurrence <id> --recorded-at <t>` | `get_exception_occurrence` |
| Archive or unarchive (write) | `traceway-write exceptions archive\|unarchive <hash>...` | `archive_exceptions` / `unarchive_exceptions` |

Search is always a case-insensitive substring of the stack trace. The list comes back newest activity first (`lastSeen`); `--order-by firstSeen` or `count` changes that. `--include-archived` is the only way to list archived issues. The `traceway` skill's debug flow walks an issue from hash to root cause.
