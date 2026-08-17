# CLAUDE.md - Docs

## Navbar

- The header navbar items must always be aligned to the left (next to the logo), not centered or spread across the header. This is achieved via CSS in `styles/custom.css`.
- Navbar order: Docs, Learn, Symbolicator, Protocol, Self Host (controlled by key order in `pages/_meta.json`).

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
