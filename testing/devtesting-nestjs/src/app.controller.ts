import {
  Controller,
  Get,
  Post,
  Param,
  Body,
  HttpException,
  HttpStatus,
} from "@nestjs/common";
import { AppService } from "./app.service";
import { TracewayService } from "@tracewayapp/nestjs";

interface JsonRecordingTest {
  name: string;
}

@Controller()
export class AppController {
  constructor(
    private readonly appService: AppService,
    private readonly traceway: TracewayService,
  ) {}

  @Get("test-ok")
  testOk(): { status: string } {
    return this.appService.getOk();
  }

  @Get("test-not-found")
  testNotFound(): never {
    throw new HttpException({ status: "not-found" }, HttpStatus.NOT_FOUND);
  }

  @Get("test-exception")
  async testException(): Promise<never> {
    await this.sleep(Math.random() * 2000);
    throw new Error("Cool");
  }

  @Get("test-message")
  testMessage(): { status: string } {
    this.appService.captureMessages();
    return { status: "ok" };
  }

  @Get("test-spans")
  async testSpans(): Promise<{ status: string; message: string }> {
    return this.appService.runSpans();
  }

  @Get("test-task")
  testTask(): { status: string } {
    this.appService.runBackgroundTask();
    return { status: "task started" };
  }

  @Get("test-param/:param")
  testParam(@Param("param") param: string): { param: string } {
    return { param };
  }

  @Get("test-self-report-attributes")
  testSelfReportAttributes(): { status: string } {
    this.appService.captureAttributesException();
    return { status: "ok" };
  }

  @Get("test-self-report-context")
  testSelfReportContext(): { status: string } {
    this.appService.captureContextException();
    return { status: "ok" };
  }

  @Get("test-cerror-simple")
  testCerrorSimple(): never {
    throw this.appService.getSimpleError();
  }

  @Get("test-cerror-stacktrace")
  testCerrorStacktrace(): never {
    throw this.appService.getStacktraceError();
  }

  @Get("test-cerror-wrapped")
  testCerrorWrapped(): never {
    throw this.appService.getWrappedError();
  }

  @Get("test-cerror-custom")
  testCerrorCustom(): never {
    throw this.appService.getCustomError();
  }

  @Post("test-recording/:param")
  testRecording(
    @Param("param") param: string,
    @Body() body: JsonRecordingTest,
  ): { status: string } {
    if (body.name !== "good") {
      throw new Error("Bad");
    }
    return { status: "ok", param } as { status: string };
  }

  private sleep(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }
}
