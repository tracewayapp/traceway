import "reflect-metadata";
import { NestFactory } from "@nestjs/core";
import { AppModule } from "./app.module.js";
import * as traceway from "@traceway/backend";

const endpoint =
  process.env.TRACEWAY_ENDPOINT ||
  "default_token_change_me@http://localhost:8082/api/report";
const port = process.env.PORT || "8080";

traceway.init(endpoint, { debug: true });

async function bootstrap() {
  const app = await NestFactory.create(AppModule);
  await app.listen(Number(port));
  console.log(`NestJS server starting on :${port}`);
}

bootstrap();
