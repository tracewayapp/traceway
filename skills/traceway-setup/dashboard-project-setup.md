# Manual Fallback: Create the Project Structure in the Dashboard UI

Use this guide only when the setup-token + CLI path in SKILL.md Step 3 is unavailable or the user prefers clicking through the UI, and only after "Analyze the Architecture" produced a repository inventory and "Propose and Confirm the Project Map" was confirmed by the user. Tailor every instruction to the detected components; do not hand the user a generic list of every supported platform. On this path ingest tokens never transit chat: the user saves each token straight into their env file or secret store.

## 1. Establish the Dashboard

Ask whether the user uses Traceway Cloud or a self-hosted instance only if the repository and conversation do not establish it.

- Traceway Cloud dashboard: https://cloud.tracewayapp.com
- Traceway Cloud registration: https://cloud.tracewayapp.com/register
- Self-hosted dashboard: `https://<their-instance>`

If the user does not have an account, direct them to registration and wait. Registration has two steps: step 1 creates the account and organization; step 2 offers "AI" and "Manual" project setup. For this manual path, tell the user to pick **Manual** on step 2 and enter the confirmed project rows right there (name plus framework per row, "Add another project" for more); each created project's token is shown for saving under the planned variable name. A user who already registered and skipped step 2 gets the same editor on `<their-instance>/setup` under the Manual tab. Do not ask for dashboard credentials.

## 2. Present the Creation Checklist

Before the user creates anything, show the exact planned structure:

| Order | Project name | Framework to select | Repository component | Signals | Credentials |
|---|---|---|---|---|---|
| 1 | `Product Backend` | OpenTelemetry | API, workers, scheduler, AI calls, backend hosts | Endpoints, tasks, AI traces, issues, logs, application and host metrics | Runtime token |
| 2 | `Product Web` | Detected browser framework | Browser application | Browser issues, web vitals, replay, source-mapped stacks | Runtime token + upload token |
| 3 | `Product Mobile` | Detected mobile framework | Independently released mobile app | Mobile issues/crashes, replay where supported, symbolicated stacks | Runtime token + upload token when symbols are needed |

The picker offers nine frameworks and no more: **OpenTelemetry**; **React**, **Svelte**, **Vue.js**, **jQuery**; **Flutter**, **React Native**, **Android**, **iOS**. Never tell the user to look for Gin, Django, Laravel, Symfony, Hono, Next.js, or Remix. Those entries do not exist, because every backend is OpenTelemetry and the browser half of a meta-framework is React.

Replace the examples with the confirmed names and omit rows that do not apply. For multiple browser or mobile applications, include one row per independently deployed or released application.

Explain the boundary in one paragraph: the backend project intentionally combines APIs, workers, tasks, AI calls, and server metrics so their telemetry and distributed traces stay together; browser and mobile runtimes use separate projects because they use platform SDKs, platform-specific dashboard behavior, and their own build artifacts.

## 3. Create Each Project

Two equivalent UI paths; pick whichever matches where the user is:

**During onboarding** (registration step 2, or `<instance>/setup`): select the **Manual** tab, enter one row per checklist entry (project name plus framework), use "Add another project" for more rows, then select **New Projects**. Save each project's token under the planned environment-variable name.

**From the running dashboard**, for each checklist row:

1. Open the Traceway dashboard.
2. Open the project selector in the header.
3. Select **Add Project**.
4. Choose the organization from the confirmed plan. The picker only appears when the user belongs to more than one organization they can write to; otherwise the organization is shown as fixed text and there is nothing to select.
5. Enter the proposed project name.
6. Select the framework from the checklist.
7. Select **New Project**.
8. Save the project token under the planned environment-variable name.
9. Select **Go to Connection** to review the tailored integration details.

Framework selection rules:

- Always select **OpenTelemetry** for a backend project, regardless of language or web framework. Go, Node, Python, PHP, Java, .NET, and Ruby backends all take this one option; the language and framework are chosen later on the Connection page, which then shows the tailored install and exporter setup.
- Select the browser framework for a browser project: **React**, **Svelte**, **Vue.js**, or **jQuery**. A meta-framework picks the framework it renders with: Next.js and Remix select **React** (Next.js browser setup is `@tracewayapp/react`), SvelteKit selects **Svelte**, Nuxt selects **Vue.js**.
- A browser application built on none of those four (vanilla JS, Angular, Solid, Preact, Astro, Web Components) still gets a **browser** project, not an OpenTelemetry one, because it uses `@tracewayapp/frontend` and the browser ingest path. Pick the closest listed browser value; it only decides which snippet the Connection page renders. Follow the plain-JS setup at https://docs.tracewayapp.com/client/js-sdk instead of that snippet.
- Select **Flutter**, **React Native**, **Android**, or **iOS** for a mobile project.
- A full-stack JS application with a production server gets an OpenTelemetry backend project and a separate browser project.

The backend enforces that last split; it is not just a recommendation. A project whose framework is React, Svelte, Vue.js, jQuery, or React Native discards every endpoint, task, AI trace, and span that arrives over OTLP. The request still answers `200`, so a server exporter pointed at a browser project's token looks healthy while its Endpoints page stays empty forever. Server telemetry must use the OpenTelemetry project's token.

## 4. Plan Credentials Without Exposing Them

