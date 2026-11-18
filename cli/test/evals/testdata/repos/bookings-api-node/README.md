# bookings-api

Availability search, bookings and reviews for the rentals site. Node 24,
Express 5, PostgreSQL 16 (`pg`). Traces go to Traceway through the
OpenTelemetry Node SDK (`src/tracing.js`). The database runs on its own VM
(`pg-bookings-1`, data on `/var/lib/postgresql`, an NVMe volume); the OTel
Collector there ships its host metrics to the same Traceway project as
`bookings-db`.

Batch jobs live in `jobs/` and run as Kubernetes Jobs (`deploy/k8s/`).

```
npm test
npm start
```
