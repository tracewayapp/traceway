# Demo Shop — Intentional Bug Catalog

This app is a deliberately buggy + slow shop, built to make Traceway telemetry look
interesting. It is meant to **appear to work** while being mostly **slow**, with a few
actions that throw exceptions. Nothing here is a real bug to "fix" — it is the demo script.

## Headline mechanic

Most endpoints branch on a single helper:

```go
// backend/slow.go
func fastPath() bool { return rand.IntN(4) == 0 }
```

So **~1 in 4 requests takes the fast, efficient path; the other ~3 in 4 take a slow path**
(an N+1 query loop, a `time.Sleep` jitter span, or a panic). The ratio is **per-endpoint**,
so a single page load that hits several endpoints is almost always slow overall — that is
the intended "the whole shop just feels sluggish" effect.

The backend reports as server **`shop-demo`**, version **`0.1.0`** (set in `backend/main.go`).
The React frontend reports through `@tracewayapp/react` (`frontend/src/main.jsx`). Point both at
the same project token so backend traces and frontend errors group together.

## Backend bugs (Go / Gin / SQLite)

| # | Name | Location | Trigger | App symptom | What shows up in Traceway |
|---|------|----------|---------|-------------|---------------------------|
| 1 | Products N+1 | `backend/products.go` → `listProducts` (slow branch) | ~75% of product-list loads | List loads, but slowly | Slow `GET /api/products` transaction with `1 + 2N` DB spans — one `SELECT name FROM categories WHERE id = ?` and one `SELECT COUNT(*) FROM reviews WHERE product_id = ?` per product |
| 2 | Product-detail N+1 + slow recommendations | `backend/products.go` → `getProduct` (slow branch) | ~75% of detail loads | Detail page sluggish | Named span `load_reviews` wrapping N per-review `SELECT ... WHERE id = ?` spans, plus a `recommendations.fetch` span that just sleeps 80–200ms |
| 3 | Cart N+1 | `backend/cart.go` → `getCart` (slow branch) | ~75% of cart views | Cart slow to render | Per-line `SELECT name, price_cents, image_url FROM products WHERE id = ?` spans |
| 4 | Slow inventory check | `backend/cart.go` → `addToCart` (slow branch) | ~75% of add-to-cart calls | "Add to cart" lags 150–500ms | `inventory.check` span that only sleeps |
| 5 | Coupon nil-map panic | `backend/coupon.go` → `applyCoupon` + package var `couponHits` | ~75% of coupon applies (valid codes) | Apply-coupon fails with a 500 | Exception **`assignment to entry in nil map`** (the package-level `couponHits` map is never initialized; only the fast path guards it). Returns 500, auto-captured by the gin middleware |
| 5b | Unknown-coupon captured error | `backend/coupon.go` → `applyCoupon` (no-rows branch) | Applying a code that isn't seeded | "Invalid coupon code" (400) | Non-panic captured error `unknown coupon code: <code>` via `c.Error(...)` |
| 6 | Checkout index-out-of-range | `backend/checkout.go` → `checkout` (`lines[0]`) | Checking out with an **empty cart** | Checkout 500s | Exception **`index out of range [0] with length 0`** with a Go stack trace |
| 7 | Slow payment gateway | `backend/checkout.go` → `checkout` (`payment.charge`) | Every slow-path checkout | Checkout takes 0.3–1.2s | `payment.charge` span (sleeps 300–1200ms) + `order.persist` span; slow transaction |
| 8 | Payment declined (manual capture) | `backend/checkout.go` → `checkout` (`rand.IntN(6)==0`) | ~1 in 6 checkouts with items | "Payment declined, try another card" (402) | Exception **`payment declined for card ****NNNN`** reported via `traceway.CaptureExceptionWithContext` — captured without a 500, demonstrating manual capture |

Notes:
- Seeded coupons: `SAVE10` (10%), `HALF50` (50%), `EXPIRED` (inactive → 400 "expired", not an exception).
- A valid coupon therefore "works about 1 in 4 tries"; the rest panic (#5). That is the literal
  reading of "works 1/4 of the time, slow/broken the rest."
- Panics are recovered (the server never dies) and return 500. The exception is captured **before**
  the response is finalized, so it still reaches Traceway.

## Frontend bugs (React / Vite)

| # | Name | Location | Trigger | App symptom | What shows up in Traceway |
|---|------|----------|---------|-------------|---------------------------|
| 9 | Uncaught async add-to-cart | `frontend/src/ProductsPage.jsx` → `handleAdd` | Backend 500s on the add-to-cart slow path | Button silently fails to add | Unhandled promise rejection auto-captured by the SDK (the handler has no try/catch on purpose) |
| 10 | Render-time null deref | `frontend/src/CheckoutPage.jsx` → `CouponBadge` | A failed coupon apply sets `discount = null`, then the badge renders | Coupon area shows "Could not display coupon." | Frontend render exception `Cannot read properties of null (reading 'percent')`, caught by `TracewayErrorBoundary` (reported, with a fallback so the page stays usable) |
| 11 | Undefined deref in handler | `frontend/src/ProductsPage.jsx` → `handleQuickView` | Clicking "Quick view" (products have no `variants` field) | Quick view shows "No variants available" | Frontend exception `Cannot read properties of undefined (reading 'find')`, manually reported via `useTraceway().captureException` |

## Coverage

- **Slowness**: #1, #2, #3, #4, #7 (N+1 fan-outs + named sleep spans → slow transactions).
- **Exceptions**: #5, #6, #8 (backend) and #9, #10, #11 (frontend).
- **Capture styles**: auto panic capture (#5, #6), gin `c.Error` (#5b), manual backend capture (#8),
  SDK global unhandled-rejection (#9), React error boundary (#10), manual frontend capture (#11).

See `README.md` for how to run everything and reproduce each item.
