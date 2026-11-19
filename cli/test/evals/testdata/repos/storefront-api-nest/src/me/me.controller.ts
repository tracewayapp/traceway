import { Controller, Get, Req, UnauthorizedException } from '@nestjs/common';
import { Pool } from 'pg';

@Controller('api/me')
export class MeController {
  constructor(private readonly db: Pool) {}

  @Get()
  async getMe(@Req() req: { customerId?: number; flags: Map<string, { enabled: boolean }> }) {
    if (!req.customerId) throw new UnauthorizedException();
    const { rows } = await this.db.query(
      'SELECT id, email, first_name, last_name, loyalty_tier FROM customers WHERE id = $1',
      [req.customerId],
    );
    const me = rows[0];
    return {
      id: me.id,
      email: me.email,
      name: `${me.first_name} ${me.last_name}`,
      loyaltyTier: req.flags.get('loyalty.tiers')?.enabled ? me.loyalty_tier : null,
    };
  }
}
