import net from "node:net";
import { trace, SpanKind, SpanStatusCode } from "@opentelemetry/api";
import {
  NodeTracerProvider,
  BatchSpanProcessor,
} from "@opentelemetry/sdk-trace-node";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import {
  defaultResource,
  resourceFromAttributes,
} from "@opentelemetry/resources";

const token = process.env.TW_TOKEN || "5638c2de607f45169bcf98aa8774fe5c";
const url =
  process.env.TW_OTLP_URL || "http://localhost:8082/api/otel/v1/traces";

const provider = new NodeTracerProvider({
  resource: defaultResource().merge(
    resourceFromAttributes({ "service.name": "symbolication-node-app" }),
  ),
  spanProcessors: [
    new BatchSpanProcessor(
      new OTLPTraceExporter({
        url,
        headers: { Authorization: `Bearer ${token}` },
      }),
    ),
  ],
});
provider.register();

const tracer = trace.getTracer("symbolication-node-app");

const err = await new Promise((resolve) => {
  const socket = net.connect(1, "127.0.0.1");
  socket.on("error", resolve);
});

console.log("raw stack:\n" + err.stack);

const span = tracer.startSpan("GET /net-check", {
  kind: SpanKind.SERVER,
  attributes: {
    "http.request.method": "GET",
    "http.route": "/net-check",
    "http.response.status_code": 502,
  },
});
span.recordException(err);
span.setStatus({ code: SpanStatusCode.ERROR, message: err.message });
span.end();

await provider.shutdown();
console.log("flushed");
