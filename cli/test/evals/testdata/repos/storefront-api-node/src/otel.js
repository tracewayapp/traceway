import { NodeSDK } from '@opentelemetry/sdk-node'
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node'

// Endpoint and headers come from OTEL_EXPORTER_OTLP_* in the environment.
new NodeSDK({ instrumentations: [getNodeAutoInstrumentations()] }).start()
