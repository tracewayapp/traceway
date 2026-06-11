# JS Symbolication v2 test apps

Demo apps for the JS symbolication v2 test plan (Honeycomb ingest, multi-format stack parser, debug IDs, js-client changes). All apps consume the local js-client checkout at `../../../js-client` via `file:` dependencies, so rebuild js-client (`npm run build` at its root) before testing SDK changes.

## Backend and projects

- Backend: `http://localhost:8082` (ClickHouse + Postgres mode), dashboard login `pasa@pasa.pasa` / `pasapasa`
- **P1** "React Test", project token `df92a59366084ae699395cb2789c4bb6`, NO source map token (token-gating scenarios, A11)
- **P2** "Rect Test SM", project id `d41d6273-f234-4d1d-8ce8-bdf3799be379`, project token `5638c2de607f45169bcf98aa8774fe5c`, source map upload token `5840239bbba04aa58e5f96ec130c10bd` (symbolication scenarios)

All apps default to P2. Override per app with the env vars listed below (use the P1 token for gating tests).

## vite-app (sections A, D, E6-E8)

Browser app that throws on button clicks: nested call chain across `src/app.ts` and `src/pricing.ts`, unhandled rejection, eval throw, Array.forEach throw, stackless captureMessage.

```bash
cd vite-app
npm install
npm run build              # minified, hidden source maps, Traceway JS SDK -> /api/report
npm run build:debug-ids    # same + tracewayDebugIdsVite bundler plugin (D2-D8)
npm run build:honeycomb    # HoneycombWebSDK + GlobalErrorsInstrumentation -> collector :4318 (A1-A13)
npm run serve              # serves dist/ on http://localhost:4173, .map requests return 404
npm run upload-maps        # uploads dist/ maps + bundles to P2
npm run dev                # vite dev server on :5174 (unminified, for sanity checks only)
```

Build-time env overrides: `VITE_TW_MODE` (`traceway` | `honeycomb`), `VITE_TW_TOKEN`, `VITE_TW_REPORT_URL`, `VITE_TW_OTLP_ENDPOINT` (point at `http://localhost:8082/api/otel/v1/traces` for the direct, no-collector A13 case; the Authorization header is always sent).

Honeycomb mode emits the exact shape from the plan: zero-duration root span named `exception` from scope `@honeycombio/instrumentation-global-errors` with `exception.*` and `exception.structured_stacktrace.{urls,functions,lines,columns}` attributes on the span. Web vitals spans are also emitted (B1/B3 noise parity).

## node-app (sections C, F)

Rollup-bundled (terser-minified) Node app that throws inside a bundled function chain (`src/index.js` -> `src/order.js` -> `src/inventory.js`) and reports via the OTel Node SDK. Emits both `dist/app.mjs` and `dist/app.cjs` (C6).

```bash
cd node-app
npm install
npm run build              # rollup + terser, hidden maps
npm run build:debug-ids    # same + tracewayDebugIdsRollup (C1, C2)
npm run start              # run ESM bundle, report to /api/otel/v1/traces directly
npm run start:cjs          # run CJS bundle
node dist/app.mjs span     # Honeycomb-style: zero-duration `exception` span with exception.* span attributes (A1 path)
npm run upload-maps        # uploads dist/ maps + bundles to P2
```

Default mode (`event`) emits a SERVER span with HTTP attributes plus an `exception` span event (the pre-existing A9 path). Env overrides: `TW_TOKEN`, `TW_OTLP_URL` (point at `http://localhost:4318/v1/traces` to go through the collector).

Note: unpromoted root spans are dropped by the backend converter, so the SERVER span must keep its `http.*` attributes for the `event` mode to store anything.

## otel-collector (sections A, B)

OTel collector contrib v0.154.0 plus a Honeycomb mock. The pipeline receives OTLP on :4317/:4318 (CORS open for browser export) and triple-exports every batch:

1. `otlphttp/traceway` -> `http://localhost:8082/api/otel` with `Authorization: Bearer $TW_PROJECT_TOKEN` (default: P2 token)
2. `otlphttp/honeycombmock` -> `http://localhost:8090`, JSON encoding; the mock dumps each request to `captures/honeycomb/NNNN_v1_traces.json` (B1 parity diffing, B2 fixtures)
3. `file` -> `captures/otlp-capture.jsonl` (B2 fixtures)

```bash
cd otel-collector
./download.sh              # fetches otelcol-contrib for this OS/arch into bin/ (gitignored)
node honeycomb-mock.mjs    # mock on :8090, run in its own terminal
./run.sh                   # collector; TW_PROJECT_TOKEN env overrides the forwarding token
```

Collector self-telemetry is disabled in `config.yaml` because :8888 is occupied on this machine. To dual-export to real Honeycomb instead of the mock, swap the `otlphttp/honeycombmock` exporter for `endpoint: https://api.honeycomb.io` with an `x-honeycomb-team` header.

## Verified working (2026-06-10, backend at main 9585f64)

- Traceway JS SDK browser path: minified frames ingest, canonicalize, and after `upload-maps` resolve to `src/pricing.ts:line:col` with original function names
- Node OTel `event` mode: V8 frames canonicalize and resolve to `../src/*.js:line:col`, `fulfillOrder()` name resolved; direct and via-collector runs group into the same hash
- Debug-ID builds (vite + rollup, mjs + cjs): injection snippet, trailing `//# debugId=` comment, and matching map `debugId` per chunk
- serve.mjs returns 404 for `.map` so maps are never public
- Honeycomb mode end to end through the collector; mock captures the structured_stacktrace payload

Known gaps (2026-06-11 update):

- Exception-span-attribute ingest (A1/A2/A8) and CORS on /api/otel (A13) are now implemented as uncommitted working-tree changes in the backend; see TEST-STATUS.md for the verification record
- Honeycomb web-vitals spans (TTFB/FCP/LCP) still become bogus endpoint rows via their url.path page-context attribute (pre-existing, B3 follow-up)
- Chrome `eval at` frames canonicalize with a leftover `)`: `app-xxx.js:1:1279), <anonymous>:1:30` (A6)
