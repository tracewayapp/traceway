# commerce-api

The shop's API: catalog, search, cart, checkout and recommendations. Node 24,
Express 5, PostgreSQL (`pg`), Redis (`ioredis`) and the search cluster.
Traces, logs (the access log in `src/access-log.js`) and metrics go to
Traceway over OTLP through the OpenTelemetry Node SDK (`src/tracing.js`) via
the OTel Collector sidecar in each pod, whose hostmetrics receiver also
reports the pod's CPU and memory.

Runs on Kubernetes, see `deploy/k8s/`.

```
npm test
npm start
```
