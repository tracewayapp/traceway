# Browser SDK v2 setup

Use this reference when the version rules in SKILL.md select v2: stable v2 is `latest` on npm, the app already runs v2, or the user asked for v2 or remote SDK config. Check with `npm view @tracewayapp/frontend dist-tags --json` and the same for the framework package. Today 2.x is published only under the `beta` dist-tag (`2.0.0-beta.N`, while `latest` is 1.x), so `@2` matches nothing and npm answers `E404`; install with `@beta` until `latest` moves to 2.x. When none of those cases applies, use the stable v1 setup in [frontend-js.md](frontend-js.md). Do not use a local SDK checkout.

## Install and initialize

Install the selected v2 package with the repository's package manager: `npm install @tracewayapp/react@beta` (or `frontend`, `vue`, `svelte`, `jquery` as appropriate) while 2.x is in beta, `@2` once `latest` is 2.x. An app that already runs v2 keeps its version range; update it within 2.x only. Keep browser SDK/wrapper majors compatible. Use the published compatible bundler plugin and source-map uploader versions; verify their availability separately rather than assuming all packages publish together. Update the lockfile.

Use a browser project (React, Vue, Svelte, or jQuery) separate from server and mobile projects. For Next.js/Remix, instrument the browser as React and the server independently with OTel. Initialize in browser code, not in SSR/server modules. The connection string is `PROJECT_TOKEN@https://traceway.example.com`; the old `/api/report` suffix is accepted too.

Plain JavaScript:

```js
import { init, captureException, track } from '@tracewayapp/frontend';

init(connectionString, { version: appVersion });

try {
  await checkout();
} catch (error) {
  captureException(error);
}
track('checkout_opened');
```

React:

```jsx
import { TracewayProvider } from '@tracewayapp/react';

<TracewayProvider connectionString={connectionString} options={{ version: appVersion }}>
  <App />
</TracewayProvider>
```

The provider captures and rethrows render errors; use `TracewayErrorBoundary` when a fallback is desired. v2 initialization is idempotent, including repeated initialization in development StrictMode. Do not install a second browser tracer/logger provider alongside it.

Vue uses `app.use(createTracewayPlugin({ connectionString, options: { version: appVersion } }))` from `@tracewayapp/vue`. Svelte uses `setupTraceway({ connectionString, options: { version: appVersion } })` from `@tracewayapp/svelte` during component initialization; in SvelteKit, guard it with `browser` from `$app/environment`. jQuery uses `init(connectionString, options)` from `@tracewayapp/jquery`.

For CDN integration, use `https://cdn.jsdelivr.net/npm/@tracewayapp/frontend@beta/dist/traceway.iife.global.js` (`window.Traceway`) or `https://cdn.jsdelivr.net/npm/@tracewayapp/jquery@beta/dist/traceway-jquery.iife.global.js` (`window.TracewayJQuery`) while 2.x is in beta, or pin the exact version from the `beta` dist-tag (`@2.0.0-beta.N`) so a later beta cannot change the page unannounced. Switch to `@2` once `latest` is 2.x. Do not leave an `@1` CDN script alongside v2 modules.

## Production collection policy

Leave the defaults in place unless the user asks for another policy:

- Errors stream at sample rate 1, subject to existing ignore filters, 1-second identical-error suppression, and a rolling limit of 60 accepted errors per minute per running client. `ignoreErrors: []` removes the ignore filters, not error protection. The next accepted error includes `traceway.error.suppressed_count` when occurrences were suppressed.
- Console (info and above), fetch/XHR, navigation, clicks, and custom events buffer locally. On error, upload up to 10 minutes of buffered breadcrumbs and a 30-second replay clip, with a 5-second clip cooldown and at most 20 clips per session. Replay segments can extend earlier than the lookback to retain a full snapshot.
- Lightweight session metadata and web vitals remain on even for healthy sessions. Resource timing is off. Do not disable sessions to suppress healthy replay uploads.
- Diagnostic uploads share a per-session ceiling of 50 MiB and 10,000 events, including replay events; developers can lower it with `collectionLimits`. Error-only replay still records locally before the error. Use explicit business-event triggers for failures that do not throw.
- Full-session replay is off by default. `recordAllSessions: true` explicitly streams replay and breadcrumbs; do not add it as setup boilerplate.
- Optional `baselineSampleRate: 0.01` streams buffered breadcrumbs and request spans for a stable 1% sample of sessions. It does not sample errors or enable full replay. OpAMP can set the equivalent `sessions.baselineSampleRate`.
- Browser projects expose Data Collection rules and targeted, expiring captures. Backend and mobile projects do not. Do not add OpAMP to those integrations.

