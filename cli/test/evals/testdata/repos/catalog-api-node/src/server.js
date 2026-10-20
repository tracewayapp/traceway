'use strict';

const express = require('express');
const q = require('./catalog/queries');
const db = require('./db');
const { toCard } = require('./catalog/cards');

const app = express();

app.get('/api/categories/:categoryId/products', async (req, res) => {
  const category = await db.query('SELECT id, name, slug FROM categories WHERE id = $1', [req.params.categoryId]);
  if (category.rows.length === 0) return res.status(404).json({ error: 'category not found' });
  const products = await q.categoryProducts(category.rows[0].id);
  res.json({ category: category.rows[0], items: products.map(toCard) });
});

app.get('/api/products/:id', async (req, res) => {
  const p = await q.product(req.params.id);
  if (!p) return res.status(404).json({ error: 'not found' });
  const images = await q.productImages(p.id);
  res.json({
    id: p.id,
    name: p.name,
    slug: p.slug,
    descriptionHtml: p.description_html,
    specs: p.spec_sheet,
    priceCents: p.price_cents,
    currency: p.currency,
    images,
  });
});

app.get('/api/products/:id/price', async (req, res) => {
  const p = await q.price(req.params.id);
  if (!p) return res.status(404).json({ error: 'not found' });
  res.json(p);
});

app.get('/api/search', async (req, res) => {
  const rows = await q.search(String(req.query.q || ''));
  res.json({ items: rows.map(toCard) });
});

app.get('/api/brands', async (_req, res) => {
  const { rows } = await db.query('SELECT id, name, slug, logo_url FROM brands ORDER BY name');
  res.json(rows);
});

app.get('/api/products/:id/reviews', async (req, res) => {
  const { rows } = await db.query(
    'SELECT id, rating, title, body, author_name, created_at FROM reviews WHERE product_id = $1 ORDER BY created_at DESC LIMIT 20',
    [req.params.id],
  );
  res.json(rows);
});

app.listen(Number(process.env.PORT || 8080));
