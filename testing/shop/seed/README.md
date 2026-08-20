# seed.mjs — demo account entity seeding

Creates the platform entities for the Shoply demo on a Traceway instance:

- Monitors (synthetic checks) on the backend project: `Traceway API`, `Marketing Site`,
  `Edge TLS` (tcp), and the intentionally failing `Payments Provider` (asserts a body
  string `/api/health` never returns → goes down after 2 cycles → incident + page).
- On-call: team `Shoply Engineering` (owns both projects), weekly schedule
  `Shoply Primary On-Call`, escalation policy `Shoply Critical`.
- Notification channels (email + escalation) and rules: `check_down` → escalation,
  `new_error`, `endpoint_p95_threshold` on `GET /api/products`, `error_rate_threshold`;
  `new_error` on the react project.
- Status page `Shoply Status` (public, slug `shoply-status`) with a backdated resolved
  incident (`Elevated checkout error rate`) and a full timeline.
- Post-mortem linked to that incident.
- Dashboards: populate-defaults (OTelemetry Server Agent), `golang` template, and a
  hand-crafted `Shoply Business` dashboard (`shop.*` metrics emitted by the backend).
- Optionally mints the react project's source map upload token and writes `frontend/.env`.

## Auth (first match wins)

1. `TRACEWAY_DEMO_PAT` — a `twp_` personal access token
2. `TRACEWAY_DEMO_EMAIL` + `TRACEWAY_DEMO_PASSWORD`
3. The traceway CLI's stored `demo` profile (`traceway login --profile demo`); the script
   immediately trades the 15-minute device JWT for a `demo-seed` PAT.

## Usage

```bash
node seed.mjs                          # seed everything except the sourcemap token
MINT_SOURCEMAP_TOKEN=1 node seed.mjs   # also mint + write frontend/.env (rotates the old token!)
```

Other env: `TRACEWAY_BASE_URL` (default `https://cloud.tracewayapp.com`),
`SHOPLY_REACT_TOKEN` / `SHOPLY_OTEL_TOKEN` (the two projects' ingest tokens, used to find
the project ids), `TRACEWAY_CLI_PROFILE` (default `demo`).

Re-runnable: every create is list-guarded by name/slug; 422 "already exists" responses are
treated as success. The failing monitor only pages on an up→down transition, so re-runs do
not duplicate pages.
