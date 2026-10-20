import { register } from 'node:module'
import { NodeSDK } from '@opentelemetry/sdk-node'
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node'
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-proto'
import { OTLPLogExporter } from '@opentelemetry/exporter-logs-otlp-proto'
import { BatchLogRecordProcessor } from '@opentelemetry/sdk-logs'

// The app is ESM, so the instrumentations patch its imports through the loader hook.
register('@opentelemetry/instrumentation/hook.mjs', import.meta.url)

// Winston logs reach OTLP through @opentelemetry/winston-transport, which the
// winston instrumentation attaches to every logger.
new NodeSDK({
  serviceName: 'shop-api',
  traceExporter: new OTLPTraceExporter(),
  logRecordProcessors: [new BatchLogRecordProcessor(new OTLPLogExporter())],
  instrumentations: [getNodeAutoInstrumentations({ '@opentelemetry/instrumentation-fs': { enabled: false } })],
}).start()
