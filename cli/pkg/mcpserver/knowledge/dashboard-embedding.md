# Embed a dashboard in another application

Use this guide when the user wants a Traceway dashboard shown inside their own product (a customer admin panel, an internal tool, a wall display) or wants a link to one that opens without a Traceway account. The work is a small integration in **their** backend and frontend. Traceway already serves the page.

## How it works

A shared dashboard is the read-only page `/embed/dashboard` on the Traceway instance. It needs no Traceway account and any site may frame it. A signed link decides what it shows: one dashboard in one project, a time range, optional tag filters and an expiry.

1. The viewer opens a page of the host application.
2. The host backend checks that the viewer may see the data, then asks Traceway for a link with `POST /api/dashboards/{id}/embed-token`, authenticated with a write token it keeps server-side.
3. Traceway answers with `embedPath`. The host backend resolves it against the Traceway origin and returns the URL to its own frontend.
4. The frontend sets it as the `src` of an iframe. The page reads the dashboard from Traceway with the token in the URL fragment.
5. Shortly before the link expires the frontend asks its backend for a new one and assigns it to the same iframe.

Traceway stores nothing per link. It verifies the signature and the dashboard's assignment on every read, so a link cannot be revoked on its own. That is why the default lifetime is five minutes and the host renews.

## Check before writing code

- **The dashboard and project.** `traceway dashboards list --output json` gives the numeric dashboard id; `traceway projects list` gives the project UUID. The dashboard must be applied to that project.
- **A tag that separates viewers.** To show each customer their own numbers, every metric on the dashboard must carry a tag that names the customer, such as `account_id`. Confirm it with `traceway metrics discover --name <metric> --key account_id --since 24h --output json`. A widget whose metric lacks the tag shows nothing under the filter. Without any such tag an embed shows every customer's data: say so and do not embed it for customers.
- **The credential.** A **Read and write** personal access token (`tww_`), created by the user in **Account > Personal Access Tokens**. It expires within 90 days, so tell the user the date it must be rotated by. Its owner needs write access to the project, and an owner or admin must switch on **Settings > Agent Access > Allow agent writes**. A project ingest token or a read-only token cannot issue links.
- **Where the credential lives.** In the host backend's secret store, read from an environment variable such as `TRACEWAY_EMBED_TOKEN`. Never in frontend code, never committed, never printed. Ask the user to put it there themselves; do not ask them to paste it into chat.
- **Two addresses.** The URL the host backend calls Traceway at, and the origin the viewer's browser reaches Traceway at. They differ when the backend uses an internal address. The iframe needs the second.

## The request

```http
POST {traceway}/api/dashboards/{dashboardId}/embed-token?projectId={projectUuid}&timeframe=7d&theme=light
Authorization: Bearer tww_...
Content-Type: application/json

{"tagFilters": {"account_id": "acme"}}
```

| Input | Meaning |
|---|---|
| `timeframe` (query) | Relative window: a positive whole number followed by `m`, `h` or `d`, up to 731 days. Omitted: the last 42 hours. |
| `theme` (query) | `light` or `dark` (default). Copied into the returned URL. |
| `from`, `to` (body) | A fixed range, RFC 3339, both or neither. Excludes `timeframe`. |
| `tagFilters` (body) | Tag values every widget is limited to. At most 20, no empty tag or value. They win over a filter saved on a widget for the same tag. |
| `expiresInDays` (body) | Lifetime in whole days, 1 to 90. |
| `expiresAt` (body) | Alternatively an RFC 3339 timestamp at most 90 days ahead. Send one expiry field or none. |

The body is optional, at most 2 KB, and an unknown field is refused. With no expiry the link lasts five minutes and the window is fixed when the link is issued. With an expiry a relative window is resolved each time the link is opened.

The **201** answer:

| Field | Meaning |
|---|---|
| `embedPath` | `/embed/dashboard?dashboard=<uuid>#token=<jwt>`. Resolve it against the browser-facing Traceway origin. |
| `expiresInSeconds` | How long the link still works. Schedule the renewal from it; it does not depend on the two clocks agreeing. |
| `expiresAt` | The same moment in UTC. |
| `token`, `dashboardUuid` | Already part of `embedPath`. |

## The host backend

Any language works: it is one HTTP POST. This is the shape for Node; port it to the framework the repository already uses and put it behind the application's existing authentication.

