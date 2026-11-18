// Loaded before the app with `node --import ./instrumentation.mjs server.js`.
// The ESM loader hook must be registered first: without it Express is not
// patched in an ES-module app and every URL becomes its own endpoint.
import { register } from "node:module";
register("@opentelemetry/instrumentation/hook.mjs", import.meta.url);

import { NodeSDK } from "@opentelemetry/sdk-node";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { OTLPMetricExporter } from "@opentelemetry/exporter-metrics-otlp-http";
import { PeriodicExportingMetricReader } from "@opentelemetry/sdk-metrics";
import { getNodeAutoInstrumentations } from "@opentelemetry/auto-instrumentations-node";

const base = `${(process.env.TRACEWAY_URL ?? "").replace(/\/+$/, "")}/api/otel`;
const headers = { Authorization: `Bearer ${process.env.TRACEWAY_TOKEN}` };

const sdk = new NodeSDK({
  serviceName: process.env.OTEL_SERVICE_NAME ?? "orders-api",
  traceExporter: new OTLPTraceExporter({ url: `${base}/v1/traces`, headers }),
  metricReaders: [
    new PeriodicExportingMetricReader({
      exporter: new OTLPMetricExporter({ url: `${base}/v1/metrics`, headers }),
      exportIntervalMillis: 30_000,
    }),
  ],
  instrumentations: [
    getNodeAutoInstrumentations({
      "@opentelemetry/instrumentation-fs": { enabled: false },
    }),
  ],
});

sdk.start();

for (const signal of ["SIGTERM", "SIGINT"]) {
  process.once(signal, () => {
    sdk.shutdown().finally(() => process.exit(0));
  });
}
