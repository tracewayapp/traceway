// Product search: a name/brand match ranked by popularity, plus the facet
// counts the filter sidebar shows.

export const PAGE_SIZE = 24

export function searchPattern(q) {
  const term = String(q ?? '').trim().slice(0, 80)
  if (term.length < 2) return null
  return '%' + term.replace(/[\\%_]/g, (c) => '\\' + c) + '%'
}

export async function search(db, q) {
  const pattern = searchPattern(q)
  if (!pattern) return { products: [], facets: [] }
  const { rows: products } = await db.query(
    'SELECT id, name, slug, brand, price_cents, thumbnail_url FROM products WHERE active AND (name ILIKE $1 OR brand ILIKE $1) ORDER BY popularity DESC LIMIT 24',
    [pattern],
  )
  if (products.length === 0) return { products, facets: [] }
  const { rows: facets } = await db.query(
    'SELECT facet, value, count(*) AS n FROM product_facets WHERE product_id = ANY($1) GROUP BY facet, value',
    [products.map((p) => p.id)],
  )
  return { products, facets }
}
