import { Module, MiddlewareConsumer, NestModule } from "@nestjs/common";
import { AppController } from "./app.controller.js";
import { TracewayMiddleware } from "./traceway.middleware.js";

@Module({
  controllers: [AppController],
})
export class AppModule implements NestModule {
  configure(consumer: MiddlewareConsumer) {
    consumer.apply(TracewayMiddleware).forRoutes("*");
  }
}