Do not ask the user to paste tokens into chat. Ask them to set the values in their secret manager, deployment environment, local untracked environment file, or CI settings and then confirm that the variables exist.

| Credential | Used by | Handling |
|---|---|---|
| Project/runtime token | Backend OTLP exporter | Secret; store in the backend deployment environment. |
| Browser connection string | Browser SDK | Build-time public configuration; never reuse the backend token. Follow the framework's public environment-variable prefix. |
| Mobile connection string | Mobile SDK | Build configuration; never reuse the backend or browser token. |
| Upload token | Frontend source maps and mobile symbols/mappings | CI secret; generate per project from **Connection** -> **Source Maps** or **Symbol Upload**. Never embed it in the application. |

Runtime credentials take explicit component names. Adapt these to the repository's conventions:

```text
TRACEWAY_URL
TRACEWAY_BACKEND_TOKEN
PUBLIC_TRACEWAY_WEB_CONNECTION_STRING
TRACEWAY_MOBILE_CONNECTION_STRING
```

For Vite use a `VITE_` public prefix, for SvelteKit use `PUBLIC_`, and for Next.js use `NEXT_PUBLIC_`.

**Upload tokens do not get invented names.** Each uploader reads one fixed variable, so a component-specific name only works when it is passed through explicitly:

| Uploader | Reads | Also reads |
|---|---|---|
| `traceway-sourcemaps` (JS source maps) | `TRACEWAY_SOURCEMAP_TOKEN` | `TRACEWAY_URL` |
| `dart run traceway:upload_symbols` (Flutter) | `TRACEWAY_UPLOAD_TOKEN` | `TRACEWAY_URL` |
| iOS dSYM upload script | `TRACEWAY_UPLOAD_TOKEN` | `TRACEWAY_URL` |
| `com.tracewayapp.symbols` Gradle plugin (Android R8) | nothing, the plugin has no environment lookup | nothing. Wire both in the `traceway { }` block: `uploadToken = System.getenv("TRACEWAY_UPLOAD_TOKEN") ?: ""` and `url = "https://<instance>"` |

Name the CI secret exactly what the uploader reads, or keep a per-component secret and pass it on the command line (`--token "$TRACEWAY_WEB_UPLOAD_TOKEN"`, or `uploadToken = System.getenv("...")` in Gradle). A mismatch here fails at release time, not at setup time. Upload tokens never use a public prefix and never ship inside the application.

When one repository releases two mobile apps, the two upload tokens collide on `TRACEWAY_UPLOAD_TOKEN`; scope them per job in CI, or pass `--token` explicitly per build.

For an existing token, establish its project name and selected framework from the dashboard before mapping it. A token is opaque; never infer its platform from its value. Reuse it only when the project matches the confirmed row.

## 5. Offer the Relevant Documentation

Include only links that match the detected architecture:

Every backend guide lives under the OpenTelemetry section, so link the overview plus the one language page that matches:

| Integration | Documentation |
|---|---|
| Project structure | https://docs.tracewayapp.com/learn/projects |
| Backend OpenTelemetry (start here) | https://docs.tracewayapp.com/client/otel |
| Node.js | https://docs.tracewayapp.com/client/otel/nodejs |
| NestJS | https://docs.tracewayapp.com/client/otel/nestjs |
| Next.js (server side) | https://docs.tracewayapp.com/client/otel/nextjs |
| Hono | https://docs.tracewayapp.com/client/otel/hono |
| Cloudflare Workers | https://docs.tracewayapp.com/client/otel/cloudflare |
| Symfony | https://docs.tracewayapp.com/client/otel/symfony |
| Laravel | https://docs.tracewayapp.com/client/otel/laravel |
| Django | https://docs.tracewayapp.com/client/otel/django |
| Host metrics agent | https://docs.tracewayapp.com/learn/otel-agent |
| JavaScript source maps | https://docs.tracewayapp.com/client/js-sdk/sourcemap-upload |
| Plain JS and other browser frameworks | https://docs.tracewayapp.com/client/js-sdk |
| React | https://docs.tracewayapp.com/client/react |
| Svelte | https://docs.tracewayapp.com/client/svelte |
| Vue | https://docs.tracewayapp.com/client/vue |
| jQuery | https://docs.tracewayapp.com/client/jquery |
| Flutter | https://docs.tracewayapp.com/client/flutter |
| React Native | https://docs.tracewayapp.com/client/react-native |
| Android | https://docs.tracewayapp.com/client/android |
| iOS | https://docs.tracewayapp.com/client/ios |
| OpenRouter | https://docs.tracewayapp.com/client/openrouter |

## 6. Completion Gate

Before implementation, confirm:

- Every selected component has exactly one matching project.
- The backend project's framework is OpenTelemetry.
- Browser and mobile projects do not reuse the backend token.
- APIs, workers, schedulers, AI calls, and the host agent are mapped to the same backend token unless the user confirmed a real isolation boundary.
- Every frontend or symbolicated mobile build has its own upload token stored as a CI secret, under the name its uploader reads or passed explicitly with `--token`.
- Environment-variable names are agreed, even if live values are not available in the current shell.

Do not block code changes merely because live values are unavailable. Implement configuration through the agreed variables, then tell the user exactly which variables must be populated before live verification.
