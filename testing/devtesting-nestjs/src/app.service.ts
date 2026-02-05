import { Injectable } from "@nestjs/common";
import { TracewayService, Span } from "@tracewayapp/nestjs";

class CustomError extends Error {
  constructor(
    public code: number,
    message: string,
  ) {
    super(`CustomError[${code}]: ${message}`);
    this.name = "CustomError";
  }
}

@Injectable()
export class AppService {
  constructor(private readonly traceway: TracewayService) {}

  getOk(): { status: string } {
    return { status: "ok" };
  }

  captureMessages(): void {
    for (let i = 0; i < 10; i++) {
      this.traceway.captureMessage(`test message ${i}`);
    }
    this.traceway.captureException(new Error("test message exception"));
  }

  @Span("db.query")
  async simulateDbQuery(): Promise<void> {
    await this.sleep(50 + Math.random() * 100);
  }

  @Span("cache.set")
  async simulateCacheSet(): Promise<void> {
    await this.sleep(10 + Math.random() * 30);
  }

  @Span("http.external_api")
  async simulateHttpCall(): Promise<void> {
    await this.sleep(100 + Math.random() * 200);
  }

  async runSpans(): Promise<{ status: string; message: string }> {
    const dbAndCacheSpan = this.traceway.startSpan("db.and.cache");

    await this.simulateDbQuery();
    await this.simulateCacheSet();
    await this.simulateHttpCall();

    this.traceway.endSpan(dbAndCacheSpan);

    return { status: "ok", message: "Spans captured" };
  }

  runBackgroundTask(): void {
    this.traceway.measureTask("traceway data processor", async () => {
      const span = this.traceway.startSpan("loading data");
      await this.sleep(Math.random() * 2000);
      this.traceway.endSpan(span);

      for (let i = 0; i < 10; i++) {
        this.traceway.captureMessage(`data loaded successfully ${i}`);
      }

      this.traceway.captureException(new Error("what an error"));
    });
  }

  captureAttributesException(): void {
    this.traceway.captureExceptionWithAttributes(new Error("Test"), {
      Cool: `{"this": "is", "a": "json", "attr": 10}`,
      Cool2: "Pretty cool2",
      Cool3: "Pretty cool3",
    });
  }

  captureContextException(): void {
    this.traceway.setTraceAttribute("Interesting", "Pretty Cool");
    this.traceway.captureException(new Error("Test2"));
  }

  getSimpleError(): Error {
    return new Error("simple error without stack");
  }

  getWrappedError(): Error {
    const base = new Error("base error");
    const wrapped = new Error(`layer 1: ${base.message}`);
    wrapped.cause = base;
    const wrapped2 = new Error(`layer 2: ${wrapped.message}`);
    wrapped2.cause = wrapped;
    return wrapped2;
  }

  getStacktraceError(): Error {
    return this.innerFunction();
  }

  getCustomError(): Error {
    return new CustomError(500, "something went wrong");
  }

  private innerFunction(): Error {
    return new Error("error from inner function");
  }

  private sleep(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }
}
