<!-- GENERATED FILE: copied from cli/pkg/mcpserver/knowledge/write-setup.md by cli/tools/skillgen. Edit there and run just gen-skills in cli/. -->
# Write setup and permissions


### 1. Install the write CLI

`traceway-write` is a separate binary with its own login, so a read-only `traceway` install can never be upgraded to write in place.

```bash
curl -fsSL https://cli.tracewayapp.com/install.sh | sh -s -- --write
traceway-write version
```

Prebuilt archives are on the [releases page](https://github.com/tracewayapp/traceway/releases) under `cli/vX.Y.Z` tags as `traceway-write_<version>_<os>_<arch>.tar.gz`. From source: `cd traceway/cli && go build -tags write -o bin/traceway-write ./cmd/traceway`.

### 2. Log in with write access

Ask the user to run the browser login themselves (in Claude Code: `! traceway-write login --url https://<instance>`). The approval page says the login asks for read and write access.

```bash
traceway-write login --url https://<instance>
```

Headless alternative: the user creates a **Read and write** personal access token (Account > Personal Access Tokens, prefix `tww_`, expires within 90 days) and stores it without echoing it:

```bash
printf '%s' "$TRACEWAY_WRITE_TOKEN" | traceway-write login --url https://<instance> --token-stdin
```

A read token (`twp_`) stored in `traceway-write` still cannot write: the server refuses every write with "This credential is read-only".

### 3. Organization opt-in

An organization owner or admin must turn on **Settings > Agent Access > Allow agent writes**. Until then writes answer 403 "Agent write access is turned off for this organization". Do not try to work around it: tell the user who can turn it on.

### 4. Select a project

```bash
traceway-write projects list
traceway-write projects use <project-id>
```

### MCP instead of the CLI

- Local stdio: `claude mcp add traceway-write -- traceway-write mcp` (uses the `traceway-write login`).
- Remote: add `https://<instance>/mcp/write` as an HTTP MCP server. The client runs OAuth and the consent page asks for read and write access. The plain `/mcp` server never exposes write tools, even to a write token.


## Operating with write access

Write access lets an agent take the routine actions an on-call engineer takes during and after an incident. It is opt-in at three levels, and every level has to agree before a write goes through:

1. **The credential.** Only write credentials can write: a `tww_` personal access token, a `traceway-write login`, or an MCP client connected to the `/mcp/write` server. A `twp_` token, a `traceway login` and the `/mcp` server are read only, whatever the person's role.
2. **The organization.** An admin has to turn on Settings > Agent Access. Until then every agent write is refused with a 403 that says so.
3. **The person.** The write runs with the role of the person who created the credential, under the same rules as the dashboard. A `readonly` member's write token cannot archive issues, change alerts, run monitors, edit post-mortems or change OpAMP rules: the write is refused with a 403 that starts "Your role in this project is read-only". `projects list` (`list_projects`) shows the person's role in each project. Acknowledging and resolving pages needs no write role, so any member's write token can do that.

Every write a write credential makes in the organization, accepted or refused because Agent Access is off, is recorded in the organization's agent activity log with the person, the credential, the project and the result.

