import express from 'express'
import pg from 'pg'
import { accessLog } from './access-log.js'
import { search } from './search.js'

const db = new pg.Pool({ connectionString: process.env.DATABASE_URL, max: 20 })
const app = express()
app.use(accessLog)
app.use(express.json())

app.get('/api/search', async (req, res, next) => {
  try {
    res.json(await search(db, req.query.q))
  } catch (err) {
    next(err)
  }
})

app.get('/api/products/:id', async (req, res, next) => {
  try {
    const { rows } = await db.query('SELECT id, name, slug, brand, description, price_cents, images FROM products WHERE id = $1 AND active', [req.params.id])
    if (rows.length === 0) return res.sendStatus(404)
    const { rows: variants } = await db.query('SELECT sku, size, color, stock FROM product_variants WHERE product_id = $1', [rows[0].id])
    res.json({ ...rows[0], variants })
  } catch (err) {
    next(err)
  }
})

app.post('/api/checkout', async (req, res, next) => {
  const client = await db.connect()
  try {
    await client.query('BEGIN')
    const { rows: [cart] } = await client.query('SELECT id, items, total_cents, currency FROM carts WHERE id = $1 FOR UPDATE', [req.body?.cartId])
    if (!cart) {
      await client.query('ROLLBACK')
      return res.sendStatus(404)
    }
    const payment = await fetch('https://api.payments.example/v1/payment_intents', {
      method: 'POST',
      headers: { authorization: `Bearer ${process.env.PAYMENTS_API_KEY}`, 'content-type': 'application/json', 'idempotency-key': `cart-${cart.id}` },
      body: JSON.stringify({ amount: cart.total_cents, currency: cart.currency, payment_method: req.body?.paymentMethod, confirm: true }),
    })
    if (!payment.ok) {
      await client.query('ROLLBACK')
      return res.status(402).json({ error: 'payment declined' })
    }
    const { id: paymentRef } = await payment.json()
    const { rows: [order] } = await client.query(
      'INSERT INTO orders (cart_id, total_cents, currency, payment_ref, created_at) VALUES ($1, $2, $3, $4, now()) RETURNING id',
      [cart.id, cart.total_cents, cart.currency, paymentRef],
    )
    await client.query('COMMIT')
    res.status(201).json({ orderId: order.id })
  } catch (err) {
    await client.query('ROLLBACK').catch(() => {})
    next(err)
  } finally {
    client.release()
  }
})

app.listen(Number(process.env.PORT ?? 8080))
