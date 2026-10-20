import { test } from 'node:test'
import assert from 'node:assert/strict'
import { search } from '../src/search.js'

test('search merges index hits with live stock', async () => {
  const fetchImpl = async () => ({ ok: true, json: async () => ({ hits: [{ objectID: '7', name: 'Trail Runner', slug: 'trail-runner' }] }) })
  const db = { query: async () => ({ rows: [{ id: 7, stock: 3, price_cents: 8900 }] }) }
  const out = await search(db, 'trail', { fetchImpl })
  assert.deepEqual(out.hits, [{ id: 7, name: 'Trail Runner', slug: 'trail-runner', priceCents: 8900, inStock: true }])
})
