const { serializeOrderSummary, serializeOrder } = require('./serialize');

function ordersRouter(express, pool) {
  const router = express.Router();

  // GET /api/orders?page=N: the signed-in account's orders, newest first.
  router.get('/orders', async (req, res, next) => {
    try {
      const page = Math.max(1, Number(req.query.page ?? 1));
      const { rows } = await pool.query(
        'SELECT id, created_at, status, item_count, total_cents, currency FROM orders WHERE account_id = $1 ORDER BY created_at DESC LIMIT 20 OFFSET $2',
        [req.accountId, (page - 1) * 20],
      );
      res.json({ orders: rows.map(serializeOrderSummary), page });
    } catch (err) {
      next(err);
    }
  });

  // GET /api/orders/:id
  router.get('/orders/:id', async (req, res, next) => {
    try {
      const { rows } = await pool.query(
        'SELECT id, created_at, status, item_count, total_cents, currency, shipping_address FROM orders WHERE id = $1 AND account_id = $2',
        [req.params.id, req.accountId],
      );
      if (rows.length === 0) return res.status(404).json({ error: 'not found' });
      const items = await pool.query('SELECT sku, name, qty, unit_price_cents FROM order_items WHERE order_id = $1', [req.params.id]);
      res.json(serializeOrder(rows[0], items.rows));
    } catch (err) {
      next(err);
    }
  });

  return router;
}

module.exports = { ordersRouter };
