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
    res.json(await search(db, String(req.query.q ?? '')))
  } catch (err) {
    next(err)
  }
})

app.get('/api/products/:id', async (req, res, next) => {
  try {
    const { rows } = await db.query('SELECT id, name, slug, description, price_cents, images FROM products WHERE id = $1', [req.params.id])
    if (rows.length === 0) return res.status(404).json({ error: 'product not found' })
    const p = rows[0]
    res.json({ id: p.id, name: p.name, slug: p.slug, description: p.description, priceCents: p.price_cents, images: p.images })
  } catch (err) {
    next(err)
  }
})

app.post('/api/cart/items', async (req, res, next) => {
  const cartId = req.get('x-cart-id')
  const { productId, qty } = req.body ?? {}
  if (!cartId || !Number.isInteger(productId) || !Number.isInteger(qty) || qty < 1) {
    return res.status(400).json({ error: 'x-cart-id, productId and a positive qty are required' })
  }
  try {
    await db.query(
      'INSERT INTO cart_items (cart_id, product_id, qty) VALUES ($1, $2, $3) ON CONFLICT (cart_id, product_id) DO UPDATE SET qty = cart_items.qty + EXCLUDED.qty',
      [cartId, productId, qty],
    )
    res.status(204).end()
  } catch (err) {
    next(err)
  }
})

app.listen(Number(process.env.PORT ?? 8080))
