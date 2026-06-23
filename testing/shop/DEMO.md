# Demo Shop — Live Demo Runbook

Three acts:

1. **Bugs, no observability** — run the app as-is (Traceway stripped out). It's slow and flaky,
   errors are swallowed or buried in the browser console. You have no idea what's breaking.
2. **Add Traceway** — re-instrument both halves. Every bug now surfaces as an issue with a stack
   trace, tags, and a frequency, plus slow transactions.
3. **Symbolication** — the frontend stack traces arrive minified (`index-abc123.js:1:48211`).
   Upload source maps and they turn into real `CheckoutPage.jsx:8` frames in your source.

The bug catalog (numbers `#1`–`#11`) is in [`BUGS.md`](./BUGS.md).

## Prerequisites

- A Traceway instance running (these notes assume **http://localhost:8082**) with a **project** +
  its **project token**.
- Go 1.25+ with CGO, Node 18+.
- The sibling SDK repos the app links against must exist (they were used to build it originally):
  - Go SDK at `../../../go-client` (relative to `backend/`)
  - JS SDK at `../../../js-client` (relative to `frontend/`)

> **Tip:** the current working tree is the *stripped* (no-Traceway) version. If you want to be able
> to return to it after the demo, commit it on a branch first (e.g. `git switch -c shop-no-tw`).
> The fully-instrumented version is preserved in commit **`fd21c509`**, which Act 2 restores from.

---

## Act 1 — Bugs with no observability

```bash
cd testing/shop
./build-and-run.sh
```

Open **http://localhost:8090** and drive it (open the browser devtools console too):

- **Products page** — everything loads *slowly* (N+1, `#1`). Click **Quick view** → "No variants
  available" (`#11`, swallowed). Click **Add to cart** a few times → it occasionally just doesn't
  add and you only see a red error in the console if you're looking (`#9`).
- **Checkout** — type a coupon (`SAVE10`) and **Apply** → it usually errors (`#5`), and the badge
  area shows "Could not display coupon." (`#10`, caught by a local boundary). **Place order** with
  an empty cart → it fails (`#6`); with items it's slow (`#7`) and sometimes declined (`#8`).

Backend bugs from the terminal:

```bash
for i in $(seq 1 8); do curl -s -o /dev/null -w "%{http_code} " -X POST localhost:8090/api/coupon \
  -H 'Content-Type: application/json' -d '{"code":"SAVE10"}'; done; echo   # ~75% 500
curl -s -o /dev/null -w "%{http_code}\n" -X POST localhost:8090/api/checkout \
  -H 'Content-Type: application/json' -d '{"name":"A","email":"a@b.c","card_last4":"4242"}'  # 500
```

**The point:** the app is clearly unhealthy, but the failures are swallowed, intermittent, or
lost in the console. You can't see *what* is breaking, *how often*, or *where*.

Stop the server (`Ctrl-C`) before Act 2.

---

## Act 2 — Add Traceway

The instrumented version of every handler and React component lives in commit `fd21c509`. Restore
those, keep the new "serve frontend from backend" wiring, and point the SDKs at your instance.

**1. Restore the instrumented handlers + frontend + Go deps** (everything except `main.go`, which
needs the new static-serving line):

```bash
cd testing/shop
git checkout fd21c509 -- \
  backend/cart.go backend/products.go backend/checkout.go backend/coupon.go \
  backend/go.mod backend/go.sum \
  frontend/src/main.jsx frontend/src/ProductsPage.jsx frontend/src/CheckoutPage.jsx \
  frontend/package.json
```

This brings back the Go `tracewaygin` middleware deps, the per-handler `tracewaydb` spans and
manual captures (`#8`), and the React `TracewayProvider` / `useTraceway` / `TracewayErrorBoundary`
usage (`#9`, `#10`, `#11`).

**2. Put `backend/main.go` back to instrumented *and* keep serving the embedded frontend.** Replace
the whole file with:

```go
package main

import (
	"database/sql"
	"net/http"
	"os"

	tracewaygin "go.tracewayapp.com/tracewaygin"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func main() {
	initDB()

	endpoint := os.Getenv("TRACEWAY_ENDPOINT")
	if endpoint == "" {
		endpoint = "default_token_change_me@http://localhost:8082/api/report"
	}

	router := gin.Default()
	router.Use(corsMiddleware())
	router.Use(tracewaygin.New(
		endpoint,
		tracewaygin.WithDebug(true),
		tracewaygin.WithServerName("shop-demo"),
		tracewaygin.WithVersion("0.1.0"),
		tracewaygin.WithOnErrorRecording(tracewaygin.RecordingUrl|tracewaygin.RecordingQuery|tracewaygin.RecordingHeader|tracewaygin.RecordingBody),
	))

	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/api/products", listProducts)
	router.GET("/api/products/:id", getProduct)
	router.GET("/api/cart", getCart)
	router.POST("/api/cart", addToCart)
	router.DELETE("/api/cart/:id", removeFromCart)
	router.POST("/api/coupon", applyCoupon)
	router.POST("/api/checkout", checkout)

	registerFrontend(router)

	router.Run(":8090")
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
```

**3. Point both SDKs at your instance + project token.** Use the **same** project token for both.

Frontend — Vite reads this at build time (baked into the bundle):

```bash
echo 'VITE_TW_CONNECTION=<PROJECT_TOKEN>@http://localhost:8082/api/report' > frontend/.env
```

Backend — read at runtime from the environment.

**4. Rebuild and run** (the env var is inherited by the embedded `./shop`):

```bash
TRACEWAY_ENDPOINT="<PROJECT_TOKEN>@http://localhost:8082/api/report" ./build-and-run.sh
```

`build-and-run.sh` reinstalls the now-restored `@tracewayapp/react` dep and rebuilds the bundle
(with source maps). If the Go build complains about the restored modules, run
`cd backend && go mod tidy` once and re-run the script.

**5. Re-drive the same actions** from Act 1 (UI + the curl loops). Now open the Traceway dashboard
for your project:

- **Issues** — the nil-map panic (`assignment to entry in nil map`, `#5`), the
  `index out of range [0] with length 0` panic (`#6`), the declined-card capture (`#8`), and the
  three frontend errors (`#9`, `#10`, `#11`) — each with a count and a stack trace.
- **Endpoints / transactions** — `GET /api/products` is slow with the N+1 fan-out; checkout shows
  the `payment.charge` span.
- Filter by server **`shop-demo`**, version **`0.1.0`**.

---

## Act 3 — Symbolication of the frontend errors

Open one of the **frontend** issues (`#10` CouponBadge, `#11` Quick view, or `#9` add-to-cart). The
stack trace is minified gibberish — something like `index-DSxfDYbG.js:1:48211`. That's because the
browser only ever sees the production bundle. Fix it by uploading the source maps.

**How matching works (no version juggling):** Traceway matches a frame to a map by **filename**
(`index-<hash>.js` → `index-<hash>.js.map`), or by embedded debug-id. The SDK's `version: 0.1.0` is
just metadata for filtering — it is **not** used for map matching. The only rule that matters:
**upload the maps from the exact build you are serving.** `build-and-run.sh` already emitted them
to `frontend/dist/assets/*.map`, and it embedded that same `dist` into the running binary, so the
hashes line up. (If you rebuild, re-upload.)

**1. Get a source-map upload token** — in the dashboard (Project settings → source maps), or via
the API with your dashboard JWT:

```bash
curl -s -X POST http://localhost:8082/api/projects/source-map-token \
  -H "Authorization: Bearer <DASHBOARD_JWT>" -H 'Content-Type: application/json' \
  -d '{}'    # -> {"sourceMapToken":"..."}
```

**2. Upload the maps** (and their sibling `.js`, which lets Traceway resolve function names too):

```bash
npx @tracewayapp/sourcemap-upload \
  --url http://localhost:8082 \
  --token "<SOURCEMAP_TOKEN>" \
  --directory frontend/dist
```

Dependency-free alternative with `curl` (single entry bundle):

```bash
cd frontend/dist/assets
curl -sS -X POST http://localhost:8082/api/sourcemaps/upload \
  -H "Authorization: Bearer <SOURCEMAP_TOKEN>" \
  -F "files=@$(ls index-*.js)" \
  -F "files=@$(ls index-*.js.map)"          # -> {"uploaded":2}
cd ../../..
```

**3. Trigger the frontend bug again** (e.g. apply a bad coupon for `#10`, or Quick view for `#11`)
so a fresh event arrives, then open that issue. The same frame now reads
`CheckoutPage.jsx:8:...` / `ProductsPage.jsx:...` — your actual source, function names and all.

> Upload the maps **before** the event you want to show symbolicated. A nice narrative is: trigger
> once to show the minified trace, upload, trigger again, open the new event to show it resolved.

---

## Reset

- Back to "bugs, no observability": discard the Act 2 restores
  (`git checkout fd21c509^ -- ...` won't help — restore your stripped branch, or
  `git restore --source=<your-no-tw-commit> -- <files>`), or just keep the `shop-no-tw` branch you
  made in the prereqs and `git switch` to it.
- The SQLite DB is in-memory and reseeded on every start, so cart/orders reset whenever you restart
  the server.