```js
// Inside the application's authenticated route for the viewer's KPI panel.
if (!viewer.accountId) throw new Error('An account is required');
const url = new URL(`/api/dashboards/${process.env.TRACEWAY_DASHBOARD_ID}/embed-token`, process.env.TRACEWAY_URL);
url.searchParams.set('projectId', process.env.TRACEWAY_PROJECT_ID);
url.searchParams.set('timeframe', '7d');
const response = await fetch(url, {
  method: 'POST',
  headers: {
    Authorization: `Bearer ${process.env.TRACEWAY_EMBED_TOKEN}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({ tagFilters: { account_id: String(viewer.accountId) } })
});
if (!response.ok) throw new Error(`Traceway returned ${response.status}`);
const { embedPath, expiresInSeconds } = await response.json();
return Response.json(
  { src: new URL(embedPath, process.env.TRACEWAY_PUBLIC_URL).href, expiresInSeconds },
  { headers: { 'Cache-Control': 'no-store' } }
);
```

Rules that keep it safe:

- The dashboard id, the project id and the filter value come from server configuration and the viewer's session. Never take them from a query parameter or the request body: that would let one customer ask for another's account.
- Refuse the request when the viewer has no account. Traceway also refuses an empty filter value with 422.
- The link is a credential for as long as it lives. Answer with `Cache-Control: no-store` and keep `embedPath`, `src` and `token` out of logs and error reports.
- Keep the default five-minute lifetime for an embed and renew it. A longer expiry is for a link a person pastes somewhere, since it cannot be taken back.

## The host frontend

```html
<iframe id="account-kpis" title="Account KPIs" style="width:100%;height:640px;border:0"></iframe>
<script>
  async function showAccountKpis() {
    const response = await fetch('/account/kpis/embed');
    if (!response.ok) throw new Error('Could not load account KPIs');
    const { src, expiresInSeconds } = await response.json();
    document.querySelector('#account-kpis').src = src;
    setTimeout(showAccountKpis, (expiresInSeconds - 30) * 1000);
  }
  showAccountKpis();
</script>
```

Assigning a new `src` for the same dashboard replaces what is on screen without a blank frame. If the host page sends a Content-Security-Policy, add the Traceway origin to `frame-src`. To switch theme without a new link, change `theme` in the query before the `#token=` fragment.

## A one-off link from the CLI or MCP

For a link to hand to a person, or to check a dashboard and filter before writing the integration:

```bash
traceway-write dashboards embed 42 --timeframe 7d --expires-in-days 3 --tag account_id=acme --yes --output json
traceway-write dashboards embed 42 --from 2026-11-01T00:00:00Z --to 2026-11-08T00:00:00Z --theme light --yes --output json
```

The MCP equivalent is `create_dashboard_embed` on the write server (`id`, optional `timeframe`, `from`, `to`, `tag_filters`, `expires_in_days`, `expires_at`, `theme`, `project_id`). Both add `url`, the link resolved against the instance. The output holds the signed token: give it to the user who asked and do not repeat it elsewhere. The command is a convenience for people and agents; an application calls the HTTP endpoint itself.

## Verify

1. Call the host endpoint as two viewers of different accounts and confirm each gets a link.
2. Open each link in a browser. The header shows the dashboard name and the time range, and the widgets show that account's values only. Compare one number with `traceway metrics query --name <metric> --tag account_id=<account> --since 7d`.
3. Confirm the browser cannot choose the account: a request to the host endpoint with another account in a parameter still returns the viewer's own.
4. Leave the page open past one renewal and confirm the frame keeps its content.

| Symptom | Cause |
|---|---|
| Issuance **403** | A read-only token, **Allow agent writes** switched off, or the token's owner cannot write to the project. |
| Issuance **404** | The dashboard is not applied to the project in `projectId`. |
| Issuance **422** | The message names it: a malformed or repeated `timeframe`, `timeframe` together with timestamps, half or a reversed range, a window over 731 days, an expiry that is past or more than 90 days ahead, or an empty tag filter. |
| The frame says the link is invalid or has expired | It was not renewed in time, or the instance's signing key changed. Request a new link. |
| The frame says the dashboard is no longer shared | The dashboard was removed from the project or deleted. |
| Widgets say they could not load | More than 600 widget reads a minute from one client address (**429**). Opens are limited to 60 a minute. Viewers behind one proxy share the budget unless the instance trusts that proxy (`TRUSTED_PROXIES`). |
| `/embed/dashboard` answers 404, or the frame stays empty | The instance predates shared dashboards, or the `src` is not the `/embed/dashboard` URL. Every other Traceway page refuses to load in a frame. |

## What the viewer can and cannot see

The page receives the dashboard's name, each widget's title and display settings, and the query results. The queries run on the server: filters saved on a widget never reach the browser. A metric's name appears where a chart uses it as a series label; set a `label` on the source to show that instead. The view is read-only, with no time picker and no links into Traceway.
