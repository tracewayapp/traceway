import express from 'express'
import pg from 'pg'
import { search } from './search.js'

const db = new pg.Pool({ connectionString: process.env.DATABASE_URL, max: 20 })
const app = express()
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

app.listen(Number(process.env.PORT ?? 8080))
