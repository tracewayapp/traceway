# Monitors, incidents, status pages and post-mortems

Monitors answer the question telemetry cannot: is the service reachable from the outside? Traces and logs only exist while requests arrive. A monitor keeps probing on a schedule, so the team learns the site is down before a user tells them.

Everything here lives under **Monitors** in the project sidebar (hidden for browser and mobile projects), in three tabs kept in `?tab=`:

| Tab | Who sees it | What it holds |
|---|---|---|
| Monitors | Every member | The project's checks. "New Monitor" needs write access on the project. |
| Status Pages | Organization owners and admins | Public uptime pages for the organization |
| Post-Mortems | Every member | The project's internal incident write-ups |

The sidebar badge counts the project's monitors that are down right now. The organization's **Monitors** page lists every monitor across all projects with the incident history (see `organization.md`).

## Check types

| Type | What it does | Configuration |
|---|---|---|
| **HTTP** | Requests a URL, like Postman would | URL (http or https), method (GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS), headers, auth (none, basic, bearer, or a custom header), a body for POST, PUT, PATCH and DELETE (Content-Type defaults to `application/json`), follow redirects (on by default), skip TLS verification (off by default) |
| **TCP** | Opens a connection to a host and port | Host, port, optional TLS, an optional payload to send, and an optional "expect response containing" string (up to 64 KB is read) |
| **Browser** | Runs a real `@playwright/test` spec in headless Chromium | The script (up to 100 KB, edited on the check) and optional environment variables. Only runs where the operator turned browser checks on. Never available on Traceway Cloud. |

HTTP expected results, all optional:

- **Status codes**: a list such as `200, 204`. Left empty, anything below 400 passes.
- **Body contains** a substring, or **body matches** a regex. Only the first 1 MB of the body is checked.
- **Max response time** in milliseconds.
- **Min TLS days remaining**: fails the run when the certificate expires sooner. This is how a team catches a certificate about to expire. TCP checks over TLS have the same option.

When several assertions fail, the run's error lists all of them separated by `; `.

Shared settings on every check:

| Setting | Range | Default |
|---|---|---|
| Interval | 30 seconds to 24 hours (the UI offers 30s, 1m, 2m, 5m, 10m, 15m, 30m, 1h, 6h, 24h) | 1 minute |
| Timeout | 1 to 120 seconds | 30 seconds |
| Failures before down | 1 to 10 consecutive failures | 2 |

The type cannot change after creation. Saving an edit schedules a run right away. On Traceway Cloud, checks can only target public addresses. Self-hosted instances allow private and LAN targets unless the operator sets `SYNTHETICS_ALLOW_PRIVATE_TARGETS=false`.

## States and runs

| Status | Meaning |
|---|---|
| **Pending** | No result yet (stored as `unknown`) |
| **Up** | The last run passed |
| **Down** | The last N runs failed, where N is the failure threshold |
| **Paused** | The check is disabled. Pausing drops queued runs. It does not resolve an open incident. |

- One success sets the check Up immediately and resets the failure counter.
- A failure only turns the check Down once consecutive failures reach the threshold. This is flap damping: one flaky probe does not page anyone at 3am.
- **Missed** runs: a queued run that is not picked up within its window (the interval, capped at 5 minutes) is recorded as `missed` instead of running late. Missed runs never change the state and are left out of uptime and latency. If Traceway itself was down, the charts show a gap, never a made-up 100%.
- HTTP and TCP probes run on the Traceway server. Browser runs run on the server or on a remote runner, shown as "via <name>" in the run history. There are no probe regions.
- A failed browser run stores a screenshot and the Playwright output. Both open from the run history.

## The monitor page

Covers the last 30 days. It has no time picker.

- **Tiles**: uptime, average latency, interval, and recent incidents (red while one is ongoing).
- **Availability strip**: green all up, red all down, amber mixed, grey no data. The tooltip shows up, total and missed counts.
- **Latency chart**: average latency per bucket.
- **Run history**: filter by All, Up, Down or Missed. Each row has the time, the executor, the result, the latency, the status code (not for browser checks) and the full error.
- **Incidents**: each with its time range, error, Resolved or Ongoing, and a **Write post-mortem** link (or **Post-mortem** once one exists).
- **Actions** (write access): Run now, Pause or Resume, Edit, Delete. Deleting removes the run history and incidents and resolves the on-call pages the monitor opened.

The monitor list shows name, type, status, 30-day uptime (green at 99.9% or more, amber at 99% or more, red below), average latency and last run. It can be searched and filtered by type.

**Run now** queues an immediate run. It is unavailable while the check is paused, and refused while a run is already queued ("A run for this check is already queued.") or for a browser check when the server has browser checks turned off. Such browser checks also skip their scheduled runs. Use it to confirm a fix without waiting for the next interval. The result appears in the run history within seconds.

## Alerting on monitors

A **Monitor Down** alert rule covers every monitor in the project. It fires when a monitor goes down and sends a recovery notice (always Info, green in Slack) when it comes back. Pending to Up sends nothing. Pointed at an escalation channel, it opens an on-call page per monitor, and recovery resolves that page on its own. See `alerts.md` and `on-call.md`.

## Incidents

An incident is a span of time when something was down, with an optional public story: a title and a timeline of updates.

