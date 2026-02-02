# CLAUDE.md - Docs

## Navbar

- The header navbar items must always be aligned to the left (next to the logo), not centered or spread across the header. This is achieved via CSS in `styles/custom.css`.
- Navbar order: Docs, Learn, Protocol, Self Host (controlled by key order in `pages/_meta.json`).

## SDK Query Parameter

- The `sdk` query parameter (e.g., `?sdk=go-gin`) must be sticky across all `/client` pages. When a user selects a framework, every sidebar link click and page navigation within `/client` must preserve the `sdk` param in the URL.
- This is handled by the `SdkProvider` in `components/SdkContext.jsx` which listens to route changes and re-injects the `sdk` param if missing.
- All internal links within `/client` pages should propagate `sdk` — do not create links that would drop the parameter.
