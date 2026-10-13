# On-call: teams, schedules, escalation and pages

On-call turns an alert into a person's phone ringing. An alert alone posts to a channel and hopes someone is watching. A **page** keeps escalating until a human acknowledges it.

## How the pieces fit

An alert rule whose channel is an **escalation policy** fires. Instead of sending a message it opens a **page**. The page walks the policy step by step. Each step's targets resolve to people through a **schedule** (whoever is on call), a **team** (every member) or a direct **user**. Each person is reached through their own **contact methods**, in the order their personal **notification rules** say.

Two ideas to hold on to:

- **The page is the unit of work, not the message.** A rule that fires two hundred times opens one page. Re-fires bump its event counter and never restart the escalation clock.
- **Escalation stops on acknowledge, not on delivery.** Sending a message proves nothing. Traceway keeps climbing the policy until someone acknowledges or the policy runs out.

Setup order: create a team, build its schedule, create a policy whose first step targets the schedule, add an **Escalation policy** channel on the Alerts page and attach rules to it, then have every responder add contact methods. A responder with no contact methods is still paged on their account email.

## Where it lives

**On-Call** in the project sidebar (`/on-call`), with a badge counting the project's open pages. Five tabs:

| Tab | What it shows |
|---|---|
| **Pages** (default) | The page queue. Filters: All active (open or acknowledged), Open, Acknowledged, Resolved. Columns: severity, subject, level ("L2 of 3", "exhausted", "repeat N"), age, events, status, who acknowledged or resolved, actions. Rows can be selected for bulk acknowledge or resolve. |
| **Overview** | Per team and per schedule: who is on call now, until when, and who is next |
| **Teams** | Teams with members, owned projects and schedules |
| **Schedules** | Schedules with their rendered timeline and overrides |
| **Policies** | Escalation policies with their chain |

Teams, schedules and policies belong to the **organization**, so the page has an organization switcher. Pages belong to a project. The organization's **On-Call** page lists pages from every project (`organization.md`). The issue page shows the team that owns the project and who is on call for it now.

## Teams

A team is a named group of people that owns projects. Each project is owned by at most one team. Fields: name (unique in the organization), description, members, owned projects. Member order only affects how the team is listed: a policy step that targets a team pages every member at once. Deleting a team deletes its schedules, and is refused while a policy targets the team or one of its schedules.

## Schedules

