<!-- GENERATED FILE: copied from cli/pkg/mcpserver/knowledge/oncall-configuration.md by cli/tools/skillgen. Edit there and run just gen-skills in cli/. -->
# Teams, schedules and escalation policies

Teams own projects and group responders. Schedules choose one responder for a time. Escalation policies decide who is notified next while a page remains unacknowledged. Teams, schedules and policies belong to the organization; alert channels and pages belong to projects. Read the learn on-call guide for runtime behavior.

## Workflow and commands

Use `--organization <id>` for organization resources. Select a project in that organization for the agent permission context. Organization owners/admins configure teams, schedules and policies. Any member can create a schedule override; deletion requires its creator, covering user or an admin.

```bash
traceway-write teams list --organization 1
traceway-write teams create --organization 1 --file team.json --yes
traceway-write schedules validate create --organization 1 --file schedule.json
traceway-write schedules create --organization 1 --file schedule.json --yes
traceway-write escalation-policies create --organization 1 --file policy.json --yes
traceway-write oncall now --organization 1
```

All three resources support `list`, `create`, `update <id>` and `delete <id>`. Create/update use `--file`. `teams set-members <id> --file F` takes `{"userIds":[1,2]}`; `teams set-projects <id> --file F` takes `{"projectIds":["<project-UUID>"]}`. Both replace the whole set. Each project can belong to only one team. Read the current state before replacements.

MCP uses `list_teams`, `create_team`, `update_team`, `delete_team`, `set_team_members`, `set_team_projects`; `list_oncall_schedules`, `get_oncall_schedule`, `create_oncall_schedule`, `update_oncall_schedule`, `delete_oncall_schedule`, `get_oncall_timeline`, `create_oncall_override`, `delete_oncall_override`; `list_escalation_policies`, `create_escalation_policy`, `update_escalation_policy`, `delete_escalation_policy`. Inputs are `organization_id`, `id` when needed, and `data` for JSON bodies. Override deletion also needs `child_id`.

## Team JSON

```json
{"name":"Platform","description":"API responders","memberUserIds":[1,2],"projectIds":["11111111-1111-4111-8111-111111111111"]}
```

Replace example IDs with existing organization users/projects. Organization admins can discover user IDs with `organization-members list --organization <id>` (`list_organization_members`); use `projects list` for project IDs. A policy targeting a team pages all members; member order is not a rotation. On update omitted member/project lists retain assignments; explicit empty arrays clear them. Deleting a team deletes its schedules and is refused when a policy references the team or its schedules.

## Schedule JSON

```json
{
  "teamId":3,"name":"Platform primary","description":"Weekly coverage",
  "timezone":"Europe/Berlin",
  "definition":{"schemaVersion":1,"layers":[{
    "name":"Primary","rotationType":"weekly","handoffDay":1,
    "handoffTime":"09:00","rotationStart":"2026-11-16",
    "userIds":[1,2],"restrictions":[]
  }]}
}
```

Send the complete desired definition on update and retain existing layer IDs. Rotation types are `daily`, `weekly`, `custom`; custom needs `intervalDays` (1–365). Weekly `handoffDay` is Monday=1 through Sunday=7. `rotationStart` is a local date, handoff time `HH:MM`, timezone an IANA name. Local handoffs remain at the same wall-clock time through daylight saving. Member order determines rotations.

Restrictions use `{"type":"daily","startTime":"09:00","endTime":"17:00"}` or weekly windows adding `startDay`/`endDay` (1–7). A later layer wins where layers overlap. No covering layer means nobody is on call. Limits: 20 layers, 50 users per layer, 10 restrictions per layer.

Verify the rendered schedule, including handoffs, gaps and daylight-saving changes:

```bash
traceway-write schedules show 4 --organization 1
traceway-write schedules timeline 4 --organization 1 --from 2026-11-16T00:00:00Z --to 2026-11-30T00:00:00Z
```

Timelines accept at most 62 days. `oncall now --organization 1` reads organization coverage; `oncall current` reads the selected project's team and coverage. These reads do not send pages.

## Temporary overrides

```json
{"userId":2,"startAt":"2026-11-18T09:00:00Z","endAt":"2026-11-18T17:00:00Z"}
```

Use `schedules create-override <schedule-id> --organization 1 --file override.json --yes`. An override beats all layers; the newest overlapping override wins. Maximum duration is 30 days; the covering person must belong to the organization but need not belong to the schedule. It expires automatically. To remove it: `schedules delete-override <schedule-id> <override-id> --organization 1 --yes`. Inspect the timeline after either change.

## Escalation policy JSON

```json
{
  "name":"Platform escalation",
  "definition":{"schemaVersion":1,"urgency":"auto","repeatCount":1,"steps":[
    {"delayMinutes":5,"targets":[{"type":"schedule","id":4}]},
    {"delayMinutes":10,"targets":[{"type":"team","id":3}]}
  ]}
}
```

There are 1–10 steps with 1–10 targets each. Targets are `schedule`, `team`, `user`, or `channel`; a channel target cannot itself be an escalation channel. IDs must belong to the organization. `delayMinutes` is the required wait after that step before escalating (integer 1–1440); the API rejects an omitted or zero delay. `repeatCount` is 0–5 extra passes. `urgency` is `auto`, `high` or `low`; auto maps critical severity to high and other severities to low.

Create a project channel `{"name":"Platform pager","channelType":"escalation","config":{"policyId":5}}`, using the returned policy ID, then route an alert rule to the returned channel ID. Ensure responders have contact methods and notification chains. A policy edit changes future pages only: open pages keep a snapshot. Exhausting a policy leaves a page open; acknowledgement stops escalation. Referenced policies/schedules cannot be deleted until references are removed. Do not use a channel test as a silent validation: it opens a real page and wakes responders.
