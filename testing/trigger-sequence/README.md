# Browser recording sequences E2E

Runs against disposable local services. The script creates, edits and deletes the rule named **User journey replay** in the embedded demo's **Shoply Storefront** project. Do not point it at shared or production data.

1. Build the sibling `js-client` repository (`npm ci && npm run build`).
2. Build `examples/devtesting-embedded` and run its binary in an empty temporary directory. Its demo API listens on 8082 and seeds the test account/projects.
3. In `frontend`, build and run `npm run preview -- --host 127.0.0.1 --port 5180`.
4. Install the existing Playwright harness: `cd testing/shop/traffic && npm ci && npx playwright install chromium`.
5. From the repository root run `node testing/trigger-sequence/fixture.mjs` (port 5190), then `node testing/trigger-sequence/e2e.mjs ../triggersequence`.
6. Stop the fixture, dashboard preview and embedded binary after testing.

The fixture uses the sibling SDK IIFE by default; `SEQUENCE_SDK_BUNDLE` overrides its path. The runner verifies UI creation/editing/reordering/removal, event order, nested click targets, hover, multiple reasons, full-page recovery, replay DOM playback, URL filters, historical reasons after deletion and dashboard page errors. It writes four real browser screenshots to the output directory. Server repository tests additionally exercise tenant isolation and deduplication on SQLite, DuckDB and ClickHouse.
