'use strict';
const express = require('express');
const cacheMetrics = require('../cache-metrics');

function reviewsRouter({ db, redis }) {
  const router = express.Router();
  router.get('/api/products/:sku/reviews', async (req, res, next) => {
    try {
      const sid = req.get('cookie')?.match(/sid=([a-f0-9]+)/)?.[1];
      const session = sid ? await redis.get(`sess:${sid}`) : null;
      cacheMetrics.record('session', Boolean(session));
      const page = Math.max(1, Number(req.query.page) || 1);
      const { rows } = await db.query(
        'SELECT id, rating, title, body, created_at FROM reviews WHERE product_sku = $1 ORDER BY created_at DESC LIMIT 10 OFFSET $2',
        [req.params.sku, (page - 1) * 10],
      );
      res.json({ reviews: rows, page });
    } catch (err) {
      next(err);
    }
  });
  return router;
}

module.exports = { reviewsRouter };
