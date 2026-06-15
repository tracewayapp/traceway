# CLAUDE.md - Docs

## Navbar

- The header navbar items must always be aligned to the left (next to the logo), not centered or spread across the header. This is achieved via CSS in `styles/custom.css`.
- Navbar order: Docs, Learn, Symbolicator, Protocol, Self Host (controlled by key order in `pages/_meta.json`).

## SDK Selection (path-derived)

- The URL **path is the single source of truth** for the active SDK. There is **no `?sdk=` query parameter** — do not reintroduce one, and never append `?sdk=` to internal `/client` links.
- `resolveSdkFromPath(pathname, lastSdk)` in `components/SdkContext.jsx` maps the first path segment after `/client/` to an SDK via `FOLDER_SDK`. Folders that map 1:1 to a framework (e.g. `gin-middleware` → `go-gin`, `react` → `js-react`) are authoritative.
- Two folders are shared/ambiguous (`SHARED_FOLDERS`): `sdk` (Go SDK Reference, any `go-*`) and `js-sdk` (JS SDK Reference, the JS frontend group). These disambiguate via the remembered framework in `localStorage['traceway-docs-sdk']` (`lastSdk`) when it belongs to the folder's group, otherwise fall back to `go-generic` / `js-generic`.
- `localStorage` stores only `lastSdk`. It is written when a specific SDK is resolved from the path, or when the user picks a framework from the dropdown (`SdkSelector`) or the `/client` picker (`FrameworkPicker`).
- Sidebar item visibility (`SDK_VISIBILITY` in `theme.config.jsx`) and the dropdown label both derive from the path-resolved SDK. `SDK_VISIBILITY` must stay complete (every `/client/<folder>` needs an entry) and matching is by exact first path segment, not substring.
- Stray `?sdk=` params on `/client` URLs are self-healed: `SdkProvider` strips them via a shallow `router.replace`, so old bookmarks/links correct themselves.
