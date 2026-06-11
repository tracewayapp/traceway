---
name: traceway-install-cli
description: Install the traceway CLI, authenticate against a Traceway instance, and select a project. Use when the user wants the Traceway command-line client set up on their machine, or when another Traceway skill needs the CLI and it is not installed yet.
---

# Install the Traceway CLI

The `traceway` CLI queries a Traceway observability instance from the terminal — exceptions, logs, endpoints, and metrics. It is designed to be first-class for both LLM agents (stable JSON output, stable error identifiers, no hung prompts) and humans.

## Step 1: Check for an Existing Install

```bash
traceway version
```

If this prints a version, skip to Step 3 (authenticate). Source builds report `dev`.

## Step 2: Install

### Option A: Prebuilt binary (preferred)

Binaries are published on the [tracewayapp/traceway releases page](https://github.com/tracewayapp/traceway/releases) under `CLI vX.Y.Z` tags (git tag format: `cli/vX.Y.Z`). Assets are named `traceway_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows), with `os` ∈ `darwin`, `linux`, `windows` and `arch` ∈ `arm64`, `x86_64`.

With `gh` (handles the fact that the latest release may be a Backend release, not a CLI one):

```bash
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m); [ "$ARCH" = "aarch64" ] && ARCH=arm64
TAG=$(gh release list --repo tracewayapp/traceway --limit 20 --json tagName --jq '[.[].tagName | select(startswith("cli/"))][0]')
gh release download "$TAG" --repo tracewayapp/traceway --pattern "traceway_*_${OS}_${ARCH}.tar.gz" --output - | tar -xz traceway
install -m 755 traceway ~/.local/bin/traceway && rm traceway
```

Without `gh`, resolve the download URL via the GitHub API:

```bash
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m); [ "$ARCH" = "aarch64" ] && ARCH=arm64
URL=$(curl -s "https://api.github.com/repos/tracewayapp/traceway/releases?per_page=20" \
  | grep -o "https://[^\"]*traceway_[^\"]*_${OS}_${ARCH}\.tar\.gz" | head -1)
curl -sL "$URL" | tar -xz traceway
install -m 755 traceway ~/.local/bin/traceway && rm traceway
```

Make sure `~/.local/bin` is on `PATH` (or install to `/usr/local/bin` instead).

### Option B: Build from source

Requires Go (see `cli/go.mod` for the minimum version):

```bash
git clone https://github.com/tracewayapp/traceway
cd traceway/cli
go build -o bin/traceway ./cmd/traceway
install -m 755 bin/traceway ~/.local/bin/traceway
```

The repo also ships a Nix dev shell: `nix develop` then `just build`.

### Verify

```bash
traceway version
```

## Step 3: Authenticate

```bash
traceway login --url https://<traceway-instance>
```

This prompts for email and password, then stores the JWT. Config (URL, username) goes to `$XDG_CONFIG_HOME/traceway/config.json`; credentials and the active project go to `$XDG_STATE_HOME/traceway/state.json`.

Login prompts interactively for the password — when running as an agent, ask the user to run the login command themselves. For non-interactive contexts where the user has the password in a secret store:

```bash
printf '%s' "$TRACEWAY_PASSWORD" | traceway login --url https://<traceway-instance> --username you@example.com --password-stdin
```

Never echo a password into the command line or shell history.

Multiple instances/accounts coexist via profiles:

```bash
traceway login --url https://traceway.example.com --profile work
traceway profiles list
traceway profiles use work
```

## Step 4: Select a Project

```bash
traceway projects list
traceway projects use <project-id>
```

The selected project is used implicitly by all subsequent commands.

## Step 5: Smoke-Check

```bash
traceway exceptions list --since 24h
traceway endpoints list --since 1h
```

## Usage Notes for Agents

- **Output**: `--output table|json|yaml`. Default is `table` on a TTY and `json` otherwise — piped/scripted calls always get machine-readable output. `--fields a,b,c` projects list responses to just those keys.
- **Exit codes**: `0` success, `1` generic/API error, `2` usage error, `3` connection failure, `4` auth failure, `5` not found, `6` rate limited, `7` server 5xx.
- **Error envelope** (stderr, JSON mode): `{"error":"token_expired","message":"...","hint":"traceway login","exit_code":4}`. The `error` field is a stable snake_case identifier — branch on it.
- **Mutations** (`exceptions archive` / `unarchive`) require `--yes` (or `TRACEWAY_ASSUME_YES=1`) in non-TTY contexts; without it they fail fast with exit 2 instead of hanging on a prompt.
- Run `traceway <command> --help` for full per-command flags.

## Command Reference

| Command | Purpose |
|---|---|
| `traceway login` / `logout` | Authenticate / forget the stored JWT |
| `traceway profiles {list,use}` | Manage multiple instances/accounts |
| `traceway projects {list,use}` | List or select the active project |
| `traceway exceptions list` | Recent grouped exceptions |
| `traceway exceptions show <hash>` | A single exception group + occurrences |
| `traceway exceptions archive/unarchive <hash>...` | Mutating; needs `--yes` non-interactively |
| `traceway logs query` | Query logs with severity / service / search filters |
| `traceway endpoints list` | Per-endpoint p50/p95/p99 stats |
| `traceway metrics query` | Time-series metric queries |
