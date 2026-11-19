import express from 'express';
import { accessLog } from './access-log.js';
import { pool } from './db.js';
import { nightsBetween } from './dates.js';

const app = express();
app.use(accessLog);
app.use(express.json());

// Indexes: availability (listing_id, day), bookings (id), listings (id),
// reviews (listing_id, created_at DESC). See migrations in the infra repo.

app.get('/api/availability', async (req, res) => {
  const ids = String(req.query.listings ?? '').split(',').filter(Boolean).map(Number);
  const { rows } = await pool.query(
    'SELECT listing_id, day, units_free, price_cents FROM availability WHERE listing_id = ANY($1) AND day BETWEEN $2 AND $3 ORDER BY listing_id, day',
    [ids, req.query.from, req.query.to]);
  res.json(rows);
});

app.get('/api/bookings/:id', async (req, res) => {
  const { rows: [b] } = await pool.query(
    'SELECT id, listing_id, guest_id, check_in, check_out, status, total_cents FROM bookings WHERE id = $1', [req.params.id]);
  if (!b) return res.sendStatus(404);
  const { rows: [listing] } = await pool.query('SELECT id, title, city, host_id FROM listings WHERE id = $1', [b.listing_id]);
  res.json({ ...b, listing });
});

app.post('/api/bookings', async (req, res) => {
  const { listingId, guestId, checkIn, checkOut, totalCents } = req.body ?? {};
  if (nightsBetween(checkIn, checkOut) < 1) return res.status(422).json({ error: 'checkOut must be after checkIn' });
  const client = await pool.connect();
  try {
    await client.query('BEGIN');
    const { rows } = await client.query(
      'SELECT units_free FROM availability WHERE listing_id = $1 AND day >= $2 AND day < $3 FOR UPDATE', [listingId, checkIn, checkOut]);
    if (rows.length === 0 || rows.some((r) => r.units_free < 1)) {
      await client.query('ROLLBACK');
      return res.status(409).json({ error: 'not available' });
    }
    await client.query('UPDATE availability SET units_free = units_free - 1 WHERE listing_id = $1 AND day >= $2 AND day < $3', [listingId, checkIn, checkOut]);
    const id = crypto.randomUUID();
    await client.query(
      "INSERT INTO bookings (id, listing_id, guest_id, check_in, check_out, status, total_cents, created_at) VALUES ($1, $2, $3, $4, $5, 'confirmed', $6, now())",
      [id, listingId, guestId, checkIn, checkOut, totalCents]);
    await client.query('COMMIT');
    res.status(201).json({ id });
  } catch (err) {
    await client.query('ROLLBACK');
    throw err;
  } finally {
    client.release();
  }
});

app.get('/api/listings/:id/reviews', async (req, res) => {
  const { rows } = await pool.query(
    'SELECT id, author_name, rating, body, created_at FROM reviews WHERE listing_id = $1 ORDER BY created_at DESC LIMIT 20', [req.params.id]);
  res.json(rows);
});

app.listen(Number(process.env.PORT ?? 8080));
