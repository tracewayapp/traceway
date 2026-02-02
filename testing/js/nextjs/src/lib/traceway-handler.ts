import { NextRequest, NextResponse } from "next/server";
import * as traceway from "@traceway/backend";

type HandlerFn = (
  req: NextRequest,
  ctx: { traceId: string; startSpan: (name: string) => ReturnType<typeof traceway.startSpan> }
) => Promise<NextResponse> | NextResponse;

export function withTraceway(handler: HandlerFn) {
  return async (req: NextRequest) => {
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
      const res = await handler(req, { traceId, startSpan });
      const durationMs = performance.now() - startMs;
      const endpoint = `${req.method} ${req.nextUrl.pathname}`;
      const completedSpans = spans.map((s) => traceway.endSpan(s));

      traceway.captureTrace(
        traceId,
        endpoint,
        durationMs,
        startedAt,
        res.status,
        0,
        req.headers.get("x-forwarded-for") || "",
        undefined,
        completedSpans
      );

      return res;
    } catch (err) {
      const durationMs = performance.now() - startMs;
      const endpoint = `${req.method} ${req.nextUrl.pathname}`;
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
        req.headers.get("x-forwarded-for") || "",
        undefined,
        completedSpans
      );

      return NextResponse.json({ error: (err as Error).message }, { status: 500 });
    }
  };
}
