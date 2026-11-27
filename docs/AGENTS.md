# AGENTS.md - Docs

## Navbar

- The header navbar items must always be aligned to the left (next to the logo), not centered or spread across the header. This is achieved via CSS in `styles/custom.css`.
- Navbar order: Docs, Learn, Symbolicator, Self Host (controlled by key order in `pages/_meta.json`). There is no Protocol tab: `/protocol` 301s to `/client/otel`.

## Backend docs live under `/client/otel`

- OpenTelemetry is the one backend integration path. Every OTel-based framework guide is a subfolder of `pages/client/otel/` (`nodejs`, `nestjs`, `nextjs`, `hono`, `cloudflare`, `symfony`, `laravel`, `python`, `django`), listed in `pages/client/otel/_meta.json`. A new backend framework guide goes there — it does **not** get a top-level `/client` entry, an `SDK_OPTIONS` value, or a `SDK_VISIBILITY` key.
- **The `/client` picker carries one backend card.** `components/FrameworkPicker.jsx` lists **OpenTelemetry** (href `/client/otel`) and nothing else for backends, matching the dashboard's project-creation picker, which offers `opentelemetry` as the only backend option. Per-framework backend cards were removed on purpose: the framework guides are reached from `/client/otel`. Do not re-add backend cards.
- **The native Go SDK is gone from the docs.** `client/sdk` and the five `*-middleware` folders were deleted, and `public/_redirects` 301s every one of those URLs to `/client/otel`. Go uses OpenTelemetry like every other backend. Do not re-add Go SDK pages, `go-*` SDK options, or `FOLDER_SDK` entries for them.
- Old flat URLs (`/client/django`, `/client/symfony`, …) are 301'd to the nested paths by `public/_redirects`, which Cloudflare serves from `out/`. Do not delete those lines when adding new ones.

## SDK Selection (path-derived)

- The URL **path is the single source of truth** for the active SDK. There is **no `?sdk=` query parameter** — do not reintroduce one, and never append `?sdk=` to internal `/client` links.
- `resolveSdkFromPath(pathname, lastSdk)` in `components/SdkContext.jsx` maps the first path segment after `/client/` to an SDK via `FOLDER_SDK`. Folders that map 1:1 to a framework (e.g. `react` → `js-react`, `otel` → `otel`) are authoritative.
- One folder is shared/ambiguous (`SHARED_FOLDERS`): `js-sdk` (JS SDK Reference, the JS frontend group). It disambiguates via the remembered framework in `localStorage['traceway-docs-sdk']` (`lastSdk`) when that belongs to the group, otherwise falls back to `js-generic`.
- `localStorage` stores only `lastSdk`. It is written when a specific SDK is resolved from the path, or when the user picks a framework from the dropdown (`SdkSelector`) or the `/client` picker (`FrameworkPicker`).
- Sidebar item visibility (`SDK_VISIBILITY` in `theme.config.jsx`) and the dropdown label both derive from the path-resolved SDK. `SDK_VISIBILITY` must stay complete (every `/client/<folder>` needs an entry) and matching is by exact first path segment, not substring.
- Stray `?sdk=` params on `/client` URLs are self-healed: `SdkProvider` strips them via a shallow `router.replace`, so old bookmarks/links correct themselves.

## Browser SDK 2.x (current) and 1.x (legacy)

- The 2.x browser SDK (OpenTelemetry, remote SDK config over OpAMP) is what npm `latest` installs; the pages describe 2.1.0 and say so in their text. Install commands carry no tag (`npm install @tracewayapp/frontend`); CDN URLs pin the major (`@2`). All active js-client packages share one version. Because the install lines carry no version, the docs describe whatever `latest` is: publish 2.1.0 to npm `latest` before deploying pages that use 2.1.0 API (`token`, `baseUrl`).
- `pages/client/js-sdk/*` documents 2.x, and so do the framework guides (`react`, `vue`, `svelte`, `jquery`) and everything under `pages/learn/`. A 2.x page never links readers to a 1.x page by default: no 1.x callout at the top and no inline `v1` links.
- `pages/client/js-sdk/migration.mdx` (sidebar title **Upgrading**) brings an app to the current version: "Upgrading from 2.0" (token and `baseUrl` instead of the connection string, the removed `/api/report` transport, other 2.1.0 changes), then "Migrating from 1.x", whose "Staying on 1.x" section is the one 2.x link into the 1.x reference. It mirrors the upgrade and migration sections of the js-client README, so update both together.
- `pages/client/js-sdk/v1/*` (sidebar **SDK 1.x (legacy)**, collapsed) is the legacy 1.x reference for the pages that differ between versions (overview, initialization, flush, distributed tracing, session replay). Its install commands and CDN URLs pin `@1`, and it is the only place 1.x connection strings and `/api/report` stay. `exceptions`, `messages`, `sourcemap-upload` and `debug-ids` cover what both versions share (the React Native guide links to `sourcemap-upload` too), so they have no `v1` copy and state any version difference inline.
- `@tracewayapp/react-native` shares the 2.x version number but still uses its own native transport and 1.x-style API (a connection string); do not describe remote config, OTLP or replay for it.

## Connection strings, hosts and versions

- Every 2.1.0 SDK (browser, Flutter, Android, iOS) starts with the project `token`, plus `baseUrl` only for a self-hosted server (default `https://cloud.tracewayapp.com`). Connection strings and `/api/report` appear only in an "Upgrading from 2.0" section (and the iOS "Migrating from 0.1.x" / other 1.x migration sections, the `v1` pages and React Native).
- The host in examples is `cloud.tracewayapp.com`; a self-hosted example is `https://traceway.mycompany.com`. The self-hosting pages (`pages/server/**`) keep their own hosts.
- The upload tools (`traceway-sourcemaps`, `traceway:upload_symbols`, the Android Gradle plugin, the iOS dSYM script) default to Traceway Cloud since 2.1.0; `--url` / `TRACEWAY_URL` is only for a self-hosted server.

## Platform pages and shared partials

- The Flutter, Android and iOS Quick Start pages share one skeleton (the same H2s in the same order; platform-only H3s are allowed) and follow their SDK READMEs. Each platform folder also has `session-capture.mdx` ("Configure Session Capture"), with the same steps on every platform.
- Text the tutorials share lives in partials without headings, so the TOC and anchors stay in the pages: `components/client-docs/session-capture/*.mdx` for the browser pages, `components/mobile-docs/*.mdx` for the mobile pages. Platform specifics are written on the page around each partial.
- A Markdown image (`![](/x.png)`) becomes a webpack import, so the build fails when the file is missing; a screenshot that is referenced before it exists (as the session-capture pages did) uses an `<img>` tag.
