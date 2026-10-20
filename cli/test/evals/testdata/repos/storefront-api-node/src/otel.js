import { register } from 'node:module'
import { NodeSDK } from '@opentelemetry/sdk-node'
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node'

// The app is ESM, so the instrumentations patch its imports through the loader hook.
register('@opentelemetry/instrumentation/hook.mjs', import.meta.url)

// Endpoint and headers come from OTEL_EXPORTER_OTLP_* in the environment.
new NodeSDK({ instrumentations: [getNodeAutoInstrumentations()] }).start()
