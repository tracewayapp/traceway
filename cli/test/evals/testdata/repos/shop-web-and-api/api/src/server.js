import express from 'express'
import pg from 'pg'
import { search } from './search.js'

const db = new pg.Pool({ connectionString: process.env.DATABASE_URL, max: 20 })
const app = express()

app.get('/api/search', async (req, res, next) => {
  try {
    res.json(await search(db, String(req.query.q ?? '')))
  } catch (err) {
    next(err)
  }
})

app.listen(Number(process.env.PORT ?? 8080))
