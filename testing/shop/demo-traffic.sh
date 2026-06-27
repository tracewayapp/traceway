#!/usr/bin/env bash
# Drives every BACKEND bug so it shows up in Traceway. The three frontend bugs
# (#9, #10, #11) are client-side JS and must be triggered in the browser — see the
# checklist printed at the end. Re-run any time to accumulate more volume.
#
# Usage: ./demo-traffic.sh [base-url]   (default http://localhost:8090)
set -uo pipefail

BASE="${1:-http://localhost:8090}"
CHECKOUT='{"name":"Ada","email":"ada@example.com","card_last4":"4242"}'

hit() { # method path [json]
  local m=$1 p=$2 d=${3:-}
  if [ -n "$d" ]; then
    curl -s -o /dev/null -w '%{http_code}' -X "$m" "$BASE$p" -H 'Content-Type: application/json' -d "$d"
  else
    curl -s -o /dev/null -w '%{http_code}' -X "$m" "$BASE$p"
  fi
}

timed() { # method path [json] -> "code (1.234s)"
  local m=$1 p=$2 d=${3:-}
  if [ -n "$d" ]; then
    curl -s -o /dev/null -w '%{http_code}(%{time_total}s)' -X "$m" "$BASE$p" -H 'Content-Type: application/json' -d "$d"
  else
    curl -s -o /dev/null -w '%{http_code}(%{time_total}s)' -X "$m" "$BASE$p"
  fi
}

empty_cart() {
  for id in $(curl -s "$BASE/api/cart" | grep -oE '"id":[0-9]+' | grep -oE '[0-9]+'); do
    curl -s -o /dev/null -X DELETE "$BASE/api/cart/$id"
  done
}

echo "== 0. Health =="
echo "  /api/health -> $(hit GET /api/health)"

echo "== 1. Slow product list, N+1 (#1) — ~3 of 4 take the slow fan-out =="
printf '  '; for i in $(seq 1 18); do printf '%s ' "$(timed GET /api/products)"; done; echo

echo "== 2. Slow product detail, N+1 + recommendations span (#2) =="
printf '  '; for id in 1 2 3 4 5 6 7 8; do printf 'id%s=%s ' "$id" "$(timed GET "/api/products/$id")"; done; echo

echo "== 3. Coupon nil-map PANIC (#5) — POST /api/coupon SAVE10, every call -> 500 =="
codes=""; for i in $(seq 1 12); do codes+="$(hit POST /api/coupon '{"code":"SAVE10"}') "; done
echo "  $codes"
echo "  -> $(grep -o 500 <<<"$codes" | wc -l | tr -d ' ')/12 panicked (assignment to entry in nil map)"

echo "== 4. Unknown-coupon captured error (#5b) — code that isn't seeded -> 400 + c.Error =="
printf '  '; for i in 1 2 3; do printf '%s ' "$(hit POST /api/coupon '{"code":"NOPE"}')"; done; echo

echo "== 5. Empty-cart checkout PANIC (#6) — index out of range [0] -> 500 =="
empty_cart
codes=""; for i in $(seq 1 6); do codes+="$(hit POST /api/checkout "$CHECKOUT") "; done
echo "  $codes (500 = panic; an occasional 402 = the 1/6 decline firing first)"

echo "== 6. Slow add-to-cart, inventory.check span (#4) — also fills the cart =="
printf '  '; for i in $(seq 1 8); do printf '%s ' "$(timed POST /api/cart '{"product_id":4,"qty":1}')"; done; echo

echo "== 7. Cart N+1 (#3) — GET /api/cart with items, ~75% slow per-line lookups =="
printf '  '; for i in $(seq 1 10); do printf '%s ' "$(timed GET /api/cart)"; done; echo

echo "== 8. Slow checkout (#7) + declined card (#8) — re-add an item each pass so it isn't empty =="
printf '  '
for i in $(seq 1 12); do
  hit POST /api/cart '{"product_id":5,"qty":1}' >/dev/null
  printf '%s ' "$(timed POST /api/checkout "$CHECKOUT")"
done; echo
echo "  (slow 0.3-1.2s = #7 payment.charge; 402 = declined #8; 200 = order placed)"

cat <<'EOF'

== Done — open the Traceway dashboard, filter server "shop-demo" / version "0.1.0" ==
  Issues:       nil-map panic (#5), index-out-of-range panic (#6), declined-card (#8),
                unknown-coupon (#5b)
  Transactions: slow GET /api/products with the N+1 span fan-out, slow checkout payment.charge

Frontend bugs are client-side JS — trigger these in the browser at the base URL:
  #11  Products -> "Quick view" on any card          (undefined .find, manual capture)
  #9   Products -> "Add to cart" a few times          (slow-path 500 -> unhandled promise rejection)
  #10  Checkout -> enter a coupon, click "Apply"      (failed apply -> null-deref in badge render,
                   until it errors                      caught by the error boundary)
EOF
