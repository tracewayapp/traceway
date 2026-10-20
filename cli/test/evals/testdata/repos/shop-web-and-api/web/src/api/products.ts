export type Product = { id: number; name: string; slug: string; description: string; priceCents: number; images: string[] }

export async function getProduct(id: number): Promise<Product> {
  const res = await fetch(`/api/products/${id}`, { headers: { Accept: 'application/json' } })
  if (!res.ok) {
    throw new Error(`product ${id} failed: ${res.status}`)
  }
  return res.json()
}
