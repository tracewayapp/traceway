# query-api

Search for the document workspace: full-text query with in-process ranking
and highlighting, typeahead suggestions, document fetch. Go, `net/http`, PostgreSQL
(full-text index) through pgx. Two instances (`deploy/query-api.yaml`).
Traces, logs, metrics and host metrics (OTel Collector hostmetrics) go to
Traceway; production builds (`go build -tags otel`) install otelhttp and
otelpgx through `internal/telemetry`.

```
go test ./...
go run ./cmd/server
```