A schedule answers one question: who is on call right now. It belongs to a team and has its own **timezone** (an IANA name, defaulting to the organization's). Set the timezone to where the people live. Handoffs and restrictions are in that timezone and hold their wall-clock time through daylight saving.

A schedule is built from **layers**. Each layer is an independent rotation:

| Field | Meaning |
|---|---|
| Rotation | `daily`, `weekly`, or custom (every N days, 1 to 365) |
| Handoff time | When the shift changes hands, HH:MM |
| Handoff day | The weekday of the handoff, for weekly rotations |
| Rotation start | The date the rotation counts from |
| Members | The people to rotate through, in order, one per period |
| Restrictions | Optional windows that limit when the layer covers anything |

Restrictions are **daily** (for example 18:00 to 09:00, wrapping past midnight) or **weekly** (for example Saturday 00:00 to Monday 09:00). With none, the layer covers every minute. If a handoff falls inside a window, the person changes mid-window.

**Stacking**: a later layer wins over earlier ones where they overlap. Exactly one person is on call at any moment: the highest layer covering it. If no layer covers a moment, nobody is on call and a step targeting the schedule reaches nobody.

**Overrides** temporarily replace whoever the layers picked, for an appointment, a swapped weekend or a vacation. They beat every layer, and the newest override wins where two overlap. An override lasts at most 30 days. The person covering must be an organization member but does not have to be in the schedule. **Any member** can create one, not only admins. It can be deleted by its creator, the person it covers, or an admin. Overrides end on their own, so there is no need to edit the rotation and remember to put it back.

The **timeline** shows a row per layer and a final **Schedule** row, which is who actually gets paged. Overrides appear in the Schedule row as dashed shifts marked Override. A red line marks now. Ranges are Day, Week, 2 Weeks and Month, up to 62 days.

Limits: 20 layers per schedule, 50 members per layer, 10 restrictions per layer. A schedule used by a policy cannot be deleted until the step is removed. A member removed from the organization is removed from every layer (a layer left empty is dropped) and their overrides are deleted. Team, schedule and policy names are unique in the organization.

Common shapes:

- **One rotation**: one weekly layer over everybody.
- **Business hours and out of hours**: two layers with opposite restrictions. The second, smaller group covers nights and weekends.
- **Primary and secondary**: two schedules, and a policy whose second step targets the secondary.

## Escalation policies

A policy is the ordered list of who to wake, and how long to wait for each.

- **Steps**: 1 to 10. Each holds 1 to 10 targets and a delay: "escalate after N minutes if not acknowledged" (1 to 1440, default 5).
- **Targets**: a **schedule** (whoever is on call when the step runs, overrides included), a **team** (every member), a **user**, or a **channel** (any notification channel of the organization except an escalation one, to post the page to Slack, a webhook or email alongside the people). Within one step each person is notified once.
- **Repeat**: 0 to 5. After the last step, its delay passes and the chain starts again from step 1, that many more times. After that the escalation is **exhausted**: nothing more is sent, but the page stays open.
- **Urgency**: **Auto** (critical becomes high, everything else low), **Always high** or **Always low**. It picks which of a responder's two notification chains runs. It is fixed when the page opens, so a low-severity re-fire never downgrades a page.

A page keeps a copy of its policy from the moment it opened, so editing a policy never changes pages already escalating. A policy used by a channel cannot be deleted.

## Pages

| Status | Meaning |
|---|---|
| **Open** | Nobody has taken it. Escalation is running. |
| **Acknowledged** | Someone is on it. Escalation stopped and every queued delivery is cancelled, including the later steps of personal notification chains. |
| **Resolved** | Finished. The dedup key is released, so the same condition can open a fresh page. |

- **Deduplication**: while a page is unresolved, the same rule and item (endpoint, metric, task, exception hash, monitor...) bump the existing page instead of opening a new one. Rules with no item dedup at rule level, so one noisy rule is one page. This is what stops a flapping error from becoming a pager storm.
- **Escalation**: step 1 is notified within a second or so. Later steps follow after their delays.
- **Acknowledge and resolve need no write access.** A read-only responder paged at 3am can still take the incident and close it.
- **Resolve and archive**: resolving a page opened by a New Issue or Error Regression rule can also archive its issue (that part needs write access). Archiving an issue in turn offers to resolve its open pages.
- **Auto-resolve**: a Monitor Down page resolves itself when the monitor recovers. A blip at night that recovers needs no human.
- **The page detail** shows the subject with severity, urgency and status, the rule, event count and last event, the alert body, the escalation chain with the current step marked (or "Escalation exhausted"), and the **delivery timeline**: every delivery with its step, target, method and status (pending, sent, failed, cancelled). That timeline is what to read when someone says they never got paged. Personal phone numbers and email overrides are masked there.

**Acknowledging without logging in**: every delivery to a person carries its own acknowledge link (`/ack/twk_...`). Opening it shows a summary of the page and an **Acknowledge** button, with no login. Opening the link is read only, so an email scanner cannot acknowledge by accident. The link can only acknowledge that one page, it is credited to the person it was sent to, and it stops working once the page is resolved. Deliveries to a channel target get the ordinary dashboard link instead, since a shared room has no single owner.

Page messages start with `[Page]`, or `[Page - escalation L2]` for later steps, and end with the acknowledge link.

## Contact methods and notification rules

Personal settings under **Account** (tabs **Contact Methods** and **Notification Rules**). Only the person can see or change their own.

| Contact method | Configuration |
|---|---|
| Email | Optional address. Blank uses the account email. |
| Slack | Incoming webhook URL, optional channel and username |
| Pushover | User key and app token |
| Telegram | Bot token and chat id |
| SMS | E.164 phone number such as `+12025550123`. Only offered when the server has Twilio configured. |

- SMS numbers must be verified before they are paged: a 6-digit code that expires in 10 minutes, with 5 attempts and at most 3 codes per hour.
- Every method has an enable toggle and a **Test** button ("If you received this, pages will reach you here.").
- **Notification rules** are two ordered chains, **High urgency** and **Low urgency**. Each step is "notify <method> after N minutes" (0 to 120, up to 10 steps). A typical high chain: Slack at 0, push at 2, SMS at 5. The whole chain is scheduled when the page reaches the person, and acknowledging cancels the steps that have not fired. If the first method cannot be used, the rest move earlier so the first usable one goes out at once.
- No chain means every enabled, verified method at once. No methods at all means the account email.

## Permissions

| Action | Who |
|---|---|
| View teams, schedules, policies, who is on call | Any organization member |
| Create or edit teams, schedules and policies | Organization owner or admin |
| Create an override | Any organization member |
| Delete an override | Its creator, the person it covers, or an admin |
| View, acknowledge and resolve pages | Anyone with read access to the project |
| Contact methods and notification rules | The person themselves |

## Reading and changing it from an agent

| Task | CLI | MCP tool |
|---|---|---|
| List pages | `traceway pages list [--status active\|open\|acknowledged\|resolved]` | `list_pages` |
| One page with its escalation and delivery log | `traceway pages show <id>` | `get_page` |
| Acknowledge (write) | `traceway-write pages ack <id>` | `acknowledge_page` |
| Resolve, optionally archiving the issue (write) | `traceway-write pages resolve <id> [--archive-issue]` | `resolve_page` |

Teams, schedules, overrides, policies and contact methods are dashboard only, both to read and to change. To answer "who is on call", send the user to **On-Call > Overview**, or to the issue page, which shows the on-call person for that project.
