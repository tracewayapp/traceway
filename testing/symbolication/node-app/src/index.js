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
import { fulfillOrder } from "./order.js";

const token = process.env.TW_TOKEN || "5638c2de607f45169bcf98aa8774fe5c";
const url =
  process.env.TW_OTLP_URL || "http://localhost:8082/api/otel/v1/traces";
const mode = process.argv[2] || "event";

async function main() {
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

  const span = tracer.startSpan("POST /orders/fulfill", {
    kind: SpanKind.SERVER,
    attributes: {
      "http.request.method": "POST",
      "http.route": "/orders/fulfill",
      "url.path": "/orders/fulfill",
    },
  });
  try {
    const result = fulfillOrder({ orderId: "ord_1042", units: 3 });
    span.setAttribute("http.response.status_code", 200);
    span.setStatus({ code: SpanStatusCode.OK });
    console.log("order fulfilled:", result);
  } catch (err) {
    span.setAttribute("http.response.status_code", 500);
    span.setStatus({ code: SpanStatusCode.ERROR, message: err.message });
    if (mode === "span") {
      const exceptionSpan = tracer.startSpan("exception", {
        attributes: {
          "exception.type": err.name,
          "exception.message": err.message,
          "exception.stacktrace": err.stack,
        },
      });
      exceptionSpan.end();
    } else {
      span.recordException(err);
    }
    console.error(`reported ${err.name} via mode=${mode}: ${err.message}`);
  }
  span.end();

  await provider.shutdown();
  console.log(`flushed to ${url}`);
}

main().catch((err) => {
  console.error("fatal:", err);
  process.exit(1);
});
