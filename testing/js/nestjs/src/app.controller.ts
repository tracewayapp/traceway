import { Controller, Get, Post, Req, Res, HttpStatus } from "@nestjs/common";
import { Request, Response } from "express";
import * as traceway from "@traceway/backend";

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

@Controller()
export class AppController {
  @Get("test-ok")
  testOk() {
    return { status: "ok" };
  }

  @Get("test-not-found")
  testNotFound(@Res() res: Response) {
    res.status(404).json({ status: "not-found" });
  }

  @Get("test-exception")
  testException() {
    throw new Error("test panic from /test-exception");
  }

  @Get("test-error-simple")
  testErrorSimple(@Req() req: Request, @Res() res: Response) {
    const err = new Error("simple error without stack");
    traceway.captureExceptionWithAttributes(
      err,
      undefined,
      (req as any).traceId
    );
    res.status(500).json({ error: "simple error" });
  }

  @Get("test-error-stacktrace")
  testErrorStacktrace(@Req() req: Request, @Res() res: Response) {
    const err = new Error("error with stack trace");
    traceway.captureExceptionWithAttributes(
      err,
      undefined,
      (req as any).traceId
    );
    res.status(500).json({ error: "stacktrace error" });
  }

  @Get("test-error-wrapped")
  testErrorWrapped(@Req() req: Request, @Res() res: Response) {
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
  }

  @Get("test-error-nested")
  testErrorNested(@Req() req: Request, @Res() res: Response) {
    const err = outerFunction();
    traceway.captureExceptionWithAttributes(
      err,
      undefined,
      (req as any).traceId
    );
    res.status(500).json({ error: "nested error" });
  }

  @Get("test-message")
  testMessage() {
    traceway.captureMessage("test message from /test-message");
    return { status: "message sent" };
  }

  @Get("test-message-attributes")
  testMessageAttributes() {
    traceway.captureMessage("test message with attributes");
    return { status: "message with attributes sent" };
  }

  @Get("test-spans")
  async testSpans() {
    const dbSpan = traceway.startSpan("db.query");
    await new Promise((r) => setTimeout(r, 50));
    traceway.endSpan(dbSpan);

    const cacheSpan = traceway.startSpan("cache.set");
    await new Promise((r) => setTimeout(r, 20));
    traceway.endSpan(cacheSpan);

    const httpSpan = traceway.startSpan("http.external_api");
    await new Promise((r) => setTimeout(r, 100));
    traceway.endSpan(httpSpan);

    return { status: "spans captured" };
  }

  @Get("test-task")
  testTask() {
    traceway.measureTask("background-data-processor", async () => {
      const span = traceway.startSpan("processing");
      await new Promise((r) => setTimeout(r, 200));
      traceway.endSpan(span);
    });
    return { status: "task started" };
  }

  @Get("test-metric")
  testMetric() {
    traceway.captureMetric("test.custom_metric", 42.0);
    return { status: "metric captured" };
  }

  @Get("test-attributes")
  testAttributes() {
    traceway.captureExceptionWithAttributes(
      new Error("exception with custom attributes"),
      {
        user_id: "usr_123",
        request_id: "req_456",
        env: "testing",
      }
    );
    return { status: "exception with attributes captured" };
  }

  @Post("test-recording")
  testRecording(@Req() req: Request, @Res() res: Response) {
    const body = req.body;

    if (body?.action === "panic") {
      throw new Error("panic triggered by /test-recording");
    }

    res.json({ status: "ok", received: body });
  }
}
