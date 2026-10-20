'use strict';

const db = require('../db');

const PAGE_SIZE = 48;

async function categoryProducts(categoryId) {
  const { rows } = await db.query(
    'SELECT * FROM products WHERE category_id = $1 AND is_active = true ORDER BY sort_rank DESC LIMIT 48',
    [categoryId],
  );
  return rows;
}

async function product(id) {
  const { rows } = await db.query('SELECT * FROM products WHERE id = $1', [id]);
  return rows[0] || null;
}

async function productImages(id) {
  const { rows } = await db.query(
    'SELECT id, url, alt, position FROM product_images WHERE product_id = $1 ORDER BY position',
    [id],
  );
  return rows;
}

async function price(id) {
  const { rows } = await db.query(
    'SELECT id, price_cents, currency, compare_at_cents FROM products WHERE id = $1',
    [id],
  );
  return rows[0] || null;
}

async function search(q) {
  const { rows } = await db.query(
    "SELECT id, name, slug, price_cents, currency, thumbnail_url FROM products WHERE is_active = true AND search_vector @@ plainto_tsquery('english', $1) ORDER BY ts_rank(search_vector, plainto_tsquery('english', $1)) DESC LIMIT 48",
    [q],
  );
  return rows;
}

module.exports = { PAGE_SIZE, categoryProducts, product, productImages, price, search };
