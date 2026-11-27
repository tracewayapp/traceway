---
name: traceway-setup
description: Analyze and instrument repositories for Traceway observability. Use when the user wants to plan, add, migrate, or verify Traceway or OpenTelemetry monitoring for backend, browser, full-stack, mobile or iOS, or AI-agent software, including project topology and user-approved setup-plan creation. Also runs on one part of a codebase when the request names it, e.g. `/traceway-setup track ai use` (find the LLM calls and instrument them with the OpenTelemetry GenAI conventions), background jobs, logs, metrics, the browser app, or a single directory. Also use it to embed a Traceway dashboard in the user's own application. Backends use OTLP/HTTP; browser and mobile clients use separate Traceway SDK projects. Accept a project token and instance URL or an organization setup token (`tws_...`) and URL; with no token, analyze first and guide setup-token acquisition. Treat any `tws_` token as a setup token.
---

# Set Up Traceway

Analyze the application first, agree on its Traceway project structure with the user, help them create that structure in the dashboard, and only then integrate and verify it.

## Operating Contract

- Inspect before asking for credentials or changing code.
- Explain what was detected, what is already tracked, and how each production component should report to Traceway.
- Present the proposed project map, or the one-line plan for a scoped run, and wait for the user to confirm it before implementation.
- There are two classes of credentials, treated differently:
  - **Setup tokens** (prefix `tws_`) are org-scoped, expire in 6 hours, and can only propose a setup plan; nothing is created until the user approves the plan on the Traceway website. They are designed to transit chat, and asking the user to paste one is expected.
  - **Project tokens and upload tokens** are long-lived. Never print their values, never ask the user to paste them into chat, and never commit them. When Step 3's apply script (or the `traceway` CLI) writes an approved plan's tokens into env files, read only the variable names and statuses from its output, never the values.
- Never print token values found in files or the environment. Report only that a value exists and the variable name that carries it.
- Never commit tokens, and never commit an env or build-config file that holds one.

## Invocation Forms

