# Demo Shop

A tiny, intentionally-buggy shop app for demoing Traceway. React (Vite) frontend + Go/Gin
backend with in-memory SQLite. No auth, single shared cart. Three pages: products, cart, checkout.

The app is built to **look like it works** but be mostly **slow**, with a few actions that throw
exceptions. Every intentional bug is cataloged in [`BUGS.md`](./BUGS.md).

```
shop/
├── BUGS.md            the demo script (what's wrong and where)
├── backend/           Go / Gin / SQLite, serves :8090, reports as "shop-demo"
└── frontend/          React + Vite, serves :5175, proxies /api -> :8090
```

## Prerequisites

- A Traceway backend running locally on **:8082** (the `/api/report` ingestion endpoint).
- A Traceway **project token** — create a project in the dashboard and copy its token.
- Go 1.25+ with **CGO enabled** (`mattn/go-sqlite3` needs a C compiler; clang ships with macOS).
- Node 18+ and npm.

The backend and frontend share one connection string: `<token>@http://localhost:8082/api/report`.
Use the **same token** on both so backend traces and frontend errors land in the same project.

## Run the backend

```bash
cd backend
go mod tidy                       # first time only
TRACEWAY_ENDPOINT="<token>@http://localhost:8082/api/report" CGO_ENABLED=1 go run .
```

Serves on **http://localhost:8090**. Without `TRACEWAY_ENDPOINT` it falls back to a
`default_token_change_me@...` placeholder and telemetry won't be accepted.

## Run the frontend

```bash
cd frontend
npm install                       # first time only
cp .env.example .env              # then set VITE_TW_CONNECTION to your token
npm run dev
```

Serves on **http://localhost:5175**. The dev server proxies `/api/*` to the backend on `:8090`,
so there is no CORS to configure. Open the URL and click around.

`.env`:

```
VITE_TW_CONNECTION=<token>@http://localhost:8082/api/report
VITE_API_BASE=/api
```

## Reproduce the telemetry

With both halves running and Traceway up on :8082:

```bash
# 1. Health
curl localhost:8090/api/health                       # {"status":"ok"}

# 2. Slow + N+1 product list (most calls hit the slow path)
for i in $(seq 1 12); do curl -s -o /dev/null localhost:8090/api/products; done

# 3. Coupon nil-map panic (~75% return 500)
for i in $(seq 1 8); do
  curl -s -o /dev/null -w "%{http_code} " -X POST localhost:8090/api/coupon \
    -H 'Content-Type: application/json' -d '{"code":"SAVE10"}'
done; echo

# 4. Empty-cart checkout panic (500: index out of range)
curl -s -o /dev/null -w "%{http_code}\n" -X POST localhost:8090/api/checkout \
  -H 'Content-Type: application/json' -d '{"name":"A","email":"a@b.c","card_last4":"4242"}'
```

In the browser at http://localhost:5175:

- **Products** load slowly (N+1). "Quick view" throws a frontend exception (#11). "Add to cart"
  can fail silently when the backend slow path 500s (#9).
- **Checkout** → "Apply" a coupon: usually 500s on the backend (#5) and the badge render is caught
  by the error boundary (#10). "Place order" with an empty cart 500s (#6); with items it is slow
  (#7) and occasionally declined (#8, a 402).

Then in the Traceway dashboard (:8082): find server **`shop-demo`** / version **`0.1.0`**.
Transactions show slow `GET /api/products` with the N+1 DB-span fan-out; Exceptions show the
nil-map panic, the index-out-of-range panic, the declined-card capture, and the three frontend
errors. Cross-reference each against [`BUGS.md`](./BUGS.md).

## Notes

- The in-memory SQLite DB is reseeded on every backend start, so the cart and orders reset on restart.
- The slow/fast split is `rand.IntN(4) == 0` per request (see `backend/slow.go`) — about 1 in 4 fast.
- Panics are recovered; the server stays up and returns 500 while still capturing the exception.
