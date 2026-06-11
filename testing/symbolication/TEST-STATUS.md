# JS symbolication v2 test plan status

Status as of 2026-06-11. Backend: main @ 9585f64 PLUS uncommitted working-tree changes (exception-span-attribute ingest + CORS on /api/otel routes, see below). ClickHouse + Postgres, SOURCEMAP_CACHE_TYPE=memory, run with -tags pgch.

Uncommitted backend changes that the A-section results depend on:
- backend/app/controllers/otelcontrollers/trace_converter.go: exceptions are now also built from exception.* SPAN attributes (Honeycomb global-errors shape); span events win when both are present; exception-bearing INTERNAL spans are never promoted to endpoints; exception-carrying unpromoted roots emit an exception row but no entity/span rows; exception.stacktrace is stripped from the attribute maps stored on endpoint/task/span rows and on the exception record itself (the structured_stacktrace arrays never reach the maps, extractAttributes skips arrays; exception.type/message are kept as small searchable attrs; the logs path creates no exception records so nothing to strip there)
- backend/app/controllers/routes.go: CORS (preflight + headers) on /api/otel/v1/{traces,metrics,logs}
- backend/app/controllers/clientcontrollers/client.controller.go: hash normalization now strips URL origins (urlOriginRe) before path stripping, so https://host/assets/app.js:1:730, file:///srv/app.mjs and bare app.js frames group identically; required for F4 four-way grouping. HASH-AFFECTING: existing unresolved issues whose frames contain URLs (e.g. d63d2938 in P3, 85516a09 in P2) fork on the next event
- New tests + testdata/honeycomb_global_error.json fixture captured from the real Honeycomb web SDK pipeline
Projects: P1 "React Test" (df92a..., sm token c991270f... generated during A11), P2 "Rect Test SM" (5638c..., upload token 5840...), P3 (2d8ea..., upload token 4872ed...).
[x] = run and confirmed. [ ] = not run, or partially run (see note).

## 0. Environment and prerequisites

- [ ] Backend CH+PG and a second pass on SQLite mode (CH+PG done and used for everything below; SQLite pass descoped for now)
- [ ] One pass with SOURCEMAP_CACHE_TYPE=memory and one with disk for A4, C3, D2 (memory done; disk pass descoped for now)
- [x] Projects with and without source map tokens available (note: P1/P3 gained tokens during testing; a never-token project must be created fresh for branch gating tests)
- [x] Demo frontend app (Vite), throws on click, minified, maps not served publicly (testing/symbolication/vite-app, serve.mjs returns 404 for .map)
- [x] Demo Node app (Rollup bundle) that throws inside a bundled function (testing/symbolication/node-app, terser-minified, mjs + cjs)
- [x] OTel collector binary (contrib v0.154.0) + Honeycomb mock for scenario B (testing/symbolication/otel-collector; real Honeycomb account not used)
- [x] js-client: npm install && npm run build passes across all workspaces including bundler-plugin
- [ ] Go gates: go build ./..., go test ./..., go vet ./app/... (build + full test suite pass with and without -tags pgch; vet fails on a PRE-EXISTING email.service.go IPv6 address-format finding that is present on clean main)
- [x] JS unit gates: full js-client vitest suite, 312 tests / 38 files pass, including debug-id.test.ts, rollup.test.ts, webpack.test.ts, debug-ids.test.ts

## A. Honeycomb frontend lib -> OTel collector -> Traceway

