import { Controller, Get, NotFoundException, Param } from '@nestjs/common';
import Redis from 'ioredis';
import { Pool } from 'pg';

const TTL_SECONDS = 300;

@Controller('api')
export class ProductsController {
  constructor(
    private readonly db: Pool,
    private readonly redis: Redis,
  ) {}

  @Get('products/:id')
  async findOne(@Param('id') id: string) {
    const key = `product:${id}`;
    const hit = await this.redis.get(key);
    if (hit) return JSON.parse(hit);
    const { rows } = await this.db.query(
      'SELECT id, slug, name, price_cents, currency, images, attributes FROM products WHERE id = $1',
      [id],
    );
    if (!rows[0]) throw new NotFoundException();
    await this.redis.set(key, JSON.stringify(rows[0]), 'EX', TTL_SECONDS);
    return rows[0];
  }

  @Get('categories/:slug/products')
  async byCategory(@Param('slug') slug: string) {
    const key = `category:${slug}:page:1`;
    const hit = await this.redis.get(key);
    if (hit) return JSON.parse(hit);
    const { rows } = await this.db.query(
      'SELECT id, slug, name, price_cents, currency, thumbnail_url FROM products WHERE category_id = $1 ORDER BY rank LIMIT 48',
      [slug],
    );
    await this.redis.set(key, JSON.stringify(rows), 'EX', TTL_SECONDS);
    return rows;
  }
}
