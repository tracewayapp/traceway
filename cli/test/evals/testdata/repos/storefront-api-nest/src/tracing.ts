import { NodeSDK } from '@opentelemetry/sdk-node';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-proto';
import { OTLPLogExporter } from '@opentelemetry/exporter-logs-otlp-proto';
import { OTLPMetricExporter } from '@opentelemetry/exporter-metrics-otlp-proto';
import { BatchLogRecordProcessor } from '@opentelemetry/sdk-logs';
import { PeriodicExportingMetricReader } from '@opentelemetry/sdk-metrics';

// Winston logs reach OTLP through @opentelemetry/winston-transport, which the
// winston instrumentation attaches to every logger.
const sdk = new NodeSDK({
  serviceName: 'storefront-api',
  traceExporter: new OTLPTraceExporter(),
  logRecordProcessors: [new BatchLogRecordProcessor(new OTLPLogExporter())],
  metricReaders: [new PeriodicExportingMetricReader({ exporter: new OTLPMetricExporter(), exportIntervalMillis: 60_000 })],
  instrumentations: [getNodeAutoInstrumentations({ '@opentelemetry/instrumentation-fs': { enabled: false } })],
});

sdk.start();
