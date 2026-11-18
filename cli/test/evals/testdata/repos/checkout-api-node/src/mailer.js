class Mailer {
  constructor(pool) {
    this.pool = pool;
  }

  async enqueueConfirmation(orderId) {
    await this.pool.query("INSERT INTO outbox (kind, ref, created_at) VALUES ('order_confirmation', $1, now())", [orderId]);
  }
}

module.exports = { Mailer };
