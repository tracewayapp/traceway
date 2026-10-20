import { Module } from '@nestjs/common';
import { APP_GUARD } from '@nestjs/core';
import { Pool } from 'pg';
import Redis from 'ioredis';
import { FeatureFlagsGuard } from './flags/feature-flags.guard';
import { FeatureFlagsService } from './flags/feature-flags.service';
import { MeController } from './me/me.controller';
import { ProductsController } from './catalog/products.controller';
import { CartController } from './cart/cart.controller';

@Module({
  controllers: [MeController, ProductsController, CartController],
  providers: [
    { provide: Pool, useValue: new Pool({ connectionString: process.env.DATABASE_URL, max: 20 }) },
    { provide: Redis, useValue: new Redis(process.env.REDIS_URL ?? 'redis://localhost:6379') },
    FeatureFlagsService,
    // Every request gets the current flags on req.flags, so handlers and
    // templates can branch on them.
    { provide: APP_GUARD, useClass: FeatureFlagsGuard },
  ],
})
export class AppModule {}
