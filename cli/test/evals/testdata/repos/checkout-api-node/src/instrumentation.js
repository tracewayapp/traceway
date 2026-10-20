const { NodeSDK } = require('@opentelemetry/sdk-node');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');

new NodeSDK({ instrumentations: [getNodeAutoInstrumentations()] }).start();
