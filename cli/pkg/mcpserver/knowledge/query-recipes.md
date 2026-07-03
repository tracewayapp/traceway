### Recipes

```bash
# What's broken right now
traceway exceptions list --since 1h --order-by lastSeen --page-size 10 --output json \
  | jq '.data[]? | {hash: .exceptionHash, count, lastSeen}'

# Did anything NEW break since a deploy at 13:00 UTC
traceway exceptions list --from 2026-06-11T13:00:00Z --to "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --order-by firstSeen --output json \
  | jq '.data[]? | select(.firstSeen >= "2026-06-11T13:00:00Z") | {hash: .exceptionHash, firstSeen, count}'

# Worst endpoint by latency
traceway endpoints list --since 1h --order-by p95 --page-size 1 --output json | jq '.data[0]'

# Errors for one service (exceptions --search is free text, not a service filter; use logs)
traceway logs query --service checkout-api --min-severity 17 --since 1h --output json \
  | jq '.data[]? | {timestamp, body, traceId}'
```

Empty results (`data: null` or `data: []`) are not errors: widen the window, re-check the active project (`traceway projects list`), and if the app was never connected to Traceway, set it up first (the `traceway-setup` skill).
