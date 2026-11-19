import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getProduct, type Product } from '../api/products'
import { rememberViewed } from '../recentlyViewed'

export function ProductPage() {
  const { id } = useParams()
  const [product, setProduct] = useState<Product | null>(null)
  const [missing, setMissing] = useState(false)

  useEffect(() => {
    let live = true
    getProduct(Number(id))
      .then((p) => {
        if (!live) return
        setProduct(p)
        rememberViewed(p)
      })
      .catch(() => live && setMissing(true))
    return () => {
      live = false
    }
  }, [id])

  return (
    <main>
      <Link to="/search">Search products</Link>
      {missing && <p>This product is not available.</p>}
      {product && (
        <article>
          <h1>{product.name}</h1>
          <p>{(product.priceCents / 100).toFixed(2)}</p>
          <p>{product.description}</p>
        </article>
      )}
    </main>
  )
}
