import express from 'express';
import { accessLog } from './access-log.js';
import { pool, redis } from './db.js';
import { priceCart } from './pricing.js';

const app = express();
// Ahead of the access log, so readiness probes stay out of it.
app.get('/healthz', (_req, res) => res.sendStatus(204));
app.use(accessLog);
app.use(express.json({ limit: '100kb' }));

app.get('/api/products', async (req, res) => {
  const { rows } = await pool.query(
    'SELECT id, sku, name, price_cents, thumbnail_url FROM products WHERE category_id = $1 AND visible ORDER BY rank LIMIT 48',
    [req.query.category]);
  res.json(rows);
});

app.get('/api/products/:id', async (req, res) => {
  const { rows: [product] } = await pool.query('SELECT id, sku, name, description, price_cents, stock FROM products WHERE id = $1', [req.params.id]);
  if (!product) return res.sendStatus(404);
  const { rows: images } = await pool.query('SELECT url, alt FROM product_images WHERE product_id = $1 ORDER BY position', [product.id]);
  res.json({ ...product, images });
});

app.get('/api/search', async (req, res) => {
  const r = await fetch('http://search.internal:9200/products/_search', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ query: { multi_match: { query: String(req.query.q ?? ''), fields: ['name^3', 'description'] } }, size: 24 }),
  });
  const body = await r.json();
  res.json(body.hits.hits.map((h) => ({ id: h._id, ...h._source })));
});

app.get('/api/cart', async (req, res) => {
  const { rows } = await pool.query(
    'SELECT p.id, p.name, p.price_cents, ci.qty FROM cart_items ci JOIN products p ON p.id = ci.product_id WHERE ci.cart_id = $1',
    [req.get('x-cart-id')]);
  res.json(priceCart(rows));
});

app.post('/api/cart/items', async (req, res) => {
  const { productId, qty } = req.body ?? {};
  if (!productId || !(qty > 0)) return res.status(422).json({ error: 'productId and qty are required' });
  await pool.query(
    'INSERT INTO cart_items (cart_id, product_id, qty) VALUES ($1, $2, $3) ON CONFLICT (cart_id, product_id) DO UPDATE SET qty = cart_items.qty + EXCLUDED.qty',
    [req.get('x-cart-id'), productId, qty]);
  res.sendStatus(201);
});

app.get('/api/categories', async (_req, res) => {
  res.type('json').send(await redis.get('categories:tree'));
});

app.get('/api/recommendations', async (req, res) => {
  const { rows } = await pool.query('SELECT product_id, score FROM recommendations WHERE user_id = $1 ORDER BY score DESC LIMIT 12', [req.get('x-user-id')]);
  res.json(rows);
});

app.post('/api/checkout', async (req, res) => {
  const client = await pool.connect();
  try {
    await client.query('BEGIN');
    const { rows } = await client.query(
      'SELECT p.id, p.price_cents, p.stock, ci.qty FROM cart_items ci JOIN products p ON p.id = ci.product_id WHERE ci.cart_id = $1 FOR UPDATE',
      [req.get('x-cart-id')]);
    const order = priceCart(rows);
    const pay = await fetch('https://api.payments.example/v1/payment_intents', {
      method: 'POST',
      headers: { authorization: `Bearer ${process.env.PAYMENTS_KEY}`, 'content-type': 'application/json' },
      body: JSON.stringify({ amount: order.totalCents, currency: 'eur' }),
    });
    if (!pay.ok) throw new Error(`payment failed: ${pay.status}`);
    await client.query("INSERT INTO orders (id, user_id, total_cents, status, created_at) VALUES ($1, $2, $3, 'paid', now())",
      [crypto.randomUUID(), req.get('x-user-id'), order.totalCents]);
    await client.query('COMMIT');
    res.status(201).json(order);
  } catch (err) {
    await client.query('ROLLBACK');
    throw err;
  } finally {
    client.release();
  }
});

app.listen(Number(process.env.PORT ?? 8080));
