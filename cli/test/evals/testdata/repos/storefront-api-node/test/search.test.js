import { test } from 'node:test'
import assert from 'node:assert/strict'
import { search, searchPattern } from '../src/search.js'

test('searchPattern escapes LIKE wildcards and ignores one-letter queries', () => {
  assert.equal(searchPattern('a'), null)
  assert.equal(searchPattern(' 50%_off '), '%50\\%\\_off%')
})

test('search returns products with their facet counts', async () => {
  const calls = []
  const db = {
    async query(sql, params) {
      calls.push(sql)
      if (sql.includes('FROM products')) return { rows: [{ id: 1, name: 'Trail Runner' }, { id: 2, name: 'Trail Boot' }] }
      return { rows: [{ facet: 'size', value: '42', n: 2 }] }
    },
  }
  const out = await search(db, 'trail')
  assert.equal(out.products.length, 2)
  assert.deepEqual(out.facets, [{ facet: 'size', value: '42', n: 2 }])
  assert.equal(calls.length, 2)
})
