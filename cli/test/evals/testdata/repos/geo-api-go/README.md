# geo-api

Places search for the maps frontend: autocomplete, place details, reverse
geocoding and batch geocoding for imports. Go, `net/http`, PostgreSQL with
PostGIS through pgx. Traces, logs (`log/slog` through the OpenTelemetry
bridge) and the Go runtime metrics
(`go.opentelemetry.io/contrib/instrumentation/runtime`) go to Traceway over
OTLP; host metrics come from the collector DaemonSet.

```
go test ./...
go run ./cmd/server
```

Routes:

- `GET /api/autocomplete?q=&locale=` – the 10 best places for a prefix (memoized)
- `GET /api/places/{id}` – one place
- `GET /api/reverse?lat=&lon=` – places near a point
- `POST /api/geocode/batch` – up to 5000 addresses at once, for imports

Runs on Kubernetes (`deploy/k8s/`), one replica with a 1GiB memory limit and
`GOMEMLIMIT=900MiB`.
