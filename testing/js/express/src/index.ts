import express, { Request, Response, NextFunction } from "express";
import * as traceway from "@traceway/backend";

const endpoint =
  process.env.TRACEWAY_ENDPOINT ||
  "default_token_change_me@http://localhost:8082/api/report";
const port = process.env.PORT || "8080";

traceway.init(endpoint, { debug: true });

const app = express();
app.use(express.json());

function generateTraceId(): string {
  return crypto.randomUUID().replace(/-/g, "");
}

app.use((req: Request, res: Response, next: NextFunction) => {
  const traceId = generateTraceId();
  const startedAt = new Date();
  const startMs = performance.now();
  const spans: ReturnType<typeof traceway.startSpan>[] = [];

  (req as any).traceId = traceId;
  (req as any).startSpan = (name: string) => {
    const span = traceway.startSpan(name);
    spans.push(span);
    return span;
  };

  res.on("finish", () => {
    const durationMs = performance.now() - startMs;
    const endpoint = `${req.method} ${req.route?.path || req.path}`;
    const completedSpans = spans
      .filter((s) => (s as any).duration !== undefined)
      .map((s) => traceway.endSpan(s));
    const pendingSpans = spans.filter(
      (s) => (s as any).duration === undefined
    );
    for (const s of pendingSpans) {
      completedSpans.push(traceway.endSpan(s));
    }

    traceway.captureTrace(
      traceId,
      endpoint,
      durationMs,
      startedAt,
      res.statusCode,
      0,
      req.ip || "",
      undefined,
      completedSpans
    );
  });

  next();
});

function innerFunction(): Error {
  const err = new Error("error from inner function");
  Error.captureStackTrace(err, innerFunction);
  return err;
}

function middleFunction(): Error {
  return innerFunction();
}

function outerFunction(): Error {
  return middleFunction();
}

app.get("/test-ok", (_req: Request, res: Response) => {
  res.json({ status: "ok" });
});

app.get("/test-not-found", (_req: Request, res: Response) => {
  res.status(404).json({ status: "not-found" });
});

app.get("/test-exception", () => {
  throw new Error("test panic from /test-exception");
});

app.get("/test-error-simple", (req: Request, res: Response) => {
  const err = new Error("simple error without stack");
  traceway.captureExceptionWithAttributes(err, undefined, (req as any).traceId);
  res.status(500).json({ error: "simple error" });
});

app.get("/test-error-stacktrace", (req: Request, res: Response) => {
  const err = new Error("error with stack trace");
  traceway.captureExceptionWithAttributes(err, undefined, (req as any).traceId);
  res.status(500).json({ error: "stacktrace error" });
});

app.get("/test-error-wrapped", (req: Request, res: Response) => {
  const base = new Error("base error");
  const wrapped = new Error("layer 1: " + base.message, { cause: base });
  const wrapped2 = new Error("layer 2: " + wrapped.message, {
    cause: wrapped,
  });
  traceway.captureExceptionWithAttributes(
    wrapped2,
    undefined,
    (req as any).traceId
  );
  res.status(500).json({ error: "wrapped error" });
});

app.get("/test-error-nested", (req: Request, res: Response) => {
  const err = outerFunction();
  traceway.captureExceptionWithAttributes(err, undefined, (req as any).traceId);
  res.status(500).json({ error: "nested error" });
});

app.get("/test-message", (_req: Request, res: Response) => {
  traceway.captureMessage("test message from /test-message");
  res.json({ status: "message sent" });
});

app.get("/test-message-attributes", (_req: Request, res: Response) => {
  // JS SDK captureMessage does not support attributes; capturing as message only
  traceway.captureMessage("test message with attributes");
  res.json({ status: "message with attributes sent" });
});

app.get("/test-spans", (req: Request, res: Response) => {
  const startSpan = (req as any).startSpan as (
    name: string
  ) => ReturnType<typeof traceway.startSpan>;

  const dbSpan = startSpan("db.query");
  setTimeout(() => {
    traceway.endSpan(dbSpan);

    const cacheSpan = startSpan("cache.set");
    setTimeout(() => {
      traceway.endSpan(cacheSpan);

      const httpSpan = startSpan("http.external_api");
      setTimeout(() => {
        traceway.endSpan(httpSpan);
        res.json({ status: "spans captured" });
      }, 100);
    }, 20);
  }, 50);
});

app.get("/test-task", (_req: Request, res: Response) => {
  traceway.measureTask("background-data-processor", async () => {
    const span = traceway.startSpan("processing");
    await new Promise((resolve) => setTimeout(resolve, 200));
    traceway.endSpan(span);
  });
  res.json({ status: "task started" });
});

app.get("/test-metric", (_req: Request, res: Response) => {
  traceway.captureMetric("test.custom_metric", 42.0);
  res.json({ status: "metric captured" });
});

app.get("/test-attributes", (_req: Request, res: Response) => {
  traceway.captureExceptionWithAttributes(
    new Error("exception with custom attributes"),
    {
      user_id: "usr_123",
      request_id: "req_456",
      env: "testing",
    }
  );
  res.json({ status: "exception with attributes captured" });
});

app.post("/test-recording", (req: Request, res: Response) => {
  const body = req.body;

  if (body?.action === "panic") {
    throw new Error("panic triggered by /test-recording");
  }

  res.json({ status: "ok", received: body });
});

app.use((err: Error, req: Request, res: Response, _next: NextFunction) => {
  traceway.captureExceptionWithAttributes(
    err,
    undefined,
    (req as any).traceId
  );
  res.status(500).json({ error: err.message });
});

app.listen(Number(port), () => {
  console.log(`Express server starting on :${port}`);
});
