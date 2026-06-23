# Demo Shop

A tiny, intentionally-buggy shop app. React (Vite) frontend + Go/Gin backend with in-memory
SQLite. No auth, single shared cart. Three pages: products, cart, checkout.

The app is built to **look like it works** but be mostly **slow**, with a few actions that throw
exceptions. Every intentional bug is cataloged in [`BUGS.md`](./BUGS.md).

> **Traceway instrumentation has been removed** from both halves on purpose — it gets re-added
> live during the demo. The bugs in [`BUGS.md`](./BUGS.md) are all still here; right now panics
> are recovered by Gin's own middleware (still 500s) and frontend errors surface in the console
> instead of being reported.

```
shop/
├── build-and-run.sh   build the frontend, embed it in the backend, compile + run
├── DEMO.md            live-demo runbook (bugs first, then add Traceway + symbolication)
├── BUGS.md            the demo script (what's wrong and where)
├── backend/           Go / Gin / SQLite, serves the API + the built frontend on :8090
└── frontend/          React + Vite source
```

## Prerequisites

- Go 1.25+ with **CGO enabled** (`mattn/go-sqlite3` needs a C compiler; clang ships with macOS).
- Node 18+ and npm.

## Run

```bash
./build-and-run.sh
```

The script `npm install`s and builds the frontend (with source maps), copies the built `dist/`
into `backend/dist` (embedded into the Go binary at compile time via `//go:embed`), then builds
and starts the backend. Everything is served from a single origin at **http://localhost:8090** —
open it and click around. There is no separate frontend dev server and no CORS to configure.

For the live demo (show bugs first, then add Traceway and watch them get symbolicated on the
dashboard), follow [`DEMO.md`](./DEMO.md).

### Frontend dev server (optional)

For hot-reload while editing the frontend, run Vite separately. It serves on **:5175** and
proxies `/api/*` to the backend on `:8090`, so start the backend (via `./run.sh`) first.

```bash
cd frontend
npm install
npm run dev
```

## Reproduce the bugs

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

In the browser at http://localhost:8090:

- **Products** load slowly (N+1). "Quick view" throws a frontend exception (#11). "Add to cart"
  can fail silently when the backend slow path 500s (#9).
- **Checkout** → "Apply" a coupon: usually 500s on the backend (#5) and the badge render is caught
  by the error boundary (#10). "Place order" with an empty cart 500s (#6); with items it is slow
  (#7) and occasionally declined (#8, a 402).

## Notes

- The in-memory SQLite DB is reseeded on every backend start, so the cart and orders reset on restart.
- The slow/fast split is `rand.IntN(4) == 0` per request (see `backend/slow.go`) — about 1 in 4 fast.
- Panics are recovered, so the server stays up and returns 500.
