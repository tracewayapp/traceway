// In-memory repository with the same contract as PgOrdersRepo, including the
// unique constraint on orders.cart_id (it throws what pg throws: code 23505).
class MemoryOrdersRepo {
  constructor(carts = {}) {
    this.carts = carts;
    this.orders = [];
    this.seq = 1000;
  }

  async findCart(cartId) {
    await tick();
    return this.carts[cartId] ?? null;
  }

  async insertOrder({ cartId, userId, totalCents, currency }) {
    await tick();
    if (this.orders.some((o) => o.cartId === cartId)) {
      const err = new Error('duplicate key value violates unique constraint "orders_cart_id_key"');
      err.code = '23505';
      err.constraint = 'orders_cart_id_key';
      err.detail = `Key (cart_id)=(${cartId}) already exists.`;
      throw err;
    }
    const order = { id: `ord_${++this.seq}`, cartId, userId, totalCents, currency, status: 'pending' };
    this.orders.push(order);
    return { ...order };
  }

  async findOrder(orderId) {
    await tick();
    const o = this.orders.find((x) => x.id === orderId);
    return o ? { ...o } : null;
  }

  async findOrderByCart(cartId) {
    await tick();
    const o = this.orders.find((x) => x.cartId === cartId);
    return o ? { ...o } : null;
  }

  async markPaid(orderId, chargeId) {
    await tick();
    const o = this.orders.find((x) => x.id === orderId);
    if (o) Object.assign(o, { status: 'paid', chargeId });
  }
}

function tick() {
  return new Promise((resolve) => setImmediate(resolve));
}

module.exports = { MemoryOrdersRepo };