- [x] A1. Exception span captured (live via collector to P3: issue d63d2938 with type+message, traceType "task" confirmed via by-hash API, no `exception` endpoint/task/span rows; unit test + golden fixture from real capture. NOTE: web-vitals spans like TTFB/FCP still become bogus endpoint rows, pre-existing behavior, follow-up under B3)
- [x] A2. Structured stacktrace preferred (golden fixture from the real Honeycomb capture produces canonical fn() + 4-space url:line:col output from the arrays; TestBuildHoneycombStackTrace + TestConvertTraces_ExceptionSpanAttrs_CapturedAsTask assert array preference over the flat string)
- [x] A3. Flat stacktrace fallback (curl payload with structured arrays stripped, only flat V8 exception.stacktrace: detected, canonicalized, and resolved via P3 maps, issue d8fe1450)
- [x] A4. Symbolication end to end via the Honeycomb browser path (upload of honeycomb build maps to P3, re-throw via collector: frames resolved to ../../src/pricing.ts:23:11 with original function names AND grouped into the same issue bc2bc70c as the Traceway-JS-SDK resolved exception, count 3 across three transports)
- [ ] A5. Cross-browser parser matrix. RUN AND FAILED AS SPECIFIED (2026-06-11): all four error types thrown from Chrome, Firefox and WebKit; every format is detected, canonicalized, debugIds attached, symbolicated, BUT each engine forks into its own issue even after resolution. Three independent causes, all surviving symbolication: (1) Firefox appends synthetic `handleEvent*` / `async*` frames; (2) WebKit elides mid-chain frames (computeOrderTotal missing, likely tail calls); (3) column conventions differ per engine (Chrome col 730 resolves to pricing.ts:23:11, WebKit col 743 resolves to 23:15). Cross-TRANSPORT grouping within one engine works (bc2bc70c count 5). Fix options to decide on: drop columns from the hash for frames that resolved to source positions, strip engine-synthetic frames during canonicalization, or both. Bun leg not run (bun not installed)
- [ ] A6. Eval and native frames (run on all three engines: no crashes and adjacent frames always resolve, but two parser bugs confirmed: Chrome `eval at` frames keep a stray `)` in the filename, Firefox keeps `file line 1 > eval` as part of the location; WebKit drops eval frames entirely which is fine; native `Array.forEach` / `[native code]` frames never reach the wire because the SDK drops them client-side, so the spec'd `fn [native code]` rendering is moot on the SDK path)
- [x] A7. Stackless error (curl with the exact no-stack shape GlobalErrors emits for cross-origin errors, type/message only: header-only issue "Error: Script error.", no symbolication crash; real cross-origin browser repro not staged)
- [x] A8. Event + span dedup (curl span with BOTH exception event and exception.* span attrs: exactly one issue, the event's A8EventError; no issue from the span attrs; unit test TestConvertTraces_ExceptionEventAndSpanAttrs_EventWins)
- [x] A9. Exception event on promoted SERVER span still works (Node OTel app; re-verified after the converter change, ingest + symbolication intact)
- [x] A10. Language detection via isJsTelemetry scope-name fallback (curl with telemetry.sdk.language removed, Honeycomb scope name kept: exception symbolicated and grouped with A3's RESOLVED issue rather than the unresolved-URL issue d63d2938, proving the JS path ran; unit test TestConvertTraces_ExceptionSpanAttrs_NoLanguageAttr)
- [ ] A11. Token gating (partial: canonicalized-not-symbolicated confirmed pre-token on P1, hash aefe74ff identical to P2 pre-upload hash; token generation + upload + re-throw flips to resolved without restart; the "NO storage lookups" assertion is not implementable, resolver gates on isJsFramework only)
- [ ] A12. Logs path (exception.stacktrace rewrite + .original, no-token canonicalize-only, non-exception passthrough)
- [x] A13. Direct-to-Traceway, no collector (REQUIRED A FIX: /api/otel routes had no CORS, browser preflight failed; added CORSReport + OPTIONS to the otel group in routes.go; preflight now 204 and the direct browser throw resolved and grouped into bc2bc70c)

## B. Honeycomb-compatible pipeline + Traceway exporter

- [ ] B1. Dual-export parity vs Honeycomb (mock captures exist; no field-by-field gap list)
- [ ] B2. Raw payloads as committed backend test fixtures (partial: one real capture now lives at backend testdata/honeycomb_global_error.json with golden, error span with structured stack + web-vitals span; remaining shapes like page-load trace with child exception not yet collected)
- [ ] B3 follow-up discovered during A1: Honeycomb web-vitals spans (TTFB, FCP, LCP) carry url.path page context and are promoted to bogus endpoint rows by classifySpan; pre-existing on main, needs a classification decision
- [ ] B3. Honeycomb-specific attributes (SampleRate, dataset routing, error bool, markers)
- [ ] B4. Classic/libhoney shape gap doc
- [ ] B5. Sampling interaction

## C. Debug IDs with Node

- [x] C1. Build artifacts (rollup: snippet + comment exactly once per bundle, map debugId matches, deterministic across rebuilds, ID changes on real content change and reverts; a comment-only source change keeps the ID because terser strips it before hashing, correct since the ID derives from emitted content; webpack: real build via TracewayDebugIdsWebpackPlugin in node-app/webpack.config.cjs verified and executed)
- [x] C2. Upload registers debug IDs (vite dist on P3 and node rollup dist on P2: by-debug-id/<id>.js{,.map} stored for both mjs and cjs outputs, .tw generated; post-upload node run resolved and grouped into the original issue b6d6b567; builds.delta metric not watched)
- [x] C3. Resolution via filename matching for Node throws (resolved to ../src/*.js with fulfillOrder() name; OTel-path-has-no-debugIds limitation documented)
- [x] C4. Stale app.debug.source_map_uuid attr ignored (curl payload with the attr on resource AND span: 200, exception resolved via the normal map path and grouped with its peers, the attr is simply stored in the occurrence attributes)
- [x] C5. Node format edge cases (real ECONNREFUSED net error: TCPConnectWrap.afterConnect [as oncomplete] frame canonicalizes intact as TCPConnectWrap.afterConnect() + node:net:1637:16; shebang rollup bundle: snippet injected AFTER the shebang, debugId comment + map field match, executes via node and directly as ./cli.mjs. FINDING: doubled header for Node system errors, OTel JS sets exception.type to err.code ECONNREFUSED while the stack first line says Error:, so the header dedup in formatExceptionStackTrace does not catch it; cosmetic follow-up)
- [x] C6. CJS + ESM (both built, artifact-checked, and executed; app.cjs run ingested to the backend)

## D. Debug IDs with the Vite frontend (regression)

- [x] D1. Baseline filename symbolication without plugin (P1/P2: upload, throw, frames + function names resolve from sibling bundles)
- [x] D2. Plugin path resolves via by-debug-id keys (P3: filename-keyed .map/.tw deleted from storage before re-throw, only by-debug-id/ artifacts present, still resolved, result hash identical to D1)
- [x] D3. Runtime registry pickup (window._tracewayDebugIds populated, error payload carries debugIds {"app-CxX5exYd.js": "bce38108-..."}, captureMessage payload omits the debugIds key entirely)
- [ ] D4. Lazy-loaded chunks, registry cache invalidation (cachedKeyCount; the refresh logic is unit-tested in debug-ids.test.ts, browser repro with a route-split chunk not staged)
- [ ] D5. Sentry interop (partial: SDK-side _sentryDebugIds fallback and traceway-wins precedence are unit-tested in debug-ids.test.ts; building with Sentry's bundler plugin + upload/extraction not run)
- [ ] D7. Concurrent deploys, same filenames, different debug IDs
- [x] D8. Sourcemap validity after injection (resolved hash bc2bc70c identical between plain build and debug-ids build, no mapping drift; raw columns shifted as expected)

## E. js-client uncommitted changes

- [x] E1. Unit tests + exports from consumer project (312 vitest tests pass; tracewayDebugIdsVite, tracewayDebugIdsRollup AND TracewayDebugIdsWebpackPlugin all consumed from real consumer builds)
- [x] E2. getInjectionOffset: shebang, leading comments, directive prologues (unit tests "skips the shebang line" / "skips use strict directives" / "skips leading comments before directives" pass)
- [ ] E3. Idempotency on instrumented code, non-JS assets untouched (no test named for it in the suite, not separately exercised)
- [x] E4. extractDebugIdFromSource precedence (unit tests: runtime marker preferred, trailing comment fallback, undefined when absent; LAST-comment tiebreak and case normalization not explicitly named in tests)
- [x] E5. stringToDebugId UUID corpus (50,003 inputs incl. empty/100KB/random binary: 0 invalid against the strict v4 regex, deterministic)
- [x] E6. Registry-key stacks in Chrome AND Firefox formats (unit tests "maps v8 stack keys" + "maps firefox stack keys" pass; both also verified live in-browser, Firefox payloads carry the correct debugIds map)
- [x] E7. No registries -> debugIds key omitted entirely on the wire (verified on gunzipped /api/report payload, plain build)
- [x] E8. Frontend bundle size check (debug-ids.ts is 2.3KB of source with zero imports; built index.mjs 42.8KB; the bundled SDK chunk stays ~201KB dominated by rrweb, no new heavy imports)
- [ ] E9. debugIds payload against pre-branch backend
- [x] E10. --version removed (CLI help has no version flag, READMEs clean; passing the old flag is SILENTLY IGNORED as an unknown arg, documented here as the decided behavior)
- [x] E11. Sibling bundle upload (maps + sibling bundles: function names resolved, verified throughout; map-only directory: upload reports "4 source map(s) and 0 bundle(s)", next throw resolves locations to src/pricing.ts and groups with the previously resolved issue, no errors)
- [ ] E12. Nested glob / 50MB guard / large dist (nested dist/assets glob worked; 51MB file rejected client-side with "File huge.js.map exceeds 50MB limit (51MB)"; only the large-real-dist total form size remains untested)
- [x] E13. README accuracy (sourcemap-upload README documents exactly --url/--token/--directory matching the CLI; no stale --version references anywhere in packages/)

D6 and E14 were removed from this plan on request (release/publish related, out of scope).

## F. Backend regression sweep

- [ ] F1. Go SDK traces untouched, hash stability vs pre-branch
- [x] F2. Traceway JS SDK canonical traces unchanged (wire payload vs stored row compared byte-identical for a no-maps chunk; existing issues accumulate events rather than forking, shown repeatedly via count increments)
- [x] F3. JVM header dedup, python/ruby never enter JS path (spring_boot_exception golden unchanged through the converter changes; live python Traceback and ruby backtrace via curl stored byte-identical with header prepended, no JS canonicalization)
- [x] F4. Four-way hash stability (TestExceptionHash_FourWayStability: SDK canonical, Chrome raw, Firefox raw, and Honeycomb structured arrays of the same minified error all hash to the documented value ae66d509d719ab7a; REQUIRED A FIX: the normalizer did not strip URL origins, so URL-bearing forms forked from the SDK's bare-filename form; fixed via urlOriginRe in client.controller.go. NOTE this is same-engine stability; cross-ENGINE grouping still forks, see A5)
- [ ] F5. Symbolicator v2 with debug-id keys (partial: .tw generated for by-debug-id/ uploads; re-upload deletes stale filename .tw first and regenerates with new inodes, unrelated builds untouched, verified on disk in memory-cache mode; disk cache mode descoped)
- [ ] F6. Throughput sanity loadgen run