Relevant v2 options: `exportDelayMs` (1000 by default), `remoteConfig` (true), `sessions` (true), `sessionRecording` (true: allows error-triggered buffering), `baselineSampleRate` (0), `errorLimits: { maxPerMinute: 60, duplicateWindowMs: 1000 }`, and per-signal ceilings such as `captureNetwork: false`. Remote rules/captures cannot raise a developer-disabled signal. `beforeCapture` receives `{ type, message, stack, attributes }`; returning false drops the event.

Do not carry over v1-only `debounceMs`, `retryDelayMs`, `eventsWindowMs`, `eventsMaxCount`, custom trace-header helpers, or a third `distributedTraceId` argument to `captureExceptionWithAttributes`. Use `captureExceptionWithAttributes(error, attributes)` in v2.

## Transport and correlation

With no `telemetry` option, all requests go to Traceway:

- Logs, exceptions, clicks, navigation, and custom events: OTLP/HTTP JSON to `/api/otel/v1/logs`.
- Fetch/XHR: client spans to `/api/otel/v1/traces`.
- rrweb replay: gzipped JSON to `/api/recordings`.
- Remote settings: OpAMP polling at `/api/otel/v1/opamp`.

Logs and spans carry `session.id`. Requests propagate W3C `traceparent` to the same origin; opt in specific cross-origin API URLs with `propagateTraceHeaderCorsUrls`. Let the backend's standard OTel middleware extract trace context, and allow the trace headers in its CORS configuration. Do not add the v1 `traceway-trace-id` bridge or browser Axios interceptor. A click is a structured log event, not an automatic parent span for a whole interaction; do not promise automatic async exception-to-request linking. Session IDs correlate logs, request spans, and replay.

If the user explicitly wants another OTLP destination:

```js
init(connectionString, {
  telemetry: {
    url: 'https://collector.example.com/otlp',
    headers: { Authorization: browserIngestCredential },
  },
});
```

The SDK appends `/v1/logs` and `/v1/traces`. The receiver needs CORS for the app origin, POST, and supplied headers. Use browser-appropriate ingest credentials, never a privileged server secret. Traceway credentials are not forwarded. Live, restored, and retried telemetry uses this destination; OpAMP, replay, and a copy of session lifecycle metadata stay on Traceway. Application exceptions sent exclusively elsewhere cannot be symbolicated or shown as issues in Traceway. Do not configure this split unless requested.

## Source maps and verification

Use the debug-ID plugin in the production build (`tracewayDebugIds()` from `@tracewayapp/bundler-plugin/vite` or `/rollup`, or `TracewayDebugIdsWebpackPlugin` from `/webpack`). Enable source maps. Upload matching bundle and map files before traffic using `traceway-sourcemaps --url <instance> --token <upload-token> --directory ./dist`, preferably through CI environment variables. Keep the upload token separate from browser ingest credentials. The SDK sends debug IDs with exception logs; Traceway symbolicates before storing the log and creating/grouping the issue.

Verify a production-bundled error resolves to the original source, its log and request spans have a session ID, and its replay opens. A healthy session without full recording enabled should create metadata without uploading a replay. Check a temporary browser Data Collection rule takes effect after the next poll (or `refreshConfig()` for an immediate test), then remove the test rule. `flush()` drains queued exports; it does not force-upload local lookback buffers. Existing clients and historical recording-embedded breadcrumbs still use the legacy `/api/report` path and remain supported.
