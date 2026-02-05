import { MiddlewareConsumer, Module, NestModule } from "@nestjs/common";
import { APP_FILTER } from "@nestjs/core";
import {
  TracewayModule,
  TracewayMiddleware,
  TracewayExceptionFilter,
} from "@tracewayapp/nestjs";
import { AppController } from "./app.controller";
import { AppService } from "./app.service";
import { UsersModule } from "./users/users.module";

@Module({
  imports: [
    TracewayModule.forRoot({
      connectionString:
        process.env.TRACEWAY_ENDPOINT ||
        "default_token_change_me@http://localhost:8082/api/report",
      debug: true,
      onErrorRecording: ["url", "query", "body", "headers"],
    }),
    UsersModule,
  ],
  controllers: [AppController],
  providers: [
    AppService,
    {
      provide: APP_FILTER,
      useClass: TracewayExceptionFilter,
    },
  ],
})
export class AppModule implements NestModule {
  configure(consumer: MiddlewareConsumer) {
    consumer.apply(TracewayMiddleware).forRoutes("*");
  }
}
