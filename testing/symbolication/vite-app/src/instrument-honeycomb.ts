import { HoneycombWebSDK } from "@honeycombio/opentelemetry-web";

const token =
  import.meta.env.VITE_TW_TOKEN || "5638c2de607f45169bcf98aa8774fe5c";
const endpoint =
  import.meta.env.VITE_TW_OTLP_ENDPOINT || "http://localhost:4318/v1/traces";

const sdk = new HoneycombWebSDK({
  endpoint,
  headers: { Authorization: `Bearer ${token}` },
  serviceName: "symbolication-vite-app",
  skipOptionsValidation: true,
  instrumentations: [],
});

sdk.start();
