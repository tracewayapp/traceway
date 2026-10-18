import { NodeSDK } from '@opentelemetry/sdk-node';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';

// OTEL_EXPORTER_OTLP_ENDPOINT and OTEL_SERVICE_NAME come from the deployment.
const sdk = new NodeSDK({
  instrumentations: [getNodeAutoInstrumentations({ '@opentelemetry/instrumentation-fs': { enabled: false } })],
});
sdk.start();
