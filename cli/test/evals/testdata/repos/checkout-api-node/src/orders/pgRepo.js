// PostgreSQL repository. orders.cart_id carries a unique constraint
// (orders_cart_id_key): a cart becomes at most one order.
class PgOrdersRepo {
  constructor(pool) {
    this.pool = pool;
  }

  async findCart(cartId) {
    const { rows } = await this.pool.query('SELECT id, user_id, currency FROM carts WHERE id = $1', [cartId]);
    if (rows.length === 0) return null;
    const items = await this.pool.query('SELECT sku, qty, unit_price_cents FROM cart_items WHERE cart_id = $1', [cartId]);
    return {
      id: rows[0].id,
      userId: rows[0].user_id,
      currency: rows[0].currency,
      items: items.rows.map((r) => ({ sku: r.sku, qty: r.qty, unitPriceCents: r.unit_price_cents })),
    };
  }

  async insertOrder({ cartId, userId, totalCents, currency }) {
    const { rows } = await this.pool.query(
      "INSERT INTO orders (cart_id, user_id, total_cents, currency, status, created_at) VALUES ($1, $2, $3, $4, 'pending', now()) RETURNING id, cart_id, user_id, total_cents, currency, status",
      [cartId, userId, totalCents, currency],
    );
    return toOrder(rows[0]);
  }

  async findOrder(orderId) {
    const { rows } = await this.pool.query('SELECT id, cart_id, user_id, total_cents, currency, status FROM orders WHERE id = $1', [orderId]);
    return rows.length ? toOrder(rows[0]) : null;
  }

  async findOrderByCart(cartId) {
    const { rows } = await this.pool.query('SELECT id, cart_id, user_id, total_cents, currency, status FROM orders WHERE cart_id = $1', [cartId]);
    return rows.length ? toOrder(rows[0]) : null;
  }

  async markPaid(orderId, chargeId) {
    await this.pool.query("UPDATE orders SET status = 'paid', charge_id = $2 WHERE id = $1", [orderId, chargeId]);
  }
}

function toOrder(r) {
  return { id: r.id, cartId: r.cart_id, userId: r.user_id, totalCents: r.total_cents, currency: r.currency, status: r.status };
}

module.exports = { PgOrdersRepo };