**Automatic incidents** come from monitors:

- Going down opens one, holding the probe's error, with an automatic **Investigating** update. Recovery resolves it with a **Resolved** update.
- The default title is "<monitor name> is down". Anyone with admin rights can rename it to something users understand, such as "Checkout is failing for some card payments". Clearing the title brings the default back.
- The monitor owns the lifecycle. An automatic incident cannot be resolved by hand, re-timed or deleted.
- The probe's error message stays internal. It never reaches a public status page.

**Manual incidents** record outages the monitors missed, or maintenance windows:

- Created with **Record Incident** on a status page's incident history (admins only). A title is required, plus an optional first update, a start time (backdating is fine) and an optional resolved time.
- Resolve one by posting a **Resolved** update or setting the resolved time. Clearing the resolved time reopens it. Manual incidents can be deleted, and a linked post-mortem survives.
- They never change a monitor's uptime numbers.

**Timeline updates** carry a status, **Investigating**, **Identified**, **Monitoring**, **Update** or **Resolved**, and a message of up to 2000 characters. They are public content, so posting, renaming and deleting need an organization owner or admin. Every member can read the incident history.

Where incidents are managed:

- **Status Pages tab > Incidents** on a page: the full paginated history of that page's monitors plus its manual incidents. Expand a row to rename it, post updates, open or write the post-mortem, and resolve or delete manual incidents.
- **The monitor page**: that monitor's incidents.
- **Organization > Monitors**: every incident in the organization that was open at any point in the selected range.

## Status pages

A status page publishes the uptime of selected monitors at `/status/<slug>`, with no login. Only organization owners and admins create and edit them.

| Field | Rules |
|---|---|
| Title | Required, up to 200 characters |
| Slug | 3 to 60 lowercase letters, digits and hyphens, unique |
| Description | Optional, up to 500 characters |
| Logo | PNG or JPEG up to 1 MB, uploaded after saving. SVG is refused. |
| Custom domain | Self-hosted only (see below). Optional hostname such as `status.acme.com`, unique, never the Traceway host itself. On Traceway Cloud leave it empty and frame the page instead. |
| Publicly accessible | On by default. A private page answers 404, the same as an unknown slug. |
| Monitors | 1 to 50. The dashboard picker offers the current project's monitors. |

What visitors see:

- A banner: "All systems operational", or a red "Some systems are experiencing issues" when a monitor is down.
- Per monitor: Operational, Major outage, or No recent data (paused monitors), its 90-day uptime, and 90 daily bars in UTC: **Operational** (green, 99.5% or more), **Partial outage** (amber), **Major outage** (red, no successful run) and **No data** (grey). Narrow screens show 60 or 30 days.
- **Past Incidents**: the last 90 days of incidents (up to 30), grouped by day, each with its title and timeline updates.
- No latency numbers and no internal error messages. Always a light theme.

The data is cached for 30 seconds and the page refreshes itself every minute. Any edit to the page or its incidents clears the cache. Status pages can be framed by any site. To serve one on a company domain, a self-hosted instance points a CNAME at Traceway behind a TLS proxy that passes the Host header through and sets **Custom domain**. On Traceway Cloud, or when touching the proxy is not an option, host a small HTML file that frames `/status/<slug>` full window.

## Post-mortems

Internal write-ups of an incident: what happened, the impact, the timeline, the root cause and the follow-ups. They belong to one project, so opening one under a different project answers not found. They never appear on a status page.

- **Creating**: a dialog asks only for a title, creates the document and opens the editor. It can be started from the Post-Mortems tab, from an incident on the monitor page, or from an incident on a status page's history (which pre-links that incident).
- **Editor**: rich text with a **Rich text / Markdown** toggle. Type `/` for headings, lists, tables and code blocks. Content is Markdown, up to 200 KB.
- **Tags**: lowercased, up to 20 per document, 50 characters each.
- **Incident link**: optional, one post-mortem per incident. The picker offers the project's own monitor incidents plus the organization's manual status page incidents that have no post-mortem yet. Linked incidents show a link to the document everywhere they appear. Deleting the incident or monitor only severs the link.
- **Activity**: every create and edit with who, when and which parts changed (title, content, tags, incident). Saving without a change records nothing.
- **Finding them**: search matches title, content and tags, and clicking tags filters by all of them together. When a similar incident hits, last time's fix is one search away.
- **Permissions**: every member with project access reads them. Creating, editing and deleting need write access on the project.

## Reading and changing it from an agent

| Task | CLI | MCP tool |
|---|---|---|
| List monitors with their current status | `traceway monitors list` | `list_checks` |
| One monitor with its incidents | `traceway monitors show <id>` | `get_check` |
| List post-mortems | `traceway post-mortems list [--search S] [--tag T]` | `list_post_mortems` |
| Read a post-mortem | `traceway post-mortems show <id>` | `get_post_mortem` |
| Run a monitor now (write) | `traceway-write monitors run <id>` | `run_check` |
| Create or edit a post-mortem (write) | `traceway-write post-mortems create\|update` | `create_post_mortem` / `update_post_mortem` |

Creating or editing monitors, status pages, incidents and public timeline updates stays in the dashboard. Status pages and updates are public, so a person should decide what the world reads.