| Invocation carries | Path |
|---|---|
| A project token + url, sometimes with the framework and a source map or symbol upload token (the prompt a project's Connection page copies) | Fast Path for a single component, otherwise Steps 1-2, then integrate against the existing project. The user chose to paste these values: write each one straight into its untracked env file or tell the user which CI secret to create, under the names in "Where the token comes from" of the platform file, and never repeat a value back |
| A setup token (`tws_`) + url | Steps 1-2, then Step 3 from "Write the plan file" (3b) |
| No token | Steps 1-2, then Step 3 from the top |
| Words naming one part of the codebase (`track ai use`, `instrument the background jobs`, `only apps/worker`), with or without a token | "Scoped Runs" below; the token rows above still decide how the credential is obtained |

**Every backend integrates with OpenTelemetry.** There is one backend path, not one per language or web framework: Go, Node, Python, PHP, Java, .NET, Ruby, and everything else export OTLP/HTTP to `<instance>/api/otel/*`. The Traceway project is created with framework **OpenTelemetry**, which is the only backend option in the dashboard's framework picker. The native Traceway Go SDK is a deliberate exception used only when the user explicitly asks for it.

## Fast Path

When all of the following hold, skip Steps 2 and 3 and go straight to the integration steps:

- the repository has exactly one deployable component, and
- the user supplied (or the environment already carries) an instance URL and a project token (not a `tws_` setup token, which exists to create projects), and
- the token's project already matches that component.

Still run Step 1, because it decides *how* to instrument. Collapse "Propose and Confirm the Project Map" to a single confirmation line ("This is a Go API; it reports to your existing `<name>` project over OpenTelemetry") and skip the project-creation step entirely. The full map-and-creation ceremony exists for repositories with more than one deployable component, or where no project exists yet. Do not make a user who handed you a token sit through a project-map review for a single service.

## Scoped Runs

The skill also runs on one part of a codebase. A request that names a part, instead of asking for the whole setup, is a scoped run: `/traceway-setup track ai use`, "instrument the background jobs", "add Traceway to `apps/worker` only", "check that the setup works". The words after the skill name, or the request that loaded the skill, carry the scope. A part is an **area** (one kind of telemetry) or a **path** (a directory, package or named component), and the two combine: "track AI use in `services/assistant`".

| The request names | Typical wording | Do |
|---|---|---|
| **AI / LLM usage** | integrate, instrument, track or monitor: AI use, LLM or model calls, tokens, cost, prompts, agents, a chatbot | Follow [ai-usage.md](ai-usage.md) from top to bottom. It carries its own analysis (its Steps A and B are rule 1 below for this scope), instrumentation and verification; read a step of this file only when it sends you here. |
| **Background work** | tasks, jobs, cron, queues, workers, schedulers, CLI commands | Step 1 items 1, 3 and 5, then Step 5, then the task check of Step 9 |
| **Endpoints** | routes, endpoint grouping, request tracing | Step 1 items 1-4, then Step 4 through "Verify endpoint grouping" |
| **Errors**, **logs** or **metrics** | exceptions, issues; log shipping; custom or business metrics | Step 1 items 1 and 3, then "Errors", "Logs" or "Metrics" in Step 4, then the matching check of Step 9 |
| **Browser app** | frontend, RUM, web vitals, session replay, source maps | Step 1 items 1, 3, 4 and 7, then the browser part of Step 7 with [rum.md](rum.md), then check 2 of Step 9 |
| **Mobile app** | Flutter, React Native, Android, iOS, crashes, symbols | Step 1 items 1, 3 and 8, then the mobile part of Step 7, then check 3 of Step 9 |
| **Host or cluster metrics** | server metrics, the agent, Kubernetes | Step 1 item 9, then Step 8 |
| **Dashboard embedding** | embed a dashboard, customer-facing charts | "Embedding Dashboards in the User's Application" |
| **Verification** | verify, check, "is it working", "why is nothing showing up" | Step 1 item 3, then Step 9 for what is already set up. Change nothing; report what fails and why. |
| **A path or component** | a directory, "only the worker", "just the API" | The whole flow with every step limited to that path. One component with a token is usually the Fast Path. |

For a verification-only request, use existing configuration and report missing prerequisites. The implementation and project-creation steps apply only when the user asks to set up instrumentation. Combine verification with any named area or path to select the relevant checks.

The wording varies, so match on meaning. When a request fits no row, name the scopes that exist and ask which one is meant. When it names no part at all, run the full setup.

Rules for every scoped run:

1. **Inspect first, but only what the scope needs.** Always run Step 1 items 1 and 3 for the code in scope: the language, and the observability that is already there, including where it exports and which variable names carry the endpoint and token. Add the Step 1 items the table lists. Follow imports to shared clients and deployment configuration when the scoped component depends on them.
2. **Reuse what exists.** A scoped run adds to the current setup: the same Traceway project, the same token variable, the same exporter and `service.name`. Do not rebuild the project map, do not rename projects or variables, and do not touch instrumentation outside the scope, even when it looks wrong. Report it instead.
3. **Make sure the scope can reach Traceway.** If the code in scope already exports to Traceway, build on that. If it exports OpenTelemetry somewhere else, ask whether Traceway replaces that destination or runs next to it. If it exports nothing, set up the minimum the scope needs (for a backend area: the exporter from "Exporter configuration" in Step 4 and the one signal the area uses) with the credential path from "Invocation Forms", limited to the one project this code reports to. Say plainly that the rest of the setup was not done.
4. **Confirm in one line, not with a project map.** State the scope, the project it reports to and what will change ("I will trace the 4 OpenAI call sites in `src/llm/`; they report to your existing `Shop Backend` project"). Wait for confirmation before changing code or provisioning a project.
5. **Verify only the scope**, with the checks named in the table, and never report success for a check you did not run.
6. **Finish with a scope report**: what changed, what was verified and how, what could not be verified, and what you noticed outside the scope, each with the scoped request that would handle it (for example "the nightly export job has no task span: `/traceway-setup instrument background jobs`").

The Operating Contract applies unchanged: the same credential rules, and no code change before the user has seen what you found.

## Step 1: Analyze the Architecture

Before changing anything, build a picture of what is deployed and what is already instrumented:

1. **Frameworks and languages**: detect them from `package.json`, `go.mod`, `composer.json`, `requirements.txt`/`pyproject.toml`, `pubspec.yaml`, `build.gradle`(`.kts`), `Package.swift`, `*.xcodeproj`/`*.xcworkspace`, `Podfile`, and source extensions. For iOS/Apple targets, note whether the sources are Swift or Objective-C. That choice picks the path in "Frontend and Mobile" (Swift gets the native SDK, and so does an Objective-C app through one small Swift bridge file; only a stack with no Traceway mobile SDK falls back to OTel).
2. **Deployable components and entry points**: inventory APIs, SSR servers, browser apps, workers, schedulers, CLIs, and mobile applications. Treat code organization as evidence, not proof that something is deployed. Note the deployment target while you are here: Kubernetes manifests, a Helm chart, Kustomize overlays, `Dockerfile` plus a compose file, or cloud-init / Ansible / Terraform. It decides the host-metrics path in Step 8.
3. **Existing observability**: find OpenTelemetry configuration, Traceway SDKs, Sentry, Datadog, New Relic, Honeycomb, logging exporters, tracing middleware, source map or symbol upload steps, and observability environment-variable names. Explain whether Traceway will replace, coexist with, or extend each integration. Never display discovered credential values.
4. **Production role of JS meta-frameworks** (Next.js, SvelteKit, Remix, Nuxt): never assume full-stack.
   - **Frontend-only signals**: `output: 'export'` in `next.config.*` (static export, so there is no server in production); no API routes (`app/api/**/route.*`, `pages/api/*`) or only trivial ones; no `'use server'` server actions; `rewrites`/proxy config or a `NEXT_PUBLIC_API_URL`-style env var pointing the browser at an external API; a separate backend service in this repo, another repo, or another language; static hosting in the deploy config (S3/CloudFront, GitHub Pages, nginx serving `out/`).
   - **Server signals**: API routes with real logic, `'use server'` server actions, database clients imported in server code, SSR reading its own data layer, `next start` or standalone output in the deploy config.
   - **Mixed or unclear**: ask how the application is deployed and whether the framework server does production work worth tracking.

   Frontend-only means integrate ONLY the browser side with the frontend SDK; do NOT add OTel to, or otherwise instrument, the framework's server side. A separate backend is its own component with its own Traceway project.
5. **Background work**: find cron jobs, queue consumers, schedulers, CLI commands, and long-running workers. Record whether libraries already emit `CONSUMER` spans.
6. **AI/LLM usage**: look for provider HTTP requests and shared model clients as well as dependencies for `openai`, `@anthropic-ai/sdk`, `anthropic`, `langchain` / `@langchain/*`, `ai` (Vercel AI SDK), `litellm`, `google-generativeai`, `cohere`, `openrouter`, and agent frameworks (`langgraph`, `crewai`, `autogen` / `ag2`, `pydantic-ai`, `@openai/agents`, `@mastra/*`, `llamaindex` / `llama-index`, `semantic-kernel`). Then decide whether this is a **conversational product** (chatbot, assistant, agent) rather than one-shot LLM calls: look for chat or streaming-chat routes (`/chat`, `/completions`, SSE or websocket handlers feeding an LLM), message-history persistence (a `messages`/`conversations` table or thread ids), tool/function-calling definitions passed to the model, or any agent framework above. When any of these turn up, the AI integration follows `ai-usage.md` in this skill directory (Step 6), which starts with a call-site inventory; if the product is conversational or tool-calling, also read `ai-agent.md` before proposing it.
7. **Browser build and release flow**: identify the bundler, output directory, source-map settings, deploy pipeline, and existing artifact uploads.
8. **Mobile build and release flow**: determine whether Flutter is obfuscated, Android uses R8/minification, iOS produces dSYMs, and whether multiple platform directories are one cross-platform product or independently released apps.
9. **Deployment and host metrics**: inspect Dockerfiles, Compose, Kubernetes, Helm, Terraform, Ansible, cloud-init, PaaS configs, and CI/CD. Determine whether the backend runs on a host where the Traceway OTel Agent is applicable.

Present the findings before asking questions:

| Component | Repository evidence | Production role | Current tracking | Proposed Traceway project | Integration | Credentials needed |
|---|---|---|---|---|---|---|
| `apps/api` | Go module + Gin routes | Backend API | OTel / none / vendor | `Product Backend` | OTel | Runtime token |
| `apps/web` | Svelte + Vite | Browser app | Existing SDK / none | `Product Web` | Traceway Svelte SDK | Runtime + upload token |

Use actual paths and findings. Include unresolved deployment assumptions explicitly.

## Step 2: Propose and Confirm the Project Map

Explain these default boundaries:

| Application boundary | Traceway project | What belongs in it |
|---|---|---|
| **Backend system** | One project with framework **OpenTelemetry** | API endpoints, child spans, issues, background tasks, AI traces, logs, host metrics, and the app's own custom metrics when it has some (Step 4, "Metrics"). APIs, workers, schedulers, and the host agent share the project token; distinguish them with stable `service.name` values. |
| **Browser frontend** | One separate project per deployed browser application, using **React**, **Svelte**, **Vue.js**, or **jQuery** | Browser errors, web vitals, session replay, distributed-trace linkage, source maps, and browser release metadata. |
| **Mobile app** | One separate project per independently released application, using **Flutter**, **React Native**, **Android**, or **iOS** | Mobile errors, crashes and hangs, sessions, wireframe replay, logs, app performance metrics, and build symbols or mappings. A single Flutter product targeting Android and iOS normally uses one project; separate native apps use separate projects. |
| **Full-stack JS app** | Two projects when its server runs in production | Server-side work goes to the OpenTelemetry backend project; browser-side work goes to the browser project. |

The dashboard's framework picker offers exactly these nine options: OpenTelemetry for every backend, React / Svelte / Vue.js / jQuery for browsers, and Flutter / React Native / Android / iOS for mobile. There is no Gin, Django, Laravel, Next.js, Remix, or plain-JavaScript entry, and none is needed. A Next.js or Remix browser project selects **React**, and its server side selects **OpenTelemetry** like any other backend. A plain-JavaScript browser app (`@tracewayapp/frontend` with no wrapper) selects the closest browser option; the four behave the same in Traceway and differ only in the Connection page snippet. The framework-specific setup for a backend lives on the project's Connection page, which asks for the language and web framework after the project exists.

Do not create one project per backend process by default. Keeping the API, workers, AI calls, and server metrics together preserves their operational context and distributed traces. Split backend projects only for a real product, ownership, access-control, compliance, or data-isolation boundary.

Ask the user to confirm only what the repository cannot establish safely:

1. Which detected components are deployed and should be tracked?
2. Traceway Cloud or self-hosted, and which organization should own the projects?
3. Are the proposed names and project boundaries correct?
4. Does the backend run on a VM/host, Kubernetes, or serverless/PaaS, and should host metrics be collected?
5. Do mobile directories represent one cross-platform product or separate released apps?

Each confirmed row must also settle what Step 3's plan file needs:

- The **framework value**, one of the nine: `opentelemetry`, `react`, `svelte`, `vuejs`, `jquery`, `flutter`, `react-native`, `android`, `ios`.
- The **env wiring**: which untracked file holds the project token (`envFile`) and under which name (`envVar`). Every SDK and the OTLP exporter take the bare token; there is no connection string to compose. Use the names the dashboard's Connection page uses, so a value copied from there matches:

  | Project | `envFile` | `envVar` | The app reads it through |
  |---|---|---|---|
  | Backend (OpenTelemetry) | the service's `.env` | any name, e.g. `TRACEWAY_BACKEND_TOKEN` | the OTLP `Authorization: Bearer` header |
  | Browser, Vite | `.env` | `VITE_TRACEWAY_TOKEN` | `import.meta.env` |
  | Browser, SvelteKit | `.env` | `PUBLIC_TRACEWAY_TOKEN` | `$env/static/public` |
  | Browser, Next.js | `.env.local` | `NEXT_PUBLIC_TRACEWAY_TOKEN` | `process.env` |
  | Flutter | `.env` at the app root | `TRACEWAY_TOKEN` | `--dart-define-from-file=.env` and `String.fromEnvironment` |
  | Android | `local.properties` | `TRACEWAY_TOKEN` | `buildConfigField` and `BuildConfig` |
  | iOS | `Traceway.xcconfig` (untracked) | `TRACEWAY_TOKEN` | an `Info.plist` key and `Bundle.main` |

  On a server other than Traceway Cloud the apply step also writes the server URL into the same file, under `urlEnvVar` (default: `envVar` with `TOKEN` replaced by `URL`, so `VITE_TRACEWAY_TOKEN` gets `VITE_TRACEWAY_URL`); on Cloud the SDKs need no URL. Leave `envFormat` out: `"connectionString"` is the deprecated 1.x form, kept only so old plans still validate.
- The **upload token**, for projects whose release build uploads source maps, Flutter symbols, Android mappings or iOS dSYMs. Traceway creates one for every React, Svelte, Vue.js, jQuery, Flutter, Android and iOS project (none for OpenTelemetry or React Native). Add `uploadTokenEnvFile` and/or `uploadTokenEnvVar` only when a local build step will read it from a file; otherwise leave both out and the user copies it from the project's Connection page into the CI secret store. The file must be untracked and must not reach the shipped app: browsers a separate file such as `web/.env.sourcemaps`; Flutter a separate `.env.symbols`, never the `.env` the build passes to `--dart-define-from-file`; Android `local.properties` (the Gradle plugin reads it there); iOS the untracked `Traceway.xcconfig` itself (Xcode hands its settings to the dSYM upload script, and `Info.plist` does not reference the token), never a committed `.xcconfig`. The default variable is `TRACEWAY_SOURCEMAP_TOKEN` for browsers and `TRACEWAY_UPLOAD_TOKEN` for Flutter, Android and iOS, the names the upload tools read; a public prefix such as `VITE_` is refused. The platform files say how each upload step reads it.
- The **deployment placement**, from Step 1's deployment analysis: when the credential cannot be hardcoded into the repository because of how the component is deployed (Vercel/Fly/K8s/CI-injected env, secret managers), add a `deployment` block with the platform name and the exact command or UI path, using the literal placeholder `<token>` for the value (and `<url>` for the server URL). The website substitutes the real values after approval; never put real tokens in the plan. A mobile app built in CI usually gets both: an `envFile` for local builds and a `deployment` block for the CI secret.

Do not modify code until the user confirms this map.

## Step 3: Create the Projects

The default path: submit the confirmed map as a setup plan, the user approves it visually on the Traceway website, and a short curl script writes the resulting project tokens (and the upload tokens the plan asks for) into env files. Nothing needs to be installed beyond `curl` and `jq`. Existing correctly mapped Traceway projects may be reused (the plan matches projects by name, so listing an existing name reuses it instead of duplicating).

### 3a. Get a setup token (skip when the invocation carried one)

Branch on account state; ask only if the repository and conversation do not establish it:

- **Has a Traceway account** (with or without projects): "Open `<instance>/setup?mode=add` (Traceway Cloud: https://cloud.tracewayapp.com/setup?mode=add). Log in if prompted; you come back to the same page. It shows a setup token starting with `tws_` (if your organization had a setup proposal before, press **Generate New Token**; with several organizations, pick the one the projects belong to). Paste it here and keep the page open." The token expires in 6 hours; generating a new one revokes the previous one. In the dashboard the same page is the **Add Projects with Your Agent** button on the organization's Projects page. Never send an account that has projects to a plain `<instance>/setup`: without a pending proposal it redirects to the dashboard.
- **No account yet**: "Open `<instance>/register` (Traceway Cloud: https://cloud.tracewayapp.com/register). Step 1 creates your account and organization. On step 2, keep 'AI' selected; it shows a setup token starting with `tws_`. Paste it here and leave that page open: my proposal will appear there for your approval, and the projects show up live once you approve."

Do not have the user create projects in the web UI on this path, and do not proceed without the token.

### 3b. Write the plan file

Write `traceway-setup-plan.json` from the confirmed map, one entry per project, with the env wiring from Step 2:

```json
{
  "projects": [
    {"name": "Product Backend", "framework": "opentelemetry",
     "envFile": "backend/.env", "envVar": "TRACEWAY_BACKEND_TOKEN"},
    {"name": "Product Web", "framework": "svelte",
     "envFile": "web/.env", "envVar": "PUBLIC_TRACEWAY_TOKEN",
     "deployment": {"platform": "Vercel",
       "instructions": "vercel env add PUBLIC_TRACEWAY_TOKEN production\n# paste: <token>"}},
    {"name": "Product iOS", "framework": "ios",
     "envFile": "ios/Traceway.xcconfig", "envVar": "TRACEWAY_TOKEN",
     "uploadTokenEnvVar": "TRACEWAY_UPLOAD_TOKEN"}
  ]
}
```

What the apply step writes for each entry:

- `envVar` gets the project token. On a server other than Traceway Cloud the same file also gets the server URL under `urlEnvVar` (default: `envVar` with `TOKEN` replaced by `URL`, `PUBLIC_TRACEWAY_TOKEN` gets `PUBLIC_TRACEWAY_URL`, `traceway.token` gets `traceway.url`). On Cloud no URL is written: the SDKs default to it.
- The upload token only when the entry names `uploadTokenEnvFile` or `uploadTokenEnvVar` (without a file it goes into `envFile`). Its variable defaults to `TRACEWAY_SOURCEMAP_TOKEN` for browsers and `TRACEWAY_UPLOAD_TOKEN` for Flutter, Android and iOS; off Cloud the same file also gets `TRACEWAY_URL` for the upload tools. Without these fields the token stays on the project's Connection page and the setup page lists it under "Next Steps: Upload Tokens".
- An `.xcconfig` value stores `//` as `/$()/` (a bare `//` starts a comment there); a `.properties` file also takes dotted keys such as `traceway.token`; missing directories are created.

Rules: `framework` is one of the nine values from Step 2; every `envFile` and `uploadTokenEnvFile` must be untracked (create it and confirm it is gitignored first); `envVar` follows the naming rules of Step 2; an upload token never goes into a file the app bundles or a committed one, and never gets a public prefix (the server refuses `VITE_`, `NEXT_PUBLIC_`, `PUBLIC_` and the like); `deployment.instructions` uses the literal `<token>` (and `<url>`) placeholders, never a real value. Do not set `envFormat`.

### 3c. Submit the plan and wait for approval (curl)

Use `curl` and `jq` directly for this step. Do NOT install the `traceway` CLI for it; nothing gets installed on this path. (Only if a `traceway` CLI is already installed whose `traceway setup apply --help` mentions `uploadTokenEnvFile` is `printf '%s' "$TRACEWAY_SETUP_TOKEN" | traceway setup apply --url "$TRACEWAY_URL" --plan traceway-setup-plan.json --token-stdin` an equivalent one-step alternative; an older CLI writes the wrong form.) If `jq` is unavailable and cannot be installed, use the manual fallback (3e).

Export the credentials first; setup tokens may transit chat, so this is fine:

```bash
export TRACEWAY_URL="<instance>"          # no trailing slash
export TRACEWAY_SETUP_TOKEN="tws_..."
```

Validate the token and show the organization (this response carries project names only, safe to display); keep its `organizationId` for the review link:

```bash
curl -s -w '\nHTTP %{http_code}\n' -H "Authorization: Bearer $TRACEWAY_SETUP_TOKEN" \
  "$TRACEWAY_URL/api/setup/session"
```

Submit the plan as a draft:

```bash
curl -s -w '\nHTTP %{http_code}\n' -X PUT \
  -H "Authorization: Bearer $TRACEWAY_SETUP_TOKEN" -H 'Content-Type: application/json' \
  --data-binary @traceway-setup-plan.json "$TRACEWAY_URL/api/setup/plan"
```

`200 {"status":"pending"}` means the draft is up; tell the user: "Review the proposal on the Traceway page you have open (or `<instance>/setup?mode=add&organizationId=<organizationId>`) and press **Approve Setup**." A `422` is a validation message: fix the plan and resubmit. A `401` means the token is invalid or expired: ask the user for a fresh one from `<instance>/setup?mode=add` (**Generate New Token**).

Then wait for the decision with the script below. The script exists because the decision response carries the project and upload tokens: values must flow API to env file without ever reaching stdout, your context, or the chat. Write it to `traceway-setup-wait.sh` exactly as given and run `sh traceway-setup-wait.sh`. Never fetch `GET /api/setup/plan` directly, and never read the env files it writes.

```sh
#!/bin/sh
# Waits for the setup decision, then writes each project's token (and, off
# Traceway Cloud, the server URL) into the env file named by the plan, plus the
# upload token where the plan asks for it. Prints statuses and names only.
set -eu
PLAN=${1:-traceway-setup-plan.json}
AUTH="Authorization: Bearer ${TRACEWAY_SETUP_TOKEN:?}"
URL=${TRACEWAY_URL:?}
URL=${URL%/}
CLOUD=https://cloud.tracewayapp.com
deadline=$(($(date +%s) + 1800))
while :; do
  resp=$(curl -s -H "$AUTH" "$URL/api/setup/plan")
  status=$(printf '%s' "$resp" | jq -r '.status // "error"')
  case "$status" in
    approved) break ;;
    rejected)
      echo "rejected: $(printf '%s' "$resp" | jq -r '.reason // "(no reason given)"')"
      exit 2 ;;
    pending|none) ;;
    *) echo "unexpected response, the setup token may have expired"; exit 1 ;;
  esac
  if [ "$(date +%s)" -ge "$deadline" ]; then
    echo "timed out waiting for approval; rerun when the user is ready"
    exit 3
  fi
  sleep 3
done

# write_var FILE KEY VALUE: replaces KEY=... (or "export KEY=...") in place or
# appends it; the value travels through the environment, never argv or stdout.
write_var() {
  case "$1" in *.xcconfig) v=$(printf '%s' "$3" | sed 's#//#/$()/#g') ;; *) v=$3 ;; esac
  case "$1" in *.properties) sep='=:' ;; *) sep='=' ;; esac
  dir=$(dirname "$1"); mkdir -p "$dir"
  tmp=$(mktemp "$dir/.tmp-env-XXXXXX")
  touch "$1"
  K=$2 V=$v S=$sep awk '
    { line = $0; sub(/\r$/, "", line); rest = line
      sub(/^[ \t]*/, "", rest); sub(/^export[ \t]+/, "", rest)
      if (index(rest, ENVIRON["K"]) == 1) {
        tail = substr(rest, length(ENVIRON["K"]) + 1); sub(/^[ \t]*/, "", tail)
        if (tail != "" && index(ENVIRON["S"], substr(tail, 1, 1)) > 0) {
          print substr(line, 1, length(line) - length(rest)) ENVIRON["K"] "=" ENVIRON["V"]; done = 1; next
        }
      }
      print }
    END { if (!done) print ENVIRON["K"] "=" ENVIRON["V"] }' "$1" > "$tmp"
  mv "$tmp" "$1"
}
url_var_for() {
  case "$1" in
    *TOKEN) printf '%sURL' "${1%TOKEN}" ;;
    *token) printf '%surl' "${1%token}" ;;
    *.*) printf '%s.url' "$1" ;;
    *) printf '%s_URL' "$1" ;;
  esac
}

i=0
n=$(jq '.projects | length' "$PLAN")
while [ "$i" -lt "$n" ]; do
  field() { jq -r ".projects[$i].$1 // empty" "$PLAN"; }
  name=$(field name); framework=$(field framework)
  envfile=$(field envFile); envvar=$(field envVar); format=$(field envFormat)
  urlvar=$(field urlEnvVar); ufile=$(field uploadTokenEnvFile); uvar=$(field uploadTokenEnvVar)
  project=$(printf '%s' "$resp" | jq -c --arg n "$name" '[.projects[] | select(.name == $n)][0] // empty')
  if [ -z "$project" ]; then
    echo "$name: missing from the approved plan"
    i=$((i + 1)); continue
  fi
  pstatus=$(printf '%s' "$project" | jq -r '.status')
  inst=$(printf '%s' "$project" | jq -r '.backendUrl // empty'); inst=${inst%/}
  if [ -z "$inst" ] || { [ "$inst" = "$CLOUD" ] && [ "$URL" != "$CLOUD" ]; }; then inst=$URL; fi
  [ -n "$urlvar" ] || urlvar=$(url_var_for "$envvar")
  wroteurl=
  msg="$name: $pstatus"
  if [ -n "$envfile" ] && [ -n "$envvar" ]; then
    token=$(printf '%s' "$project" | jq -r '.token')
    if [ "$format" = connectionString ]; then
      write_var "$envfile" "$envvar" "$token@$inst"
    else
      write_var "$envfile" "$envvar" "$token"
      if [ "$inst" != "$CLOUD" ]; then write_var "$envfile" "$urlvar" "$inst"; wroteurl=1; fi
    fi
    msg="$msg, wrote $envvar${wroteurl:+ and $urlvar} to $envfile"
  fi
  utoken=$(printf '%s' "$project" | jq -r '.sourceMapToken // empty')
  if [ -z "$ufile" ] && [ -n "$uvar" ]; then ufile=$envfile; fi
  if [ -n "$utoken" ] && [ -n "$ufile" ]; then
    if [ -z "$uvar" ]; then
      case "$framework" in
        react|svelte|vuejs|jquery|nextjs) uvar=TRACEWAY_SOURCEMAP_TOKEN ;;
        *) uvar=TRACEWAY_UPLOAD_TOKEN ;;
      esac
    fi
    write_var "$ufile" "$uvar" "$utoken"
    msg="$msg, wrote $uvar to $ufile"
    if [ "$inst" != "$CLOUD" ]; then
      ukey=TRACEWAY_URL
      case "$ufile" in *.properties) case "$envvar" in *.*) ukey=$urlvar ;; esac ;; esac
      if [ -z "$wroteurl" ] || [ "$ufile" != "$envfile" ] || [ "$ukey" != "$urlvar" ]; then
        write_var "$ufile" "$ukey" "$inst"
      fi
    fi
  elif [ -n "$utoken" ]; then
    msg="$msg, upload token on the Connection page"
  fi
  echo "$msg"
  i=$((i + 1))
done
echo "done: deployment credentials and unwritten upload tokens are on the Traceway page the user has open ($URL/setup?mode=add)"
```

Env files the script creates come out mode 0600 via mktemp. Failure handling:

- **Rejected (exit 2)**: the script prints the user's rejection reason. Revise the map with the user, update the plan file, submit again (a new PUT replaces the pending draft), and rerun the wait script. Already-approved projects are matched by name, never duplicated.
- **Expired or invalid token (401 / exit 1)**: ask the user for a fresh token from `<instance>/setup?mode=add` (**Generate New Token**), re-export it, resubmit, rerun.
- **Timeout (exit 3)**: resubmit and rerun when the user is ready; it is safe.

Delete the plan file and `traceway-setup-wait.sh` after a successful apply; they hold no secrets but they are setup litter.

### 3d. Confirm and handle deployment credentials

On approval the Traceway page celebrates and lands the user on the new project's dashboard, which doubles as the confirmation that the projects exist. Before that, the page shows **Next Steps** panels: "Deployment Credentials" for every project with a `deployment` block (the exact command with the real values), and "Upload Tokens" for every upload token the plan did not ask to have written (with the variable the release build reads). Walk the user through both before they continue. The panels stay reachable at `<instance>/setup?mode=add&organizationId=<organizationId>` (the link `traceway setup apply` prints) until the next proposal is submitted in that organization. Never relay those values through chat; later, each project's Connection page carries its project token and upload token.

### 3e. Manual fallback

When the user prefers clicking, `curl` or `jq` is unavailable, or apply keeps failing, follow `dashboard-project-setup.md`: the user creates each project in the dashboard UI and puts the tokens straight into their env files or secret store. On this path the old rule applies in full: project tokens never transit chat.

**Upload tokens: the uploaders read fixed variable names.** `traceway-sourcemaps` reads `TRACEWAY_SOURCEMAP_TOKEN`, `dart run traceway:upload_symbols` and the iOS dSYM script read `TRACEWAY_UPLOAD_TOKEN`, and the Android Gradle plugin takes `uploadToken` (and `url`) from the build script, which reads `TRACEWAY_UPLOAD_TOKEN` (and `TRACEWAY_URL`; see `android.md`). Every uploader sends to Traceway Cloud unless `TRACEWAY_URL` (or `--url`) names a self-hosted server; a blank value counts as unset. Either name the CI secret exactly that, or keep a component-specific secret and pass it explicitly:

```bash
traceway-sourcemaps --token "$TRACEWAY_WEB_UPLOAD_TOKEN" --directory ./dist   # + --url "$TRACEWAY_URL" for a self-hosted server
```

Inventing a name like `TRACEWAY_WEB_UPLOAD_TOKEN` and then calling the uploader with no `--token` fails at release time, not at setup time, so make the choice explicit in the CI step. Every React, Svelte, Vue.js, jQuery, Flutter, Android and iOS project has an upload token from the start; it is on the project's **Connection** page (Source Map Upload or Symbol Upload card), which can also regenerate it.

## Integration Paths

Pick the path by project type, and pick the project type from the "Analyze the Architecture" findings, never from the framework name alone (a Next.js repo can be a full-stack app or just the frontend of a separate backend). Per path, this is not negotiable per framework; it is how Traceway is designed to receive data:

| Project type | Path |
|---|---|
| **Backend** (any language) | OpenTelemetry, exporting OTLP/HTTP to `<instance>/api/otel/v1/*`. Always, including Go. |
| **Frontend** (browser SPA, or a JS meta-framework running frontend-only in production) | Traceway `@tracewayapp/<framework>` SDK + bundler plugin + source map upload (see "Frontend and Mobile" below). |
| **Full-stack JS** (Next.js, SvelteKit, Remix, actually serving its API/SSR in production per "Analyze the Architecture") | BOTH sides, each under its own Traceway project: server side via OpenTelemetry AND browser side via the frontend SDK. |
| **Mobile** (Flutter, React Native, Android, native iOS in Swift or Objective-C) | The Traceway platform SDK. Never OTel. Sole exception: an iOS/Apple app on a stack with no Traceway SDK has no native SDK, so it uses an OTel library (e.g. Honeycomb) exporting to Traceway like a backend (see "Frontend and Mobile"). An Objective-C app uses the iOS SDK through a Swift bridge file. |

The rules every backend integration must satisfy, and the table of what each span becomes, are in Step 4 where they are applied.

## Embedding Dashboards in the User's Application

This is a separate integration from sending telemetry, and it needs no setup token. When the user wants a Traceway dashboard shown inside their own product, such as each customer's KPIs in an admin panel, read [dashboard-embedding.md](dashboard-embedding.md) and follow it: their backend asks Traceway for a short-lived signed link per authorized viewer, and their frontend shows it in an iframe.

- It uses a **Read and write** personal access token (`tww_`) that the user creates and stores in their backend's secrets. Treat it like a project token: never print it, never ask for it in chat, never commit it. A project ingest token cannot issue links.
- Per-customer embeds filter on a tag the metrics already carry, such as `account_id`. When you instrument the backend in Step 4 and the user plans to embed, put that attribute on the metrics the dashboard will chart, and keep its values bounded (an account or tenant id, never a user or request id).
- The account in the filter comes from the viewer's server-side session, never from a browser parameter.
- The dashboard itself is built in the Traceway UI or with the `traceway-write` skill; this skill only wires the embed into the application.

## Step 4: Backend OTel Setup

The same shape in every language:

1. Install the language's OpenTelemetry SDK plus the auto-instrumentation for the web framework and database clients.
2. Point the OTLP/HTTP exporter at Traceway with the project token as a Bearer header.
3. Set `service.name` (becomes the Server Name in Traceway) and `service.version` (enables release comparison) on the resource.
4. Verify endpoint grouping before anything else.

### Three rules that are not negotiable

1. **Endpoints MUST arrive parametrized.** `http.route` must be the route pattern (`/api/users/:id`), never the concrete URL. Traceway uses the value as-is, but only when it starts with `/`: a route *name* like `app_user_show` is discarded exactly like a missing value. With no usable route it falls back to `url.path`, and the Endpoints page explodes into one row per unique URL.
2. **Background work MUST use `SpanKind.CONSUMER`.** A root span with the default `INTERNAL` kind and no HTTP attributes is silently dropped (exceptions recorded on it still reach Issues, but the task run itself is lost).
3. **The exporter MUST use an OpenTelemetry project's token.** If OTLP arrives with a browser or mobile project's token (framework React, Svelte, Vue.js, jQuery, Flutter, Android, iOS, or React Native), Traceway keeps the Issues and stores the spans as plain spans, but creates no endpoint, task, or AI trace from them. The symptom is "errors appear but the Endpoints page is empty", and the cause is almost always two projects whose tokens got swapped.

### How Traceway classifies spans

The rules are evaluated in this order and the first match wins.

| # | Span | Condition | Becomes |
|---|---|---|---|
| 1 | Any span | `SpanKind = INTERNAL` **and** carries `exception.*` attributes | **Issue.** Never promoted, even with HTTP or `gen_ai.*` attributes present. A child span keeps its ordinary Span row, a root span gets only the Issue |
| 2 | Root span, or a span whose parent is not in the same export batch | `SpanKind = SERVER` or `INTERNAL` with HTTP attributes | **Endpoint** |
| 3 | Any span | `SpanKind = CONSUMER` | **Task** |
| 4 | Root span | `SpanKind = INTERNAL` with a `console.command` attribute | **Task** (CLI command) |
| 5 | Any span | Has any `gen_ai.*` attribute | **AI Trace** |
| 6 | Non-root span | Has a parent span id | **Span** (child) |
| 7 | Root span | Nothing above matched | **Dropped** (exceptions recorded on it still become Issues, unlinked) |

Because the order is fixed, a `CONSUMER` span carrying `gen_ai.*` is a Task, and a `SERVER` request span carrying `gen_ai.*` is an Endpoint. Put the model call on its own child span (Step 6).

Two more rules that do not depend on span kind:

- An `exception` event, or `exception.*` attributes, on any span produces an **Issue**, whatever the span itself became.
- An endpoint that returns **404** with no real route matched (`http.route` missing, or a catch-all like `/`, `/*`) is renamed to the literal endpoint `UNMATCHED`, so bot scans and typo'd URLs collapse into one row. A concrete matched route returning 404 keeps its own name.

For the exact rules, endpoint naming, metric conversion, and the remaining quirks, read `data-model.md` in this skill directory. It is the authoritative reference.

### Exporter configuration

Where the SDK supports the standard env vars, prefer them; they work identically across languages:

```bash
OTEL_SERVICE_NAME=my-service
OTEL_RESOURCE_ATTRIBUTES=service.version=1.2.3   # there is no OTEL_SERVICE_VERSION variable
OTEL_EXPORTER_OTLP_ENDPOINT=https://<instance>/api/otel
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf        # required, several SDKs default to gRPC
OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer <project-token>"
OTEL_TRACES_EXPORTER=otlp
OTEL_METRICS_EXPORTER=none                       # off by default, read "Metrics" below before enabling
OTEL_LOGS_EXPORTER=otlp
```

Two of those lines are load-bearing in a way that fails silently:

- **`OTEL_EXPORTER_OTLP_PROTOCOL` is not optional.** Left unset, the Python SDK and the Java agent resolve `otlp` to **gRPC**, and Traceway has no gRPC listener. The app starts, serves traffic, exits 0, prints no warning, and every export is lost. (PHP is the one exception to the value: it needs `http/json`.)
- **There is no `OTEL_SERVICE_VERSION`.** `service.version` only reaches Traceway through `OTEL_RESOURCE_ATTRIBUTES`. Without it, every endpoint row comes back with an empty App Version and release comparison does nothing.

SDKs append `/v1/traces`, `/v1/metrics`, `/v1/logs` to the endpoint automatically, so set the **base** URL only. A full signal path in `OTEL_EXPORTER_OTLP_ENDPOINT` produces `/v1/traces/v1/traces`, and the signal-specific variables (`OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`) are used verbatim with nothing appended. When configuring in code instead, the full URLs are `https://<instance>/api/otel/v1/traces` (and `/v1/metrics`, `/v1/logs`) with header `Authorization: Bearer <project-token>`.

When Step 3's apply script wrote the env files, the plan's variables already hold real values locally, so wire the exporter to exactly those names (e.g. `Authorization=Bearer ${TRACEWAY_BACKEND_TOKEN}`) and run live verification immediately. Off Traceway Cloud the script also wrote the server URL under the plan's `urlEnvVar` (default `TRACEWAY_BACKEND_URL` for `TRACEWAY_BACKEND_TOKEN`; set `urlEnvVar: "TRACEWAY_URL"` in the plan when the exporter code reads that name). On Cloud it writes no URL, so add `TRACEWAY_URL=https://cloud.tracewayapp.com` yourself where the exporter code reads one; it is not a secret. On the manual fallback the user populates the variables first.

Constraints: OTLP/HTTP only (protobuf or JSON). OTLP/gRPC is not supported, there is no listener on port 4317. `Content-Encoding: gzip` is fine. The body is capped at 10 MB after decompression, and an oversized batch is rejected outright with `413` and `{"error":"request body exceeds the 10MB limit"}`. Nothing is ingested partially. A wrong or missing token answers `401`, and any other path answers `404`. All three are invisible from the application: most SDKs log exporter failures at debug level only, so when nothing arrives, turn the SDK's own diagnostic logging on first.

### Node.js example

```bash
npm install @opentelemetry/sdk-node @opentelemetry/auto-instrumentations-node \
  @opentelemetry/instrumentation @opentelemetry/api @opentelemetry/exporter-trace-otlp-http
```

Declare `@opentelemetry/api` yourself even though it also arrives transitively. Relying on hoisting breaks under pnpm's strict `node_modules` and under Yarn PnP.

Create `instrumentation.mjs` at the project root and load it before the app: `node --import ./instrumentation.mjs server.js`. Use the `.mjs` extension. A `.js` file holding `import` syntax fails to load in a CommonJS project, which is what most projects still are.

```javascript
import { register } from "node:module";
register("@opentelemetry/instrumentation/hook.mjs", import.meta.url);

import { NodeSDK } from "@opentelemetry/sdk-node";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { getNodeAutoInstrumentations } from "@opentelemetry/auto-instrumentations-node";

const url = process.env.TRACEWAY_URL;
const headers = { Authorization: `Bearer ${process.env.TRACEWAY_BACKEND_TOKEN}` };

const sdk = new NodeSDK({
  serviceName: process.env.OTEL_SERVICE_NAME ?? "my-service",
  traceExporter: new OTLPTraceExporter({ url: `${url}/api/otel/v1/traces`, headers }),
  instrumentations: [getNodeAutoInstrumentations()],
});

sdk.start();
```

Three parts of that snippet are load-bearing:

- **The two `register` lines install the ESM loader hook.** Without them an ESM app (`"type": "module"`, `import express from "express"`) still gets HTTP spans but no `http.route`, so every URL becomes its own endpoint row. A CommonJS app (`require("express")`) is patched without the hook, and the lines are harmless there, so keep them either way.
- **`serviceName`** becomes the Server Name in Traceway. Leave it out and every span reports `unknown_service:node`. For release comparison also set `OTEL_RESOURCE_ATTRIBUTES=service.version=1.2.3`.
- **There is no metric reader on purpose.** The HTTP instrumentation records per-route histograms that duplicate the endpoint data and cost far more than the traces (see "Metrics" below). Keep `OTEL_METRICS_EXPORTER=none` from the env block so NodeSDK does not add a metric reader from the environment either. If the app has its own metrics to send, follow "Metrics": pass `metricReaders` (a list; the singular `metricReader` is deprecated) and drop the HTTP histograms with `views`.

Auto-instrumentation covers Express routes (sets `http.route`), status codes, errors, and database clients (`pg`, `mysql2`, `mongodb`, `ioredis`). SQLite and custom business logic need manual `tracer.startActiveSpan()` child spans.

#### Next.js server exception

Next.js must not use the generic `instrumentation.mjs` preload above, `node --import`, or `NODE_OPTIONS`. Next.js creates its own incoming request root span with the matched `http.route`; preloading the generic Node HTTP instrumentation creates a competing root named from the literal URL, so `/api/users/1` and `/api/users/2` become separate endpoints.

For a Next.js server, create a minimal `instrumentation.ts` hook and put the SDK in a separate `instrumentation.node.ts`. Next.js compiles the hook for both Node and Edge; importing OTel packages directly in the hook, even with dynamic `import()` calls after a runtime guard, can make the development Edge compilation resolve Node built-ins and fail. The wrapper must conditionally import only the Node module:

```typescript
export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    await import("./instrumentation.node");
  }
}
```

Keep every Node-only import and the SDK startup in the sibling module:

```typescript
import { getNodeAutoInstrumentations } from "@opentelemetry/auto-instrumentations-node";
import { OTLPLogExporter } from "@opentelemetry/exporter-logs-otlp-http";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { resourceFromAttributes } from "@opentelemetry/resources";
import { BatchLogRecordProcessor } from "@opentelemetry/sdk-logs";
import { NodeSDK } from "@opentelemetry/sdk-node";

(Error as unknown as { prepareStackTrace?: unknown }).prepareStackTrace = undefined;

const instance = process.env.TRACEWAY_URL!.replace(/\/+$/, "");
const base = `${instance}/api/otel`;
const headers = { Authorization: `Bearer ${process.env.TRACEWAY_BACKEND_TOKEN}` };

new NodeSDK({
  resource: resourceFromAttributes({
    "service.name": process.env.OTEL_SERVICE_NAME ?? "nextjs-server",
    "service.version": process.env.APP_VERSION ?? "development",
  }),
  traceExporter: new OTLPTraceExporter({ url: `${base}/v1/traces`, headers }),
  logRecordProcessors: [
    new BatchLogRecordProcessor({
      exporter: new OTLPLogExporter({ url: `${base}/v1/logs`, headers }),
    }),
  ],
  instrumentations: [getNodeAutoInstrumentations()],
}).start();
```

Install the log exporter and SDK packages in addition to the generic Node dependencies: `@opentelemetry/exporter-logs-otlp-http`, `@opentelemetry/sdk-logs`, `@opentelemetry/api-logs`, and `@opentelemetry/resources`. As in the generic example there is no metric reader; keep `OTEL_METRICS_EXPORTER=none` and see "Metrics" before adding one. Next.js versions before 15 also need `experimental.instrumentationHook: true`. Full setup and verification: https://docs.tracewayapp.com/client/otel/nextjs

Resetting `Error.prepareStackTrace` in `instrumentation.node.ts` lets Node apply source maps instead of leaving server Issues pointed at minified Next chunks. Production must also start with `NODE_OPTIONS=--enable-source-maps`, and the build must emit `.next/server/**/*.js.map`; verify both rather than assuming readable server stacks. If the webpack build omits them, add `experimental: { serverSourceMaps: true }` to `next.config`. Also list `@opentelemetry/auto-instrumentations-node` in `serverExternalPackages` so Next does not bundle the instrumentation registry or warn about its optional transports.

### Per-language notes

- **Go**: use the framework's OTel middleware. `go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin`, `github.com/riandyrn/otelchi` (a community package, and pass `otelchi.WithChiRoutes(r)` or it reports raw URLs), `github.com/gofiber/contrib/otelfiber/v2`. All three set `http.route` from the matched route pattern. Exporter: `otlptracehttp.WithEndpointURL(...)` + `WithHeaders`. Two Go-only traps:
  - **Set the propagator.** Go's global propagator is a no-op by default, so `traceparent` is never sent or read and cross-service traces never link. One line, right after `otel.SetTracerProvider(tp)`:

    ```go
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{}, propagation.Baggage{},
    ))
    ```

  - **stdlib `net/http` does not report its route.** `otelhttp` reads the route from `r.Pattern`, which `ServeMux` fills in only after it has matched, so the usual top-level `otelhttp.NewHandler(mux, "server")` records no `http.route` at all and every URL becomes its own endpoint row. Either wrap each registered handler, which puts the span start inside the mux:

    ```go
    mux.Handle("GET /api/users/{id}", otelhttp.NewHandler(usersHandler, "users"))
    ```

    or keep the single top-level wrap and stamp the route yourself:

    ```go
    // imports: strings, go.opentelemetry.io/otel/trace,
    //          semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
    func withRoute(mux *http.ServeMux) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if _, pattern := mux.Handler(r); pattern != "" {
                if i := strings.IndexByte(pattern, '/'); i >= 0 {
                    route := pattern[i:] // drop the "GET " prefix, http.route must start with "/"
                    trace.SpanFromContext(r.Context()).SetAttributes(semconv.HTTPRoute(route))
                }
            }
            mux.ServeHTTP(w, r)
        })
    }

    handler := otelhttp.NewHandler(withRoute(mux), "server")
    ```

- **Python**: `pip install opentelemetry-distro opentelemetry-exporter-otlp`, then `opentelemetry-bootstrap -a install`, then run the app under `opentelemetry-instrument` with the env vars above (`opentelemetry-instrument uvicorn app:app`, `opentelemetry-instrument gunicorn wsgi:app`, `opentelemetry-instrument python worker.py`). Starting the server without that prefix sends nothing. FastAPI and Flask instrumentation sets `http.route` correctly with zero application code, in the framework's own syntax (`GET /users/{user_id}`, `GET /orders/<order_id>`), and both frameworks already answer `500` on an unhandled exception, so the endpoint status and the Issue line up without extra work. On FastAPI 0.142.0 or newer, the next bullet's built-in telemetry is the default; if you still take this path there, you MUST construct the app as `FastAPI(telemetry={"auto_configure": False})`, or FastAPI adds a second exporter and every span reaches Traceway twice. Three Python-only gotchas:
  - **Logs need two more variables.** `OTEL_LOGS_EXPORTER=otlp` attaches the OTel handler to the **root** logger, whose level defaults to `WARNING`, so every `logger.info(...)` is filtered out before the bridge sees it and only WARN and above reach Traceway. Attaching the handler also replaces the root handler list, so the app's own records stop appearing on stdout. Add both:

    ```bash
    OTEL_PYTHON_LOG_CORRELATION=true   # restores console output, stamps otelTraceID/otelSpanID
    OTEL_PYTHON_LOG_LEVEL=info         # only read alongside the line above
    ```

    `OTEL_PYTHON_LOG_LEVEL` on its own does nothing. Do **not** set `OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED=true`; on current versions the bridge is already on and that variable switches it to the SDK's deprecated handler.
  - **Metrics take a minute** (only when the app exports its own, see "Metrics"). The SDK default for `OTEL_METRIC_EXPORT_INTERVAL` is 60000 ms, so a correct pipeline looks empty while you are checking it. Set `OTEL_METRIC_EXPORT_INTERVAL=10000` during verification only, and put it back afterwards.
  - **Django needs two extra things**: export `DJANGO_SETTINGS_MODULE` or `opentelemetry-instrument` fails to start Django at all, and add the route middleware from the Django guide, because `opentelemetry-instrumentation-django` reports `http.route` as Django's own pattern (`api/users/<int:user_id>/`), which has no leading `/` and is therefore discarded.

  Guides: https://docs.tracewayapp.com/client/otel/python and https://docs.tracewayapp.com/client/otel/django
- **FastAPI 0.142.0 and newer: use FastAPI's built-in telemetry by default** (https://fastapi.tiangolo.com/advanced/opentelemetry/). Read the installed version from the lockfile or `pip show fastapi`. Below 0.142.0 there is no built-in telemetry, so use the generic Python path above; bumping FastAPI just to get it is the user's call. The built-in path needs no agent and no `opentelemetry-instrument` prefix:
  1. Install `fastapi[standard]` (or `fastapi[opentelemetry]` when the app avoids the standard extra). Plain `fastapi` ships only `opentelemetry-api`, so FastAPI logs the warning `FastAPI automatic telemetry configuration failed: Automatic OpenTelemetry export requires fastapi[opentelemetry] or fastapi[standard]...` at startup and exports nothing.
  2. Set the same env block from "Exporter configuration" in the process environment. FastAPI reads it at lifespan startup and builds OTLP/HTTP exporters for traces, metrics and logs; the exporters pick up `OTEL_EXPORTER_OTLP_HEADERS` and the resource picks up `OTEL_SERVICE_NAME` and `OTEL_RESOURCE_ATTRIBUTES`. It only speaks `http/protobuf`. Any other protocol logs `FastAPI automatic telemetry configuration failed` and the app runs with no export.
  3. Start the server as usual (`fastapi run`, `uvicorn app:app`, `gunicorn -k uvicorn.workers.UvicornWorker app:app`).
  4. **Add an exception handler, or no Issues appear.** Built-in telemetry marks the request span `ERROR` with status `500` and `error.type`, but sends the exception only as a log record (`http.server.request.exception`) with no `exception` event on the span. Traceway builds Issues from span exception events, so you get a failed endpoint row and an empty Issues page. Record the exception on the request span:

     ```python
     from fastapi import Request
     from fastapi.responses import JSONResponse
     from opentelemetry import trace
     from opentelemetry.trace import Status, StatusCode

     @app.exception_handler(Exception)
     async def record_unhandled_exception(request: Request, exc: Exception):
         span = trace.get_current_span()
         span.record_exception(exc)
         span.set_status(Status(StatusCode.ERROR, str(exc)))
         return JSONResponse({"detail": "Internal Server Error"}, status_code=500)
     ```

     `HTTPException` and validation errors keep their own handlers, so a 404 or 422 is not recorded as an Issue.

  What it covers: SERVER spans with `http.route` in FastAPI syntax (`GET /users/{user_id}`), status codes, `fastapi.dependencies` / `fastapi.endpoint` / `fastapi.serialization` child spans, `traceparent` propagation, HTTP server metrics (unwanted, see "Metrics": keep `OTEL_METRICS_EXPORTER=none` and confirm in the metric explorer that no `http.server.*` metric arrives after startup), and FastAPI's own exception and validation logs. It does **not** cover database queries, outgoing HTTP calls, or the app's own `logging` calls. If the user wants those, and most apps with a database will, use the generic Python path (`opentelemetry-instrument`) and turn off FastAPI's export: `FastAPI(telemetry={"auto_configure": False})`. Leave `auto_configure` on under `opentelemetry-instrument` and FastAPI adds a second exporter to the agent's provider, so every span is sent twice. (FastAPI does notice the contrib middleware and stops creating its own spans, so you never get two request spans, only duplicate exports.) State that trade-off in the setup plan rather than choosing silently.
- **PHP**: Laravel via `composer require keepsuit/laravel-opentelemetry open-telemetry/exporter-otlp php-http/guzzle7-adapter`; Symfony via `composer require traceway/opentelemetry-symfony open-telemetry/exporter-otlp php-http/guzzle7-adapter` (the stock Symfony auto-instrumentation sets `http.route` to the route NAME, which Traceway discards; see `data-model.md`). PHP does not take `OTEL_PHP_AUTOLOAD_ENABLED` as a plain env var next to the block above. Laravel needs nothing extra, the keepsuit service provider starts the SDK. Symfony starts the SDK through `open_telemetry.sdk.autoload_enabled: true` in `config/packages/open_telemetry.yaml`, or through a real process environment variable set in php-fpm, the Dockerfile, or Apache. Never put `OTEL_PHP_AUTOLOAD_ENABLED` in `.env`: Dotenv reads it after Composer autoload, the bundle then skips starting the SDK, and every signal silently becomes a no-op. PHP also prefers `OTEL_EXPORTER_OTLP_PROTOCOL=http/json`, though `http/protobuf` works too and is only slower without `ext-protobuf`.
  - **Symfony workers: never instrument `messenger:consume`.** The bundle already excludes it. `traces.console.excluded_commands` defaults to `['messenger:consume', 'messenger:consume-messages']`, and that key is a prototyped array node, so setting it in `config/packages/open_telemetry.yaml` REPLACES the default instead of adding to it. If you override it for any reason you MUST re-list both entries. A consumer runs in an endless loop, so its command span never ends, never exports, and swallows every child span for the life of the worker. Excluding the command does not silence the work inside it: the Messenger middleware still emits one CONSUMER span per message, and those are what become Tasks in Traceway. Check what actually resolved with `bin/console debug:config open_telemetry traces.console`.
- **Java / .NET / anything else**: the standard OTel agent or SDK with the env vars above works as-is.
- Full per-framework docs: https://docs.tracewayapp.com/client/otel

### Verify endpoint grouping (do this first)

Hit a parametrized route a few times with different IDs and check the Traceway Endpoints page: you must see ONE row (`GET /api/users/:id`), not one row per ID. If you see raw IDs, the instrumentation is not setting `http.route`; fix that before continuing. Last resort, set it manually in a middleware that knows the matched route pattern (the value must start with `/`, or it is discarded):

```javascript
import { trace } from "@opentelemetry/api";

trace.getActiveSpan()?.setAttribute("http.route", matchedRoutePattern);
```

### Errors

Thrown errors must be recorded as exception events to appear as Issues. Auto-instrumentation handles uncaught errors; for caught-and-handled ones:

```javascript
import { trace, SpanStatusCode } from "@opentelemetry/api";

const span = trace.getActiveSpan();
span?.recordException(error);
span?.setStatus({ code: SpanStatusCode.ERROR, message: error.message });
```

(Go: `span.RecordError(err, trace.WithStackTrace(true))`; the stack trace option is what produces the `exception.stacktrace` attribute.)

**Return HTTP 500 when an exception happens.** A request that throws or panics must respond with `500` and set the span status to `ERROR`. Never let it fall through to a `200`/`2xx`. The common trap is a recover/panic middleware that reports the exception but lets the response writer keep its default `200`: Traceway then records the endpoint transaction as a *success*, so the Endpoints page looks healthy while Issues fills up, and the exception becomes an unlinked island instead of correlating to a failed request (and, with distributed tracing, to the frontend call that triggered it). Always pair "an exception was recorded" with "respond `500` and set span status `ERROR`". Genuine client errors that are NOT bugs (validation `422`, auth `401`, not-found `404`) keep their real 4xx status and should not be recorded as exceptions in the first place.

Go has a sharper version of the same trap. `otelgin` records the status only after `c.Next()` returns, which never happens during a panic unwind, so an unrecovered panic leaves the endpoint row at status **0** even though Gin's own recovery already returned 500 to the client, and the Issue arrives with a title but no stack trace. Register a recovery middleware **after** `otelgin.Middleware` so it runs inside the server span:

```go
// imports: fmt, net/http, go.opentelemetry.io/otel/codes, go.opentelemetry.io/otel/trace
func otelRecovery() gin.HandlerFunc {
    return func(c *gin.Context) {
        defer func() {
            if r := recover(); r != nil {
                err, ok := r.(error)
                if !ok {
                    err = fmt.Errorf("%v", r)
                }
                span := trace.SpanFromContext(c.Request.Context())
                span.RecordError(err, trace.WithStackTrace(true))
                span.SetStatus(codes.Error, err.Error())
                c.AbortWithStatus(http.StatusInternalServerError)
            }
        }()
        c.Next()
    }
}

router.Use(otelgin.Middleware(serviceName))
router.Use(otelRecovery())
```

### Logs

`OTEL_LOGS_EXPORTER=otlp` wires the exporter only. The application's own log calls still need a bridge, or the Logs page stays empty while traces and metrics look perfectly healthy.

- **Node**: add `@opentelemetry/winston-transport`, or the Pino or Bunyan instrumentation, so each record picks up `trace_id` and `span_id`.
- **Python, PHP, Java, .NET**: `opentelemetry-instrument` and the language agents bridge the standard logger automatically. FastAPI's built-in telemetry does not: it exports only FastAPI's own records, so an app whose `logging` output should reach Traceway takes the `opentelemetry-instrument` path.
- **Go**: build an `sdklog.LoggerProvider` with `otlploghttp.WithEndpointURL(...)`, register it with `global.SetLoggerProvider(lp)`, and `defer lp.Shutdown(ctx)` or the last batch is lost on exit.

Emit from the **request** context (`c.Request.Context()` in Gin, `r.Context()` in net/http, the active context in Node). A line emitted from a background context carries no trace id and cannot be opened from the endpoint it belongs to. Traceway reads `severity_text` (or derives it from `severity_number`), the body, `service.name`, and the trace and span ids.

### Metrics

**Backends export no OTel metrics by default.** Keep `OTEL_METRICS_EXPORTER=none` from the env block and, when the SDK is configured in code, create no metric reader or meter provider. Nothing in the Traceway UI needs them:

- Endpoint throughput, status codes and P50/P95/P99 latency are computed from spans.
- Host CPU, memory, disk and network come from the Traceway OTel Agent or the Kubernetes collectors (Step 8).
- An OTel project's runtime metrics (`go.*`, `jvm.*`, `process.runtime.*`) do not fill the built-in CPU and memory charts (`data-model.md`, "Built-in metric names").

Leaving the SDK defaults on is expensive. Web framework instrumentation (`otelgin`, `otelhttp`, `otelchi`, `otelfiber`, Node's HTTP instrumentation, FastAPI's built-in telemetry, the Java agent) records `http.server.request.duration`, `http.server.request.body.size` and `http.server.response.body.size` (or the older `http.server.duration`) as one histogram series per route, method and status code. Three things compound:

1. **The SDK default temporality is cumulative.** Every series the process has seen since it started is re-sent on every export with its running total, whether or not a request arrived. An idle server costs as much as a busy one.
2. **Each series is about 550 bytes** of attributes, bucket counts and bucket boundaries. Usage is billed on the request body after decompression, so gzip does not reduce it.
3. **HTTP metrics overlap endpoint analytics.** Current Traceway retains histogram distributions (and legacy average/count series), but endpoints already provide request latency and throughput. Enable HTTP histograms when a metric-based objective or integration needs them, rather than collecting duplicate signals by default. This backend policy does not apply to browser RUM metrics: keep those enabled for population coverage.

Measured on a production Gin API serving about 1,400 requests a day, exporting every 30 seconds: about 117 histogram series per export, about 187 MB a day, 91% of the project's ingest. That uses up a 1 GB allowance in five days. Without the HTTP histograms the same project sends about 18 MB a day.

Turn metrics on only when the user wants the application's own metrics (business counters, queue depth, cache hit rate), and then do all three of these:

1. **Use delta temporality:** `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE=delta`, which the Go, Python, Node, Java and .NET exporters read. Traceway stores each data point as it arrives, so with cumulative temporality a counter charts its running total and a histogram's `.avg` is the average since the process started. With delta, idle series send nothing. Traceway's own backend exports this way.
2. **Drop the framework's HTTP histograms** with a view on `http.server.*` and the Drop aggregation. Zero-code agents (`opentelemetry-instrument`, the Java agent) have no environment variable for views: build the meter provider in code, or rely on delta temporality alone, which sends HTTP series only for intervals that had traffic.
3. **Keep the SDK's 60-second export interval** in production. Every export repeats each active series, so a 5- or 10-second interval multiplies the volume by 12 or 6.

Go:

```go
// imports: go.opentelemetry.io/otel,
//          go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp,
//          sdkmetric "go.opentelemetry.io/otel/sdk/metric"
exp, err := otlpmetrichttp.New(ctx,
    otlpmetrichttp.WithEndpointURL(base+"/v1/metrics"),
    otlpmetrichttp.WithHeaders(headers),
    otlpmetrichttp.WithTemporalitySelector(sdkmetric.DeltaTemporalitySelector),
)
if err != nil {
    return err
}
mp := sdkmetric.NewMeterProvider(
    sdkmetric.WithResource(res),
    sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)), // 60 s default interval
    sdkmetric.WithView(sdkmetric.NewView(
        sdkmetric.Instrument{Name: "http.server.*"},
        sdkmetric.Stream{Aggregation: sdkmetric.AggregationDrop{}},
    )),
)
otel.SetMeterProvider(mp)
defer mp.Shutdown(ctx)
```

Go's framework middleware reads the global meter provider, so an app that never calls `otel.SetMeterProvider` sends no HTTP metrics at all, and that is the default to keep. The view above covers `otelgin`, `otelhttp`, `otelchi` and `otelfiber` alike. To silence only Gin's middleware, pass `otelgin.WithMeterProvider(noop.NewMeterProvider())` (package `go.opentelemetry.io/otel/metric/noop`).

Node: add `views: [{ instrumentName: "http.server.*", aggregation: { type: AggregationType.DROP } }]` to `NodeSDK` (`AggregationType` comes from `@opentelemetry/sdk-metrics`). Python, with the meter provider built in code: `MeterProvider(metric_readers=[...], views=[View(instrument_name="http.server.*", aggregation=DropAggregation())])` (`View` and `DropAggregation` come from `opentelemetry.sdk.metrics.view`).

### Go uses OpenTelemetry

There is no separate Traceway Go SDK. It was retired, and Go instruments with `opentelemetry-go` exactly like every other backend: `go get go.opentelemetry.io/otel/sdk go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`, then the same env vars as the block above. Set up traces and logs only; do not create a meter provider unless the app has its own metrics (see "Metrics"). Host CPU and memory come from the Traceway OTel Agent (see "Deployment and Server Metrics"), not from the app.

## Step 5: Background Tasks (Boundaries and Labeling)

If "Analyze the Architecture" found background work, instrument it as Tasks. The rules:

- **Boundary: one Task = one execution of a unit of background work.** A whole cron job run is one task. One queue message or job is one task. One CLI command invocation is one task. Per-item work inside a run (each email in a batch, each row in an import) is a child span of the task span, never a separate `CONSUMER` span.
- **Do not double-wrap.** If a library's auto-instrumentation already emits `CONSUMER` spans (Kafka, RabbitMQ, Symfony Messenger consumers), wrapping them again creates duplicate Task entries.
- **Labeling: the span name IS the task name and the grouping key.** Use a stable identifier like `cleanup-expired-sessions` or `process-email-queue`. Never embed job IDs, timestamps, or user IDs in the name; each unique name becomes a separate task group. Dynamic context (job ID, batch size) belongs in span attributes, where it shows on the task detail page without affecting grouping.
- **The kind must be `CONSUMER`.** A root span with the default `SpanKind.INTERNAL` is dropped silently; this is the most common reason "my cron job doesn't show up". CLI commands may alternatively be root `INTERNAL` spans with a `console.command` attribute (Laravel/Symfony console instrumentation does this).

```typescript
import { trace, SpanKind, SpanStatusCode } from "@opentelemetry/api";

const tracer = trace.getTracer("my-app");

async function runScheduledJob() {
  await tracer.startActiveSpan(
    "cleanup-expired-sessions",
    { kind: SpanKind.CONSUMER },
    async (span) => {
      try {
        await doWork();
        span.setStatus({ code: SpanStatusCode.OK });
      } catch (error) {
        const err = error instanceof Error ? error : new Error(String(error));
        span.recordException(err);
        span.setStatus({ code: SpanStatusCode.ERROR, message: err.message });
        throw error;
      } finally {
        span.end();
      }
    }
  );
}
```

(Go: `tracer.Start(ctx, "cleanup-expired-sessions", trace.WithSpanKind(trace.SpanKindConsumer))`.)

## Step 6: AI Traces

If "Analyze the Architecture" found model calls, instrument the model calls by following [ai-usage.md](ai-usage.md) in this skill directory. It is the same procedure a scoped `/traceway-setup track ai use` run follows: inventory every call site, instrument each as an OpenTelemetry GenAI span, verify. In a full setup its Step A is already done (the exporter from Step 4), so start at its Step B.

What that file establishes, in short:

- **One span per model call** (one provider API request = one AI Trace row), kind `CLIENT`, named `{operation} {model}` as the GenAI conventions say, and always a span of its own. A span becomes an **AI Trace** when it carries at least one `gen_ai.*` attribute *and* matched none of the earlier classification rules, so `gen_ai.*` on the HTTP route span is ignored (it is an Endpoint), on a `CONSUMER` span too (it is a Task), and an `INTERNAL` span that records `exception.*` attributes is never promoted. Since calls happen inside a request or task, the span stays linked to its Endpoint or Task by trace ID.
- **Traceway reads most of the GenAI conventions as written and differs in a few places**: content goes on `gen_ai.prompt` / `gen_ai.completion` rather than `gen_ai.input.messages` / `gen_ai.output.messages`, cost is `gen_ai.usage.input_cost` / `.output_cost` and yours to compute, `trace.name` is the grouping name, `user.id` is the customer, and tool or agent wrapper spans must carry no `gen_ai.*` attribute or they count as model calls. The full table is "Where Traceway differs from the spec" in `ai-usage.md`.
- **Token counts are integer attributes and costs are doubles.** A number sent as a string is silently stored as 0, with no error on the export.
- **Streaming**: end the span when the stream finishes, not when the call returns.

**Conversational product?** If "Analyze the Architecture" flagged a chatbot, assistant, or tool-calling agent, also follow `ai-agent.md` in this skill directory. It covers `gen_ai.conversation.id` (multi-turn grouping), what value belongs in `user.id` (a stable end-customer id, never a session id), tool-call payload shapes, and sub-agents. Without those the Users tab stays empty, and the Conversations tab fills with one throwaway single-turn conversation per request, because Traceway falls back to the OTel trace id. That is worse than empty: it looks like the feature is working.

Zero-code path for OpenRouter users: in OpenRouter Settings -> Observability, add an OpenTelemetry Collector destination pointing at `https://<instance>/api/otel/v1/traces` with header `{"Authorization": "Bearer <project-token>"}`. Docs: https://docs.tracewayapp.com/client/openrouter

## Step 7: Frontend and Mobile

Browser and mobile apps use the Traceway SDKs, version **2.1.0** on every platform. They all start the same way: the project token as `token`, and `baseUrl` only for a self-hosted server (the default is `https://cloud.tracewayapp.com`). There is no connection string to build. The start code and the env files are exactly what the project's Connection page shows, so a value copied from there matches; the platform files have them.

Every 2.1.0 SDK is built on OpenTelemetry: it exports logs, request spans and device-aggregated performance metrics over OTLP, uploads replay to `/api/recordings`, and polls OpAMP for the project's **Data Collection** rules (browser, Flutter, Android and iOS projects all have that page). Keep production defaults: errors stream, breadcrumbs and replay stay on the device until an error uploads them. Do not turn on `recordAllSessions` merely to prove setup works. Every SDK is built so that it never takes the host app down: a failure inside it is contained, and the app's own crashes still reach the app's previous handlers.

For browser RUM, read [rum.md](rum.md): version capabilities, release/environment/route dimensions, production collection policy and verification.

### Choose the browser SDK version

Choose from the app's installed packages and the live npm registry, not from SDK versions seen in a source checkout or an assumption about where `latest` points:

- **Read the dist-tags first**: `npm view @tracewayapp/frontend dist-tags --json`, and the same for the selected wrapper (`@tracewayapp/react`, `vue`, `svelte` or `jquery`). Only `latest` decides: a `beta` tag can still name a prerelease older than the stable release, so do not install `@beta` unless the user asks for a prerelease.
- **New integration**: install `2.1.0` (`npm install @tracewayapp/react@2.1.0`, the line the dashboard shows) when the registry has it. `token` and `baseUrl` need 2.1.0 or later. If `latest` is an older 2.x, tell the user; a 2.0.x install starts with the connection string described under "Upgrading from 2.0" in [frontend-js.md](frontend-js.md).
- **An existing integration keeps its major**: a 2.x app updates within `@2` (move a `2.0.0-beta.N` pin to stable 2.x, which RUM metrics need). A 1.x app stays on 1.x (add or update packages with `@1`, because the plain package name installs 2.x) unless the user asks for the upgrade or for something only 2.x has (RUM metrics, Data Collection rules, `traceparent` trace linking, replay images and masks). That is a migration, not a version bump: https://docs.tracewayapp.com/client/js-sdk/migration. Never mix 1.x and 2.x packages, never substitute a local checkout.
- **`latest` is still 1.x, or differs between the packages you need** (a registry mirror or proxy that lags): that is an answer from the registry, not a failure. Check `npm view @tracewayapp/frontend@2 version` and the same for the wrapper; `E404` means that registry has no stable 2.x. Tell the user; a new integration then waits for the registry or, if the user agrees, installs 1.x with `@1`.
- **The command itself fails** (network, proxy or authentication error): retry once. If it still fails, say that the SDK version could not be verified, keep an existing integration on its current major, and for a new integration ask the user before choosing.

### Browser

React / Vue / Svelte / jQuery / plain JS, three pieces, all expected:

1. **SDK**: install the registry-verified version of `@tracewayapp/react` (or `vue`, `svelte`, `jquery`, `frontend` for plain JS) and start it with the token from the env file: React `<TracewayProvider token={import.meta.env.VITE_TRACEWAY_TOKEN}>`, Vue `createTracewayPlugin({ token })`, SvelteKit `setupTraceway({ token: PUBLIC_TRACEWAY_TOKEN })`, jQuery and plain JS `init({ token })`. It captures errors, sessions, breadcrumbs, request spans with `traceparent`, web vitals and other performance metrics, screen context, and error-triggered session replay.
2. **Bundler plugin**: `npm install -D @tracewayapp/bundler-plugin`, then add `tracewayDebugIds()` from `@tracewayapp/bundler-plugin/vite` (or `/rollup`, or `TracewayDebugIdsWebpackPlugin` from `/webpack`) to the bundler config, with source maps enabled (`build.sourcemap: true` / `devtool: "source-map"`).
3. **Source map upload**: `npm install -D @tracewayapp/sourcemap-upload`, then run `traceway-sourcemaps --token "$TRACEWAY_SOURCEMAP_TOKEN" --directory ./dist` as a postbuild or CI step (add `--url <instance>` for a self-hosted server; Traceway Cloud is the default since 2.1.0). A postbuild that reads the token from the file the setup wrote loads it first (`frontend-js.md` has the line). The upload token is the browser project's, a CI secret, never committed and never given a public prefix.

For the per-framework start code, env files, options, masks and replay images, sending telemetry to another OpenTelemetry provider, and the full debug-ID and source map pipeline, read [frontend-js.md](frontend-js.md). Online docs: https://docs.tracewayapp.com/client/react (or `vue`, `svelte`, `jquery`, `js-sdk`).

**Full-stack JS** (Next.js, SvelteKit, Remix): only when "Analyze the Architecture" confirmed the framework's server actually serves the app in production. A frontend-only deployment gets just the browser pieces above; a separate backend follows "Backend OTel Setup". When it is genuinely full-stack, integrate both sides under the two confirmed projects: server side follows "Backend OTel Setup" with the backend project's token, and browser side follows the three pieces above with the frontend project's token. For Next.js specifically, use the `instrumentation.ts` framework hook in the "Next.js server exception" above; never apply the generic Node preload to it.

For the Next.js **browser** build, `@tracewayapp/bundler-plugin` currently has a webpack entry point, not a Turbopack one. Set `productionBrowserSourceMaps: true`, add `TracewayDebugIdsWebpackPlugin` only when `!isServer && !dev` (the plugin is production-only and must not replace Next's development source-map mode), and make the production build run `next build --webpack` (Next.js 16 defaults to Turbopack, which otherwise ignores this plugin). After building, confirm client `.js` files contain `//# debugId=` and their `.js.map` siblings contain `debugId`, then upload `.next/static/chunks` with the React project's source-map upload token.

### Mobile

Always the platform SDK, never OTel. Read the lockfile or build files for the installed version first; an existing app keeps its major unless the user asks to move.

- **Flutter**: `flutter pub add 'traceway:^2.1.0'`, then `Traceway.run(token: const String.fromEnvironment('TRACEWAY_TOKEN'), options: TracewayOptions(...), child: MyApp())`, built with `flutter run --dart-define-from-file=.env`. Add `Traceway.navigatorObserver` to `MaterialApp(navigatorObservers: …)` (or go_router's `observers`) for screen names. It captures Dart errors (native crashes need the native Android or iOS SDK in the host apps), logs from `print`, `package:logging` and `Traceway.log`, request spans, sessions, `app.*` performance metrics (the project gets the **Flutter Performance** dashboard) and wireframe replay on every platform; video replay is the separate `traceway_video` add-on. Obfuscated release builds (`--obfuscate --split-debug-info`) need their `.symbols` uploaded with `dart run traceway:upload_symbols`. The older 1.x generation stays on 1.x unless the user asks. Read [flutter.md](flutter.md). Docs: https://docs.tracewayapp.com/client/flutter
- **Android** (native Kotlin/Java): `implementation("com.tracewayapp:traceway:2.1.0")` from Maven Central (core library desugaring when `minSdk` is below 26), then `Traceway.init(this, TracewayOptions(token = BuildConfig.TRACEWAY_TOKEN, screenCapture = true))` in `Application.onCreate()`, with the token in `local.properties` and a `buildConfigField`. It captures uncaught exceptions, ANRs (`ApplicationNotResponding`) and native crashes (`NativeCrash`, API 30+) from the exit reasons, app hangs (`AppHang`), `System.out`/`System.err` and `Traceway.log` lines, OkHttp requests through `TracewayOkHttpInterceptor`, screens and taps, sessions, `app.*` performance metrics (the **Mobile Performance** dashboard) and a wireframe replay of views and Jetpack Compose screens. Apply the `com.tracewayapp.symbols` Gradle plugin (`version "2.1.0"`): it uploads the R8 `mapping.txt` of release builds with the upload token and captures `android.util.Log` (and Timber) lines. Read [android.md](android.md). Docs: https://docs.tracewayapp.com/client/android
- **iOS / Swift** (native SwiftUI or UIKit): add `https://github.com/tracewayapp/traceway-ios.git` with Swift Package Manager, `from: "2.1.0"` (Xcode 26.4 or later), then `Traceway.initialize(token: …, options: TracewayOptions(version: "1.0.0", screenCapture: true))` as early as possible, with the token from an untracked `Traceway.xcconfig` (or the app's existing gitignored secrets `.xcconfig`) through an `Info.plist` key. An `.xcodeproj` app without Xcode at hand gets the package through its project generator or the `project.pbxproj` entries in `ios.md`. A SwiftUI app names its screens with `.tracewayScreen("/path")` (it records none by itself) and its tappable controls with `.tracewayIdentifier("pay")`. It captures uncaught `NSException`s and fatal signals (uploaded on the next launch), MetricKit crashes and watchdog terminations, app hangs (`AppHang`), `NSLog`/`os_log`/`Logger` lines (and `print` with `captureStdout`), requests, screens, taps, sessions, `app.*` performance metrics (the **Mobile Performance** dashboard) and a wireframe replay. Release crashes arrive as bare addresses until the build's dSYMs are uploaded with the upload token. The older 0.1.x generation (`Traceway.start`) stays as it is unless the user asks. Read [ios.md](ios.md). An Objective-C app uses the same SDK through one Swift bridge file (`ios.md`, "Objective-C apps"). **If the app is not a native Swift or Objective-C app** (a cross-platform stack with no Traceway mobile SDK, or a team standardized on OpenTelemetry), there is no native SDK: use an OpenTelemetry library exporting to `<instance>/api/otel` with an `Authorization: Bearer <project-token>` header, exactly like a backend; `ios.md` has that path too. Docs: https://docs.tracewayapp.com/client/ios
- **React Native**: `npm install @tracewayapp/react-native`, wrap the app in `TracewayProvider`. It keeps its own API and the older report transport (no OTLP, no Data Collection rules, no replay); its Connection page shows its own snippet. Docs: https://docs.tracewayapp.com/client/react-native

**A local or plain-HTTP server while testing a mobile app**: `traceway setup` writes the server's own address (for a local server often `http://localhost:8082`; `traceway setup apply` then prints a `Note:` about it on stderr, and the curl path does not, so check the URL yourself), which the host-side upload tools (Gradle plugin, dSYM script, `traceway:upload_symbols`) need as it is. The app needs an address its device can reach: the iOS Simulator reaches the Mac's `localhost` directly; an Android emulator or a USB device does too after `adb reverse tcp:8082 tcp:8082` (otherwise the emulator reaches the host as `10.0.2.2`); a phone on Wi-Fi needs the host's LAN address. Plain `http://` is blocked by default: Android 9+ needs a debug network security config that permits cleartext to that host (`android.md`), and iOS needs `NSAppTransportSecurity` › `NSAllowsLocalNetworking` in the app's `Info.plist` (`ios.md`). Keep those exceptions to debug builds, change only the app's value when it must differ, and tell the user. Traceway Cloud and an HTTPS self-hosted server need none of this.

Every mobile file has "Send telemetry to another OpenTelemetry provider": the `telemetry` option sends logs, traces and metrics to any OTLP/HTTP receiver while errors, replay and Data Collection stay on Traceway. Set it up only when the user asks.

## Step 8: Deployment and Server Metrics

Use the deployment and host-metrics choice confirmed in "Propose and Confirm the Project Map". If it remains unresolved, ask before changing deployment configuration:

1. **How is this project deployed?** Docker on a VM / directly on a VM or bare metal / Kubernetes / serverless or PaaS.
2. **Do you want server (host) metrics tracked in Traceway?** CPU, memory, disk, filesystem, network of the machine running the app.

| Deployment | Wants host metrics | What to do |
|---|---|---|
| Docker on a VM, or directly on a VM/host | Yes | Install the **Traceway OTel Agent** on the host (below). For Docker deploys this is the default; the agent goes on the host, not in a container. |
| Kubernetes | Any | Not the agent, which is a host service. Deploy the two collector workloads in `kubernetes.md` in this skill directory: a DaemonSet for node metrics, pod metrics and container logs, and a one-replica Deployment for cluster state and events. In-process app metrics, when the app has its own, flow via the OTLP metrics exporter from "Backend OTel Setup" (see "Metrics" there). |
| Serverless / PaaS | Any | No host to install on; skip. |
| Anything | No | Skip. |

On Kubernetes, stop here and read `kubernetes.md` in this skill directory. The four things that decide whether it works (`service.name` per node, the three `*.utilization` metrics that ship disabled, `k8s.cluster.name`, and `root_path: /hostfs`) are all easy to miss, and getting any of them wrong produces a plausible-looking config that reports nothing useful.

The agent is a tiny pre-configured OTel Collector that scrapes host metrics every 60s. It MUST use the same backend project token as the API, workers, tasks, and AI traces. Install it on the host (Linux systemd / macOS launchd; PowerShell installer exists for Windows):

```bash
curl -fsSL https://install.tracewayapp.com/install.sh | \
  TRACEWAY_TOKEN=<project-token> \
  TRACEWAY_ENDPOINT=https://<instance>/api/otel \
  TRACEWAY_SERVICE_NAME=<host-label, e.g. api-prod-eu-1> \
  bash
```

- `TRACEWAY_ENDPOINT` ends in `/api/otel` and is required for self-hosted instances; omit only for Traceway Cloud.
- Optional: `TRACEWAY_LOG_PATHS` (comma-separated globs to tail as logs), `TRACEWAY_PROCESS_NAMES` (per-process metrics).
- Re-running the installer upgrades in place, so the command is safe to keep in provisioning scripts.
- If the repo has host provisioning or deploy scripts (cloud-init, Ansible, Terraform `user_data`, `deploy.sh`), add the command there with the token referenced from a secret. Otherwise hand the operator the filled-in one-liner; do not modify the repo.

Metrics arrive within ~60s under their hostmetrics names (`system.cpu.utilization`, `system.memory.usage`, ...). The host is identified by the `server_name` tag, which comes from `TRACEWAY_SERVICE_NAME`; resource metadata such as `host.name`, `host.id`, `host.arch`, `os.type`, and cloud region is also retained, but it does not replace that canonical identity. Give every host a distinct `TRACEWAY_SERVICE_NAME` or the hosts cannot be told apart. The organization overview recognizes these hostmetrics directly, but the legacy Go SDK CPU/memory widgets still read their exact SDK metric names. Install the **OTelemetry Server Agent** dashboard template for the full set: open the command palette with Cmd K, search for it, and it installs a dashboard already wired to `system.cpu.utilization`, `system.memory.*`, `system.filesystem.*`, `system.disk.*` and `system.network.*`. Custom widgets cover anything the template misses. Agent repo: https://github.com/tracewayapp/traceway-otel-agent

## Step 9: Verify

Every dashboard page named below is at `<instance>/<page>`: `/endpoints`, `/issues`, `/tasks`, `/ai-traces`, `/logs`, `/sessions`, `/dashboards`. Set the time picker to the last 15 minutes before reading any of them. Verify each project independently and finish with a project-to-component summary.

When the CLI applied the plan, the local env files hold real values, so run local verification now rather than deferring it. Deployment environments are verified only after the user completes the website's Next Steps panel; re-state which variables must exist there before that verification can pass.

1. **Backend project**
   - Run `curl <app>/api/users/1`, then `/2`, then `/3`, and open `/endpoints`. Exactly one row, `GET /api/users/:id`, with non-zero status codes. Three rows means `http.route` is not being set. Fix that before checking anything else.
   - Two results that look broken but are not: a request to a URL matching no route arrives as `UNMATCHED`, not as the URL you typed; and if the project has "drop healthy healthchecks" enabled, successful `/health` requests are discarded on ingest, so never use a healthcheck route as the smoke test.
   - Trigger a handled or test exception, then open `/issues`. The exception is listed with its endpoint or task context, and that request's endpoint row shows status 500.
   - Open one endpoint row: database queries and outgoing HTTP calls appear as child spans.
   - Run each background job once, then open `/tasks`. One row per job, under a stable name with no ids or timestamps in it.
   - If logs were wired, emit one INFO and one ERROR line inside a request, then open `/logs`. Both carry the same trace id as that endpoint.
   - If "AI Traces" applied, run the checks in Step E of `ai-usage.md`: one call per instrumented feature, then `/ai-traces` lists each feature by name with model, tokens and cost.
   - If the OTel Agent was installed, host metrics reach this same backend project within about 60 seconds, each host under its own `server_name` tag.
   - Run `traceway metrics discover --since 15m`. With metrics off (the default) only the agent's `system.*` names appear. If the app exports its own metrics, they appear too, and no `http.server.*` name is among them; if one is, the view from "Metrics" is missing.
   - If host or Kubernetes metrics were set up, open `/organization`. Every host and node appears as its own instance row with CPU, memory and disk. Kubernetes nodes are grouped by cluster. One row where you expected several means `service.name` is not distinct per machine; empty rings mean the `*.utilization` metrics are not enabled.
2. **Browser project**
   - Verify the installed SDK actually supports the requested RUM features. Set release/environment metadata and normalized route names; generate interactions and a SPA navigation, hide the page to flush metrics, allow the server flush interval, then inspect Web Performance and recorded metric names. Missing INP without an interaction is expected. Metrics and diagnostic replay are separate checks; do not enable full-session recording to make charts populate.
   - Trigger a test browser error and confirm it appears in this project, not the backend project.
   - Confirm the stack resolves to original source files and lines after the matching source maps and bundles are uploaded.
   - If distributed tracing was configured, confirm a browser issue or request links to its backend trace.
   - Open the error's session on `/sessions`: it has the log lines, request spans and a replay clip. A healthy session has metadata and no replay upload.
3. **Each mobile project**
   - Trigger the platform's safe test error or crash flow and confirm it appears only in the matching mobile project. A crash (Android, iOS) is sent on the next launch, and iOS crash tests must run without the debugger attached.
   - Open its session on `/sessions`: the error has its breadcrumbs, and with `screenCapture` on, a wireframe replay clip.
   - Use the app for a minute, send it to the background, then open the **Flutter Performance** (Flutter) or **Mobile Performance** (Android, iOS) dashboard: app start and screen times arrive.
   - For obfuscated Flutter, minified Android, or Release iOS builds, confirm the matching symbols, R8 mapping, or dSYM resolves the production stack.
   - Optional: a temporary Data Collection rule takes effect when the app next polls (at start, on return to the foreground, every 5 minutes); remove the test rule afterwards.
4. **Credential and topology audit**
   - Confirm browser and mobile integrations do not use the backend token, and that the OTLP exporter does not use a browser project's token (rule 3 in Step 4).
   - Confirm APIs, workers, schedulers, AI calls, and host metrics use the backend project unless the user approved an isolation boundary.
   - Report any verification that could not run because a local, deployment, or CI variable is not yet populated.
5. If the `traceway` CLI is installed, run `traceway login`, select the backend project with `traceway projects use <project-id>` (or pass `--project <project-id>` on every call), then:
   ```bash
   traceway endpoints list --since 15m
   traceway exceptions list --since 15m
   traceway logs query --since 15m
   traceway metrics query --name system.cpu.utilization --since 15m
   ```
   Tasks and AI traces have no `list` subcommand, only `show <id>`, so check those two on `/tasks` and `/ai-traces` in the dashboard.
6. **Hand the user a test drive.** After your own verification, finish by telling the user exactly how to see the connection working themselves: two or three copy-paste actions against THEIR app, each paired with the Traceway page it lands on. For example: hit one real parametrized endpoint a few times (`curl <their-app>/api/users/1`, `/2`, `/3` becomes a single row on `/endpoints` within seconds), trigger their app's error path or a deliberate test exception (appears on `/issues` linked to the failing request), and run or wait for one background job (`/tasks`). Use their actual routes and commands, remind them to set the dashboard time picker to the last 15 minutes, and note that approval already dropped them on the right project's dashboard.
