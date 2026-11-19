import { register } from 'node:module';
import { NodeSDK } from '@opentelemetry/sdk-node';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';

// The app is ESM, so the instrumentations patch its imports through the loader hook.
register('@opentelemetry/instrumentation/hook.mjs', import.meta.url);

// OTEL_EXPORTER_OTLP_ENDPOINT and OTEL_SERVICE_NAME come from the deployment.
const sdk = new NodeSDK({
  instrumentations: [getNodeAutoInstrumentations({ '@opentelemetry/instrumentation-fs': { enabled: false } })],
});
sdk.start();
