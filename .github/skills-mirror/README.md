# Traceway skills

Agent skills and Claude Code plugins for [Traceway](https://tracewayapp.com), the open-source observability platform.

| Skill | What it does |
|---|---|
| `traceway-setup` | Instruments a project to report to Traceway |
| `traceway` | Queries production telemetry through the `traceway` CLI and debugs issues to root cause |
| `traceway-learn` | Explains every part of Traceway and how it is meant to be used |
| `traceway-write` | Makes routine on-call changes through the `traceway-write` CLI, once an admin allows agent writes |

## Install

For any agent that reads `SKILL.md` (Claude Code, Cursor, Codex and more):

```bash
npx skills add tracewayapp/skills --all
```

As Claude Code plugins:

```
/plugin marketplace add tracewayapp/skills
/plugin install tw@traceway
/plugin install tw-write@traceway
```

`tw` holds the three read-only skills. `tw-write` holds `traceway-write` on its own, so write access stays a separate choice. The skill changes nothing by itself: writes also need the `traceway-write` CLI and an organization admin who turned on Agent Access. See [Agent write access](https://docs.tracewayapp.com/learn/agent-write-access).

## Where this comes from

This repository is generated. Each Traceway release publishes it from [tracewayapp/traceway](https://github.com/tracewayapp/traceway), and the next release overwrites anything changed here by hand. Send changes there instead:

- `traceway`, `traceway-learn` and `traceway-write` are generated from `cli/pkg/mcpserver/knowledge`.
- `traceway-setup` lives in `skills/traceway-setup`.
- The plugin manifests live in `.claude-plugin/` and `plugins/tw-write/.claude-plugin/`.

Plugin versions follow the Traceway release that last changed them.
