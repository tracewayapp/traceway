<!-- GENERATED FILE: copied from cli/pkg/mcpserver/knowledge/learn/organization.md by cli/tools/skillgen. Edit there and run just gen-skills in cli/. -->
# Organization, projects, access and settings

## Organizations and projects

- An **organization** is the tenant. It owns members and roles, projects, dashboards, teams, schedules, escalation policies and status pages. Nothing is shared across organizations. A user can belong to several, with one role in each.
- A **project** is one application boundary and belongs to exactly one organization. Telemetry is isolated per project, and almost every page is scoped to the current project. Switch projects from the switcher in the top-left header, where projects are grouped under their organization.
- **Frameworks**: a backend project is always **OpenTelemetry**, whatever the language. Browser projects are React, Svelte, Vue.js or jQuery, and mobile projects are Flutter, React Native, Android or iOS. Browser and mobile projects get a smaller sidebar: Issues, Sessions, Alerts, On-Call and Connection, with no endpoints, tasks, spans, logs, profiles, dashboards, AI traces or monitors. Their home page opens Issues.
- **Recommended split**: one backend project for all server-side signals (APIs, workers, schedulers, AI calls, logs, metrics, the host agent), with a distinct `service.name` per process. One project per independently deployed browser app and per independently released mobile app. Prefer more projects over more organizations: the organization views, dashboards, on-call and status pages are all organization-wide.

## Roles

| Role | Access |
|---|---|
| `owner` | Everything, including managing the organization |
| `admin` | Everything in every project, plus members, settings, teams, schedules, policies and status pages |
| `user` | Read and write in the organization's projects |
| `readonly` | Read only. Can still acknowledge and resolve on-call pages and create schedule overrides. |

Admins can override the role per project for `user` and `readonly` members (Settings > Team Members, expand a member). Owners and admins always have full access. The **effective role** on a project is the owner or admin role, else the override, else the organization role. A read-only effective role hides the project token.

A token or agent login never has more access than the person who created it.

## Home page (`/`)

The project's landing page, for backend projects. It answers "is anything wrong right now?":

- **Starred** widgets from dashboards, always over the last 24 hours. They can be reordered, resized and unstarred.
- **Endpoints**: the worst endpoints by impact over 24 hours (Good ones hidden), with Calls, Typical (P50), Slow (P95) and Impact.
- **Issues**: the 10 latest issues in 24 hours, messages excluded.
- A project with no data yet shows **Connect Your Application** with setup steps and **Check Again**. An account with no projects shows **Create Your First Project** with **Set Up Projects**, which opens the guided setup at `/setup`.

From an agent, the same question is the `whats_broken` recipe: recent exceptions, error logs and the worst endpoints by impact.

## Organization views (`/organization`)

For organizations with more than one project: open the organization itself in the switcher. Login lands here when the first organization has several projects. A single-project organization has no organization pages. Every member can open them, read-only included, and they only show projects the person can already see. If one project cannot be read, the rest still render with a notice.

| Page | What it shows |
|---|---|
| **Servers** | Every server reporting metrics in the last 30 minutes, one row per `server_name` per project: CPU, memory, disk, network, last signal, and host, OS, cloud and Kubernetes details. Health: **Critical** (CPU or memory 95% or more, disk 90% or more), **Warning** (CPU 80%, memory 85%, disk 80%), **Stale** (nothing for 3 minutes), **Healthy**. Filter by health and project, group by project or Kubernetes cluster. A row opens that server's dashboard. Needs host metrics from the OTel Agent or a collector. |
| **Issues** | Issues from every project in the range (24 hours by default), with the project, trend, events and last seen |
| **Monitors** | Every monitor with status, 30-day uptime and latency, and the incident history across monitors and status pages (3 months by default) |
| **On-Call** | Pages from every project (1 month by default), filtered by status and searchable, refreshed every minute |
| **Projects** | Every project with its framework and the person's effective access |

Servers and Monitors are hidden when every project is a browser or mobile app.

## Connection (`/connection`)

The setup guide for the current project: the project token, the OTLP endpoint and the snippet for the chosen language and framework. Two tabs:

- **AI**: install the skills with `npx skills add tracewayapp/traceway --all` and paste a ready-made prompt with the URL and token. The `traceway-setup` skill instruments the application.
- **Manual**: step-by-step setup.

Browser and mobile projects also get **Source Map Upload** (browser and Android) or **Symbol Upload** (Flutter and iOS) with an upload token for CI, hidden from read-only members. Regenerating it invalidates the old one.

## Account (`/account`)

Per-person settings:

- **Personal Access Tokens**: name, prefix, access, created, last used, expires. Access is **Read only** (`twp_`, the default, can be non-expiring) or **Read and write** (`tww_`, expires within 1 to 90 days, and only writes where an admin turned on Agent Access). A token is shown once.
- **Contact Methods** and **Notification Rules** for on-call (`on-call.md`).

## Settings (`/settings`, owners and admins)

Stacked sections, with an organization switcher for people who manage several:

- **Organization Details**: the name, and the timezone (**Update Timezone**), which sets the timestamps every member sees and the default for new schedules.
- **Team Members**: name, email, role, status, per-project role overrides. **Pending Invitations**. Invitations carry a role.
- **Agent Access**: the **Allow agent writes** switch and the **Agent activity** log (when, who, credential, project, action, result). See below.

Per-project settings open from the project switcher (**Edit Project**), not from Settings: **Project** (name, framework), **Healthchecks** (drop healthy healthcheck requests, extra healthcheck paths), **Profiles** (extra pprof labels), **AI** (flagged term packs and custom terms), and the **Danger Zone**.

## How agents get access

- The `traceway` CLI and the `/mcp` server are **read only** for product data. The one exception is `traceway setup apply`, which submits an approved project setup plan with a `tws_` setup token. Logins use a browser device flow, `--password`, or a personal access token.
- **Writes are opt in at three levels**, all of which must agree:
  1. A write credential: a `tww_` token, a `traceway-write` login, or the `/mcp/write` server.
  2. An organization owner or admin turns on **Settings > Agent Access**.
  3. The person behind the credential has the role the change needs, as in the dashboard. A `readonly` member can still acknowledge and resolve pages, but not make the other writes.
- Agent credentials only reach the routes the CLI and MCP use. Account, member, token and settings routes refuse them, as do creating rules, channels, monitors, schedules and status pages, and every delete.
- Every agent write, accepted or refused by the switch, is in the agent activity log for 90 days.

The writes agents can make are archiving and unarchiving issues, acknowledging and resolving pages, enabling, disabling and snoozing alert rules, running a monitor, creating and editing post-mortems, and marking an endpoint slow. The `traceway-write` skill covers how to make them safely.

| Task | CLI | MCP tool |
|---|---|---|
| List projects | `traceway projects list` | `list_projects` |
| Pick the current project | `traceway projects use <id>` | the `project_id` param on any tool |
