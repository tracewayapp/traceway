export type SearchHit = { id: number; name: string; slug: string; priceCents: number; inStock: boolean }

// Shoppers stop waiting long before this; past it we show the
// "taking too long" message and let them retry.
export const SEARCH_TIMEOUT_MS = 8000

export async function searchProducts(q: string): Promise<SearchHit[]> {
  const params = new URLSearchParams({ q })
  const res = await fetch(`/api/search?${params}`, {
    headers: { Accept: 'application/json' },
    signal: AbortSignal.timeout(SEARCH_TIMEOUT_MS),
  })
  if (!res.ok) {
    throw new Error(`search failed: ${res.status}`)
  }
  const body = await res.json()
  return body.hits
}
