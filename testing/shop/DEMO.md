# Shoply demo runbook

> The full setup lives in README.md (build, seed, traffic). This file is the tour to give
> once data is flowing.

Projects: **Shoply Storefront** (react — issues, sessions, replays) and
**Shoply API** (opentelemetry — endpoints, traces, logs, metrics, AI, monitors).

## Suggested tour

1. **Issues (backend)** — open `assignment to entry in nil map`: full Go stack from the
   recovered panic, occurrences over time, the owning endpoint `POST /api/coupon`, and the
   trace waterfall showing the SQL spans.
2. **Distributed tracing** — open a react issue like `Request to /coupon failed (500)`:
   the occurrence carries the distributed trace id and links straight to the backend trace.
3. **Session replay** — from the same react issue open the replay clip; then the Sessions
   page for full sessions with varied users, viewports and rage-scrolling window shoppers.
4. **Endpoints** — `GET /api/products` p95 vs p50 (N+1 slow path), `POST /api/checkout`
   latency spread, all endpoints grouped by route (ids collapse into `:id`).
5. **Logs** — filter severity ERROR; every line links to its trace.
6. **Metrics & dashboards** — Server/Application Metrics pages (`shop-backend`), the
   OTelemetry Server Agent + Go Application dashboards, and Shoply Business (orders,
   revenue, coupon failures).
7. **AI Traces** — Conversations tab: the order-status conversation shows the
   `lookup_order` tool call; the refund complaint is flagged; Users tab has per-user costs.
8. **Monitors** — three green checks + `Payments Provider` down: incident, uptime bars,
   and the check_down page it opened (Pages sidebar badge). Ack it live.
9. **On-call** — team, weekly schedule, escalation policy; `/status/shoply-status` public
   page with the resolved `Elevated checkout error rate` incident timeline.
10. **Post-mortem** — the linked write-up under Monitors → Post-Mortems.
