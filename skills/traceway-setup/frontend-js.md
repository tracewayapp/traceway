# Browser SDK Reference

Browser apps use the Traceway browser SDK, `@tracewayapp/frontend`, directly or through a thin wrapper: `@tracewayapp/react`, `@tracewayapp/vue`, `@tracewayapp/svelte` or `@tracewayapp/jquery`. This file describes 2.1.0. It is built on OpenTelemetry and follows the project's **Data Collection** rules. Read [rum.md](rum.md) for performance measurement and verification. Docs: https://docs.tracewayapp.com/client/js-sdk (and `/client/react`, `/vue`, `/svelte`, `/jquery`).

## Choose the version

- **New integration**: install 2.1.0 (the dashboard's install line pins it: `npm install @tracewayapp/react@2.1.0`). SKILL.md, Step 7, says how to check the npm registry first.
- **An app on 2.x**: update it within 2.x. The start options `token` and `baseUrl` need 2.1.0 or later. An app on 2.0.x starts with a connection string until it is upgraded (see "Upgrading from 2.0").
- **An app on 1.x** (older projects): keep it on 1.x unless the user asks to move. See "Older projects: 1.x" at the end. Moving to 2.x is a migration, not a version bump: https://docs.tracewayapp.com/client/js-sdk/migration.

Read the installed version from the lockfile, not only from the range in `package.json`. Never mix 1.x and 2.x packages, and never substitute a local SDK checkout.

## Install

| Package | Use for |
|---|---|
| `@tracewayapp/frontend` | Plain JS or TS, and frameworks without a wrapper |
| `@tracewayapp/react` | React and Next.js (browser side): provider, error boundary, hooks, route helpers |
| `@tracewayapp/vue` | Vue 3: plugin with a global `errorHandler`, composables |
| `@tracewayapp/svelte` | Svelte and SvelteKit: `setupTraceway()` / `getTraceway()` |
| `@tracewayapp/jquery` | jQuery: also reports failed `$.ajax` calls |

```bash
npm install @tracewayapp/react@2.1.0   # or frontend, vue, svelte, jquery
```

Each wrapper depends on the exact `@tracewayapp/frontend` version it was released with and re-exports its whole API (everything except `init` and the client class). Do not add a separate `@tracewayapp/frontend` dependency next to a wrapper. Use the repository's package manager and update the lockfile.

The Traceway project's framework is `react`, `vuejs`, `svelte` or `jquery`. Next.js and Remix browser apps use `react`. Plain JS has no id of its own: pick the closest one (all four behave the same in Traceway; only the Connection page snippet differs).

CDN, no bundler:

```html
<script src="https://cdn.jsdelivr.net/npm/@tracewayapp/frontend@2.1.0/dist/traceway.iife.global.js"></script>
<script>
  Traceway.init({ token: "YOUR_TOKEN" });
</script>
```

jQuery's CDN build is `https://cdn.jsdelivr.net/npm/@tracewayapp/jquery@2.1.0/dist/traceway-jquery.iife.global.js` (`window.TracewayJQuery`), loaded after jQuery. Pin an exact version so a later release does not change the page unannounced.

## Initialize

Start the SDK once, in browser code, as early as possible. `token` is the project token. The SDK sends to Traceway Cloud (`https://cloud.tracewayapp.com`) by default; pass `baseUrl` only for a self-hosted server. These snippets are the ones the dashboard's Connection page shows.

Plain JS (`init` takes every option in the same object):

```ts
import { init } from "@tracewayapp/frontend";

init({
  token: import.meta.env.VITE_TRACEWAY_TOKEN,
  baseUrl: import.meta.env.VITE_TRACEWAY_URL, // only for a self-hosted server
  version: "1.4.2",
});
```

React (the wrappers take `token` and `baseUrl` next to an `options` object that holds every other option):

```tsx
import { TracewayProvider } from "@tracewayapp/react";

function App() {
  return (
    <TracewayProvider token={import.meta.env.VITE_TRACEWAY_TOKEN} options={{ version: "1.4.2" }}>
      <YourApp />
    </TracewayProvider>
  );
}
```

Self-hosted: add `baseUrl={import.meta.env.VITE_TRACEWAY_URL}`. The provider captures render errors and rethrows them, so the app behaves as without Traceway. `TracewayErrorBoundary` is only for a custom fallback UI inside the provider. A second `init()` (StrictMode, a second provider) is a no-op until `await shutdown()`.

Next.js (App Router), browser side only, with `"use client"`:

```tsx
// app/traceway-provider.tsx
"use client";

import { TracewayProvider } from "@tracewayapp/react";

export function TracewayClientProvider({ children }: { children: React.ReactNode }) {
  return (
    <TracewayProvider token={process.env.NEXT_PUBLIC_TRACEWAY_TOKEN}>
      {children}
    </TracewayProvider>
  );
}
```

Render `<TracewayClientProvider>{children}</TracewayClientProvider>` inside `<body>` in `app/layout.tsx`. Self-hosted: `baseUrl={process.env.NEXT_PUBLIC_TRACEWAY_URL}`.

Vue:

```ts
import { createApp } from "vue";
import { createTracewayPlugin } from "@tracewayapp/vue";
import App from "./App.vue";

const app = createApp(App);

app.use(createTracewayPlugin({
  token: import.meta.env.VITE_TRACEWAY_TOKEN,
  options: { version: "1.4.2" },
}));

app.mount("#app");
```

SvelteKit (root layout, Svelte 5):

```svelte
<!-- src/routes/+layout.svelte -->
<script>
  import { setupTraceway } from "@tracewayapp/svelte";
  import { PUBLIC_TRACEWAY_TOKEN } from "$env/static/public";

  let { children } = $props();

  setupTraceway({
    token: PUBLIC_TRACEWAY_TOKEN,
  });
</script>

{@render children()}
```

Do not wrap `setupTraceway` in a `browser` check: it puts the capture functions into Svelte's context, and it does nothing on the server. Self-hosted: import `PUBLIC_TRACEWAY_URL` too and pass `baseUrl: PUBLIC_TRACEWAY_URL`. `$env/static/public` only exports variables that are defined at build time, so a build fails on the import of one that is missing: define `PUBLIC_TRACEWAY_TOKEN` in every build (CI and preview builds included), and `PUBLIC_TRACEWAY_URL` wherever it is imported. Either may be empty: an empty token leaves the SDK off with one console warning, and an empty URL means Traceway Cloud. A plain Svelte app (Vite) uses `import.meta.env.VITE_TRACEWAY_TOKEN` in its root component. Svelte has no global error boundary: in Svelte 5 pass `onerror={(error) => captureSvelteError(error)}` to `<svelte:boundary>`; SvelteKit `load` and navigation errors are reported from `handleError` in `src/hooks.client.js` with the plain `captureException` export.

jQuery (with a bundler, pass the jQuery you import; a bundled jQuery is not on `window`):

```ts
import $ from "jquery";
import { init } from "@tracewayapp/jquery";

init({
  token: import.meta.env.VITE_TRACEWAY_TOKEN,
  jQuery: $,
});
```

`init()` never throws, in any package. Without a usable token or URL (a preview build without its env variable), it logs one `console.warn` and the SDK stays off; every other function does nothing. SvelteKit is the one place where the build itself needs the variable: a `$env/static/public` import of a name that is not defined fails the build, so define it there even when it is empty. On the server (SSR, no `window`) it does nothing, so the wrappers are safe to render there.

### Where the token comes from

Keep the values in an untracked env file, with the bundler's public prefix. This is what the Connection page and `traceway setup` write:

| App | File | Variables |
|---|---|---|
| Vite (React, Vue, Svelte, jQuery, plain JS) | `.env` | `VITE_TRACEWAY_TOKEN`, and `VITE_TRACEWAY_URL` only for a self-hosted server |
| SvelteKit | `.env` | `PUBLIC_TRACEWAY_TOKEN`, `PUBLIC_TRACEWAY_URL` (read from `$env/static/public`) |
| Next.js | `.env.local` | `NEXT_PUBLIC_TRACEWAY_TOKEN`, `NEXT_PUBLIC_TRACEWAY_URL` |

Other bundlers use their own public prefix (`REACT_APP_` for Create React App). The project token ends up in the shipped bundle by design: it is an ingest credential, not a dashboard login. Still never commit the env file.

The `traceway setup` plan entry for a browser project:

```json
{"name": "Product Web", "framework": "react", "envFile": "web/.env", "envVar": "VITE_TRACEWAY_TOKEN"}
```

The apply step writes `VITE_TRACEWAY_TOKEN`, plus `VITE_TRACEWAY_URL` on a server other than Traceway Cloud (`urlEnvVar` overrides the name). The source map upload token is written only when the plan names `uploadTokenEnvFile` or `uploadTokenEnvVar` (default variable `TRACEWAY_SOURCEMAP_TOKEN`, never with a public prefix), see "Source map upload".

### Upgrading from 2.0

2.0 started with a connection string: `init("<token>@https://host", options)`, `<TracewayProvider connectionString="…">`, `connectionString:` in the Vue plugin and in `setupTraceway`. It still works in 2.1.0, every spelling of it, but it is deprecated and logs one warning that names the new form. Rewrite it as `token` (+ `baseUrl: "https://host"` for a self-hosted server) and rename the env variable to `…_TRACEWAY_TOKEN` (+ `…_TRACEWAY_URL`). An app pinned to 2.0.x keeps its connection string until it is upgraded. The `transport` option is ignored with a warning: the SDK always sends OTLP.

## What is captured automatically

- Uncaught errors (the window's `error` event) and unhandled promise rejections. Failed `$.ajax` calls with `@tracewayapp/jquery`. Vue component, watcher and lifecycle errors with the Vue plugin. React render errors with the provider.
- Sessions: start, end and attributes of every visit.
- Breadcrumbs: `console.*` and `log()` lines (info and above), `fetch`/XHR as OpenTelemetry client spans, navigation, clicks, `recordAction`/`track` events. By default they stay on the device; an error uploads up to the last 10 minutes of them and a 30-second replay clip (5-second cooldown, at most 20 clips per session).
- Performance metrics, aggregated on the device: web vitals, route changes, requests, page views, errors, rage clicks. Uploaded when the page hides, at session end and every 5 minutes. See [rum.md](rum.md).
- Screen context on exceptions, messages, log lines and actions, with the keys of the mobile SDKs: `app.screen.name` (the route template, else the path), `app.screen.route`, `app.screen.previous`, `app.screen.duration_ms`, `viewport`.
- Auto attributes on every session, exception and message: `url`, `path`, `referrer`, `userAgent`, `language`, `platform`, `viewport`, `screen`, `timezone`.

Errors are not sampled. Identical errors within 1 second are suppressed, and one client accepts at most 60 errors a minute; the next accepted error carries `traceway.error.suppressed_count`. The default `ignoreErrors` drops network errors, timeouts and 4xx status errors (`Failed to fetch`, `Load failed`, `/status code 4\d{2}/`, …); `ignoreErrors: []` captures everything.

**The SDK never takes the page down.** A failure in its own code (a public function, a listener, a timer, the `console`/`fetch` wrappers, the replay recorder) is contained, and the page's own call goes on unchanged. The page's own errors still reach the page's handlers. `debug: true` logs contained failures.

## Manual capture

Every wrapper re-exports these functions, so code outside components imports them from the wrapper. They do nothing before `init()` (or before the provider mounts).

```ts
import { captureException, captureMessage, log, setAttribute, recordAction, track } from "@tracewayapp/react";

try {
  await checkout();
} catch (error) {
  captureException(error, { step: "payment" }); // optional string attributes
}
captureMessage("Checkout fell back to card form", { plan: "pro" });
log("cart synced", "debug", { items: 3 }); // trace, debug, info (default), warn, error, fatal
setAttribute("user.id", "u_42"); // on every later event and the session; rules and captures target user.id
recordAction("checkout", "payment_submitted", { amount: 42 });
track("checkout_opened"); // a named event that triggers can match
```

In components: React `useTraceway()`, Vue `useTraceway()`, Svelte `getTraceway()` return `captureException`, `captureMessage` and `recordAction`; outside a provider they return the same functions instead of throwing. Attributes per component: React `<TracewayAttributes>` / `useTracewayAttributes`, Svelte and Vue `useTracewayAttributes`. `captureExceptionWithAttributes(error, attributes)` is the 1.x name of `captureException(error, attributes)`.

- **One user action as a trace**: `startInteraction(name, attributes?)` returns `{ traceId, spanId, isEnded, run, bind, setAttribute, captureException, end }`. Requests made inside `run(callback)` join its trace; the context does not cross an `await`, so wrap each later step in another `run`.
- **Spans of your own**: `tracer()` returns the SDK's OpenTelemetry tracer (`undefined` before `init()`). Its spans carry `session.id` and nest under the active interaction. The SDK registers no OpenTelemetry globals.
- **Route names**: React Router framework mode, Remix, TanStack Router, the Next.js Pages Router, Vue Router and Nuxt need no code. React Router `createBrowserRouter`: `<TracewayProvider router={router}>` or `trackRouter(router)`. React Router `<Routes>` and the Next.js App Router: `useTracewayRoute(pathname, params)` in a layout. SvelteKit: `$effect(() => { page.url.pathname; trackRoute(page.route.id); })` in the root layout. Anything else: `setRoute("/orders/:id")`.
- **Navigation the address does not show** (a hash or memory router, a modal, a wizard step): `recordNavigation(path, action = "push", from?, route?)`.
- **Custom metrics**: `metric(name, value, dims?)`, `count(name, n?, dims?)`, `setMetricDimension(key, value)` (at most two dimensions, never a user id).
- `flush(timeoutMs?)` sends what is queued (it does not release buffered breadcrumbs); `refreshConfig()` fetches Data Collection rules now, for example right after login; `currentSessionId()`, `getInstanceId()`.

## Options

Set them in `init({ … })` (plain JS, jQuery) or in `options` (React, Vue, Svelte). Keep the defaults unless the user asks for another policy.

| Option | Default | What it does |
|---|---|---|
| `token` / `baseUrl` | none / `https://cloud.tracewayapp.com` | Project token; server, only for self-hosted |
| `version`, `environment`, `serviceName` | none, none, the page's host | `service.version` (release comparison, rule targeting), `deployment.environment.name`, `service.name` |
| `attributes` | `{}` | Initial global attributes |
| `debug` | `false` | Logs SDK decisions and contained failures with the page's own `console.warn` |
| `ignoreErrors` / `beforeCapture` | `DEFAULT_IGNORE_PATTERNS` / none | Drop errors by pattern; `beforeCapture({ error, type, message, isMessage, stack, attributes })` returns `false` to drop |
| `errorLimits` | `{ maxPerMinute: 60, duplicateWindowMs: 1000 }` | Per-client error limit and duplicate suppression |
| `remoteConfig` | `true` | Poll Data Collection rules over OpAMP |
| `collection` | SDK defaults | A local collection policy (modes, sampling, triggers, buffer, replay, metric interval) |
| `telemetry` | none | Send logs, traces and metrics to another OpenTelemetry provider |
| `sessions`, `sessionRecording` | `true`, `true` | Report sessions; allow rrweb replay at all |
| `replayImages` | `"always"` | The most a replay records of pictures: `"off"`, `"wifi"` (their URLs) or `"always"` (also small copies). `replay.images` from `collection` or a rule applies only up to it |
| `recordAllSessions` | `false` | Stream every replay segment and breadcrumb. Not setup boilerplate |
| `baselineSampleRate` | `0` | Stream breadcrumbs and request spans for a stable sample of healthy sessions (`0.01` = 1%); no replay |
| `collectionLimits` | `{ maxBytes: 52428800, maxEvents: 10000 }` | Per-session diagnostic upload ceiling; can only be lowered |
| `captureLogs`, `captureNetwork`, `captureNavigation`, `captureUserActions`, `captureWebVitals`, `captureResourceTiming`, `captureLongTasks`, `captureRouteTiming`, `captureMetrics` | `true` | Ceilings: a rule can switch a signal off, never on past `false` |
| `metricDimensions` | none | Up to two dimensions of your own on every metric |
| `detectRoutes` | `true` | Read route templates from the app's router |
| `captureHttpServerErrors` | `false` | Report fetch/XHR responses with status 500 and above as message issues |
| `propagateTraceHeaderCorsUrls` | none | Cross-origin URLs that get the W3C `traceparent` header |
| `exportDelayMs` | `1000` | Delay between log and span exports |

The 1.x options `debounceMs`, `retryDelayMs`, `eventsWindowMs` and `eventsMaxCount` no longer exist; do not carry them over.

## Session replay

rrweb replay runs in every session; by default it stays on the device and only an error uploads a 30-second clip. A session without an error uploads metadata, no replay. Do not turn on `recordAllSessions` to prove that setup works: a Data Collection rule or a capture streams replay for the sessions that need it.

Masks are classes in the markup; no code is needed, and masking happens before upload:

- `class="traceway-mask"`: layout stays; text, text attributes (`title`, `alt`, `aria-*`, `data-*`, `placeholder`, `href`, …) and typed values become `*`; images, video, canvas and SVG become empty boxes without their URL.
- `class="traceway-blank"`: an empty box of the element's size; nothing inside is recorded.
- rrweb's `rr-mask` (text only), `rr-block` (box) and `rr-ignore` (stop an input's events) keep working.
- Input values are masked by default (`maskAllInputs`, set in `collection.replay` or by a rule). With it off only passwords are masked, so put fields that must stay hidden inside `traceway-blank`.
- A mask does not hide a stylesheet `background-image`; put such pictures inside `traceway-blank`.

`replay.images` (in `collection.replay` or a Data Collection rule) decides what the replay shows of pictures outside a mask: `"off"` empty boxes without URLs; `"wifi"` (default) the pictures by their URLs, no pixels uploaded (the browser has no network type check); `"always"` adds a small copy of each picture the page may read (same origin or CORS): PNG at most 192 px for a picture with transparency, else JPEG quality 0.45 at most 384 px, at most 48 KiB each, 128 kept, 4 MiB per session.

## Send telemetry to another OpenTelemetry provider

Only when the user asks for it. `telemetry` sends logs, traces and metrics to any OTLP/HTTP endpoint; with a `token`, errors, replay, captures and Data Collection rules stay on Traceway:

```ts
init({
  token: import.meta.env.VITE_TRACEWAY_TOKEN,
  telemetry: { url: "https://collector.example.com", headers: { "x-api-key": "…" } },
});
```

A shared `url` gets `/v1/logs`, `/v1/traces` and `/v1/metrics` appended; `telemetry.logs`, `.traces` and `.metrics` (`{ url, headers }`) take a complete signal URL for one signal. Ordinary log lines, request spans and metrics then go to the receiver only, so Traceway's session timeline and Web Performance dashboard do not get them. The receiver must accept OTLP/HTTP JSON with CORS for the app's origin, delta sums and exponential histograms. Use a browser-safe credential, never a server secret; the Traceway token is never sent there. Without a `token` the SDK runs standalone: only the `telemetry` destination, no Data Collection rules, no sessions, no replay, and exceptions are plain OTLP logs.

## Source map upload

Production stacks are minified until the build's source maps are uploaded. Three pieces:

1. Make the build emit maps: Vite `build: { sourcemap: true }`, Rollup `output: { sourcemap: true }`, webpack `devtool: "source-map"`. Vite emits none by default, and the uploader then prints `No .map files found` and exits 0.
2. Add debug IDs, so each frame resolves against the map of the exact build: `npm install -D @tracewayapp/bundler-plugin`, then `tracewayDebugIds()` from `@tracewayapp/bundler-plugin/vite` (or `/rollup`), or `new TracewayDebugIdsWebpackPlugin()` from `@tracewayapp/bundler-plugin/webpack` (webpack 5). esbuild is not covered; without debug IDs frames resolve by file name.
3. Upload after each production build: `npm install -D @tracewayapp/sourcemap-upload`, then

```bash
traceway-sourcemaps --token "$TRACEWAY_SOURCEMAP_TOKEN" --directory ./dist
```

The maps go to Traceway Cloud by default; for a self-hosted server add `--url https://traceway.mycompany.com`. `--url` and `--token` can come from `TRACEWAY_URL` and `TRACEWAY_SOURCEMAP_TOKEN`, and a blank value counts as unset. `--url` is optional since 2.1.0 of `@tracewayapp/sourcemap-upload`; an older uploader (1.x, 2.0.x) needs it on Traceway Cloud too (`--url https://cloud.tracewayapp.com`). The uploader does not read env files, and npm does not either, so a postbuild step that relies on the file the setup wrote (`uploadTokenEnvFile`, for example `.env.sourcemaps`) loads it first:

```json
{
  "scripts": {
    "build": "vite build",
    "postbuild": "set -a; . ./.env.sourcemaps; set +a; traceway-sourcemaps --directory ./dist"
  }
}
```

The file also holds `TRACEWAY_URL` off Cloud, so the same line works on a self-hosted server. On CI, set `TRACEWAY_SOURCEMAP_TOKEN` (and `TRACEWAY_URL` off Cloud) as secrets instead of the file; `.` fails when the file is missing, so a CI step calls `traceway-sourcemaps` directly rather than through this postbuild. The upload token comes from the project's **Connection** page (Source Map Upload card). It is a CI secret: never commit it, never give it a public prefix, keep it apart from the project token. A component-specific CI secret name is fine when the step passes it with `--token`. Self-hosted servers need blob storage (S3 or a persistent volume), or uploaded maps disappear when the container is recreated.

Next.js browser build: the bundler plugin has a webpack entry, not a Turbopack one. Set `productionBrowserSourceMaps: true`, add `TracewayDebugIdsWebpackPlugin` only when `!isServer && !dev`, build with `next build --webpack` (Next.js 16 defaults to Turbopack), confirm client `.js` files contain `//# debugId=`, and upload `.next/static/chunks`.

## Verify

- Trigger a test error from a production build (`captureException(new Error("Traceway test"))` behind a button) and open `/issues` in the browser project, not the backend project. The stack resolves to original files and lines.
- Open the session on `/sessions`: the error has its log lines, request spans and a replay clip.
- A healthy session without an error has session metadata and no replay upload.
- Interact with the app, navigate once, hide the tab, wait for the server flush, then open the **Web Performance** dashboard. No interaction means no INP.
- If distributed tracing was set up, a request from the browser links to its backend trace. Same-origin requests carry `traceparent`; cross-origin APIs must be listed in `propagateTraceHeaderCorsUrls` and allow the header in CORS.
- A temporary Data Collection rule takes effect after the next poll (or `refreshConfig()`); remove the test rule afterwards.

## Older projects: 1.x

Recognise 1.x by `@tracewayapp/*` 1.x in the lockfile. It starts with the connection string the project already has: `init(connectionString, options)` (plain JS, jQuery), `<TracewayProvider connectionString={…}>`, `createTracewayPlugin({ connectionString, options })`, `setupTraceway({ connectionString, options })`. Keep that value as it is. Add or update 1.x packages with `@1` (`npm install @tracewayapp/react@1`), because the plain package name installs 2.x. 1.x has no OpenTelemetry export, performance metrics, Data Collection rules or `traceparent` linking, and its options are `debounceMs`, `retryDelayMs`, `eventsWindowMs` and `eventsMaxCount`. Its `traceway-trace-id` header no longer links anything: do not add backend middleware or Axios interceptors for it. The bundler plugin and source map upload above work with 1.x too. Reference: https://docs.tracewayapp.com/client/js-sdk/v1.
