import { Body, Controller, Get, Headers, Post } from '@nestjs/common';
import { Pool } from 'pg';

@Controller('api/cart')
export class CartController {
  constructor(private readonly db: Pool) {}

  @Get()
  async get(@Headers('x-cart-id') cartId: string) {
    const { rows } = await this.db.query(
      'SELECT l.sku, l.qty, l.unit_price_cents FROM cart_lines l WHERE l.cart_id = $1',
      [cartId],
    );
    return { lines: rows, totalCents: rows.reduce((s, l) => s + l.qty * l.unit_price_cents, 0) };
  }

  @Post('lines')
  async addLine(@Headers('x-cart-id') cartId: string, @Body() body: { sku: string; qty: number; unitPriceCents: number }) {
    await this.db.query(
      'INSERT INTO cart_lines (cart_id, sku, qty, unit_price_cents) VALUES ($1, $2, $3, $4) ON CONFLICT (cart_id, sku) DO UPDATE SET qty = cart_lines.qty + EXCLUDED.qty',
      [cartId, body.sku, body.qty, body.unitPriceCents],
    );
    return { ok: true };
  }
}
