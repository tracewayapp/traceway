// Product search: the hosted index answers the query, Postgres adds live
// stock and price.

const INDEX_URL = process.env.SEARCH_INDEX_URL ?? 'https://shop-prod.search-cloud.example/1/indexes/products/query'

export async function queryIndex(q, { fetchImpl = fetch } = {}) {
  const res = await fetchImpl(INDEX_URL, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Api-Key': process.env.SEARCH_API_KEY ?? '' },
    body: JSON.stringify({ query: q, hitsPerPage: 24 }),
  })
  if (!res.ok) throw new Error(`search index answered ${res.status}`)
  const body = await res.json()
  return body.hits.map((h) => ({ id: Number(h.objectID), name: h.name, slug: h.slug }))
}

export async function search(db, q, opts) {
  const hits = await queryIndex(q, opts)
  if (hits.length === 0) return { hits }
  const { rows } = await db.query('SELECT id, stock, price_cents FROM product_stock WHERE id = ANY($1)', [hits.map((h) => h.id)])
  const live = new Map(rows.map((r) => [r.id, r]))
  return {
    hits: hits.map((h) => ({ ...h, priceCents: live.get(h.id)?.price_cents ?? null, inStock: (live.get(h.id)?.stock ?? 0) > 0 })),
  }
}
