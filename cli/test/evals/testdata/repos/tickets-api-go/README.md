# tickets-api

Event listings, seat maps and ticket reservations. Go, `net/http`,
PostgreSQL through pgx. Traces, logs and the otelhttp request metrics go to
Traceway over OTLP; each VM's OTel Collector adds host metrics.

Runs on three EC2 instances (`tickets-node-a`, `-b`, `-c`) behind an
application load balancer with round-robin routing; see `deploy/terraform/`.

```
go test ./...
go run ./cmd/server
```

Routes:

- `GET /api/events?city=` – upcoming events in a city
- `GET /api/events/{id}` – one event with its seat map
- `POST /api/orders` – reserve seats
- `GET /api/orders/{id}` – one order
