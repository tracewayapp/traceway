'use strict';
const express = require('express');
const { CACHE_TTL_SECONDS, cacheKey } = require('../catalog-cache');
const cacheMetrics = require('../cache-metrics');

const CATALOG_SQL = `SELECT p.id, p.sku, p.title, p.brand, p.image_url, c.slug AS category, pr.amount_minor, pr.currency
  FROM products p JOIN categories c ON c.id = p.category_id JOIN prices pr ON pr.product_id = p.id AND pr.price_list = $1
  WHERE p.active ORDER BY c.position, p.rank`;

function priceList(locale) {
  return locale === 'en-GB' ? 'gbp' : locale === 'en-US' ? 'usd' : 'eur';
}

function catalogRouter({ db, redis }) {
  const router = express.Router();
  router.get('/api/catalog', async (req, res, next) => {
    try {
      const key = cacheKey(req.originalUrl);
      const cached = await redis.get(key);
      cacheMetrics.record('catalog', Boolean(cached));
      if (cached) {
        return res.type('json').send(cached);
      }
      const { rows } = await db.query(CATALOG_SQL, [priceList(req.query.locale)]);
      const body = JSON.stringify({ products: rows });
      await redis.set(key, body, 'EX', CACHE_TTL_SECONDS);
      res.type('json').send(body);
    } catch (err) {
      next(err);
    }
  });
  return router;
}

module.exports = { catalogRouter, priceList };
