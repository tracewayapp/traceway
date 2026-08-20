# Intentional bugs (current behavior)

Backend errors are recorded on the active OTel span (`RecordError` + status Error), so each
one groups into an Issue attached to its endpoint. Frontend errors are captured by the
`@tracewayapp/react` SDK; fetch failures carry the backend `traceway-trace-id` for
distributed-trace linking.

## Backend

| # | Where | Trigger | Behavior |
|---|---|---|---|
| 1 | `GET /api/products` | ~75% of requests | N+1 category/review-count queries, 200-300ms jitter per product |
| 2 | `GET /api/products/:id` | ~75% | review N+1 (`load_reviews` span) + `recommendations.fetch` sleep |
| 3 | `GET /api/cart` | ~75% | per-line product lookups |
| 4 | `POST /api/cart` | ~75% | `inventory.check` span sleeps 150-500ms |
| 5 | `POST /api/coupon` unknown code | always | 500 "invalid coupon code", recorded error |
| 5b | `POST /api/coupon` `EXPIRED` | always | 400 "this coupon has expired" (not recorded — client error) |
| 6 | `POST /api/checkout` empty cart | always | 500 "your cart is empty", recorded error |
| 7 | `POST /api/checkout` | ~75% | `payment.charge` sleeps 300-1200ms |
| 8 | `POST /api/checkout` | 1 in 6 | 500 "payment declined", recorded error (card digits normalize out of the hash) |
| 12 | `POST /api/coupon` valid code | ~75% | **flagship bug**: `couponHits` map is nil → `assignment to entry in nil map` panic, recovered by the OTel recovery middleware into a 500 + Issue with full Go stack |

## Frontend

| # | Where | Trigger | Behavior |
|---|---|---|---|
| 9 | Products page add-to-cart | backend 500 | `handleAdd` has no try/catch → api.js capture + unhandled rejection |
| 10 | Checkout `CouponBadge` | any failed coupon apply (`discount` becomes null) | render crash `Cannot read properties of null (reading 'percent')`, caught + reported by `TracewayErrorBoundary`, fallback shown |
| 11 | Quick view of **4K Monitor** | always | `p.variants.find(...)` TypeError, captured via `captureException`, "No variants available" toast; all other products work |
