import { NodeSDK } from '@opentelemetry/sdk-node'
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node'

new NodeSDK({ instrumentations: [getNodeAutoInstrumentations()] }).start()
