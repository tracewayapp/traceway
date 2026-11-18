import { NestFactory } from '@nestjs/core';
import { Module } from '@nestjs/common';
import { DashboardController } from './dashboard/dashboard.controller';
import { DashboardService } from './dashboard/dashboard.service';
import { DashboardPresenter } from './dashboard/dashboard.presenter';
import { InternalClient } from './clients/internal-client';
import { accessLog } from './access-log';

@Module({
  controllers: [DashboardController],
  providers: [DashboardService, DashboardPresenter, InternalClient],
})
class AppModule {}

async function bootstrap() {
  const app = await NestFactory.create(AppModule);
  app.use(accessLog);
  await app.listen(process.env.PORT ?? 3000);
}

bootstrap();
