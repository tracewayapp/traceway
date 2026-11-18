import { getNodeAutoInstrumentations } from "@opentelemetry/auto-instrumentations-node";
import { OTLPLogExporter } from "@opentelemetry/exporter-logs-otlp-http";
import { OTLPMetricExporter } from "@opentelemetry/exporter-metrics-otlp-http";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { resourceFromAttributes } from "@opentelemetry/resources";
import { BatchLogRecordProcessor } from "@opentelemetry/sdk-logs";
import { PeriodicExportingMetricReader } from "@opentelemetry/sdk-metrics";
import { NodeSDK } from "@opentelemetry/sdk-node";

// Let Node apply source maps so server Issues point at the original files.
Error.prepareStackTrace = undefined;

const instance = (process.env.TRACEWAY_URL ?? "").replace(/\/+$/, "");
const base = `${instance}/api/otel`;
const headers = { Authorization: `Bearer ${process.env.TRACEWAY_BACKEND_TOKEN}` };

new NodeSDK({
  resource: resourceFromAttributes({
    "service.name": process.env.OTEL_SERVICE_NAME ?? "catalog",
    "service.version": process.env.APP_VERSION ?? "0.9.0",
  }),
  traceExporter: new OTLPTraceExporter({ url: `${base}/v1/traces`, headers }),
  metricReaders: [
    new PeriodicExportingMetricReader({
      exporter: new OTLPMetricExporter({ url: `${base}/v1/metrics`, headers }),
      exportIntervalMillis: 30_000,
    }),
  ],
  logRecordProcessors: [
    new BatchLogRecordProcessor({
      exporter: new OTLPLogExporter({ url: `${base}/v1/logs`, headers }),
    }),
  ],
  instrumentations: [getNodeAutoInstrumentations()],
}).start();
