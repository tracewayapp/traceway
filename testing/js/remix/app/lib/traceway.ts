import * as traceway from "@traceway/backend";

const endpoint =
  process.env.TRACEWAY_ENDPOINT ||
  "default_token_change_me@http://localhost:8082/api/report";

let initialized = false;

export function ensureInit() {
  if (!initialized) {
    traceway.init(endpoint, { debug: true });
    initialized = true;
  }
}

type HandlerFn = (
  request: Request,
  ctx: { traceId: string; startSpan: (name: string) => ReturnType<typeof traceway.startSpan> }
) => Promise<Response>;

export function withTraceway(handler: HandlerFn) {
  return async ({ request }: { request: Request }) => {
    ensureInit();

    const traceId = crypto.randomUUID().replace(/-/g, "");
    const startedAt = new Date();
    const startMs = performance.now();
    const spans: ReturnType<typeof traceway.startSpan>[] = [];

    const startSpan = (name: string) => {
      const span = traceway.startSpan(name);
      spans.push(span);
      return span;
    };

    try {
      const res = await handler(request, { traceId, startSpan });
      const durationMs = performance.now() - startMs;
      const url = new URL(request.url);
      const endpoint = `${request.method} ${url.pathname}`;
      const completedSpans = spans.map((s) => traceway.endSpan(s));

      traceway.captureTrace(
        traceId,
        endpoint,
        durationMs,
        startedAt,
        res.status,
        0,
        request.headers.get("x-forwarded-for") || "",
        undefined,
        completedSpans
      );

      return res;
    } catch (err) {
      const durationMs = performance.now() - startMs;
      const url = new URL(request.url);
      const endpoint = `${request.method} ${url.pathname}`;
      const completedSpans = spans.map((s) => traceway.endSpan(s));

      if (err instanceof Error) {
        traceway.captureExceptionWithAttributes(err, undefined, traceId);
      }

      traceway.captureTrace(
        traceId,
        endpoint,
        durationMs,
        startedAt,
        500,
        0,
        request.headers.get("x-forwarded-for") || "",
        undefined,
        completedSpans
      );

      return new Response(JSON.stringify({ error: (err as Error).message }), {
        status: 500,
        headers: { "Content-Type": "application/json" },
      });
    }
  };
}

export { traceway };
