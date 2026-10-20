import 'reflect-metadata';
import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { accessLog } from './access-log';
import { session } from './session';

async function bootstrap() {
  const app = await NestFactory.create(AppModule);
  app.use(accessLog);
  app.use(session);
  await app.listen(Number(process.env.PORT ?? 8080));
}
bootstrap();
