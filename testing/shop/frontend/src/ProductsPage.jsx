import React, { useEffect, useState } from 'react'
import { getProducts, addToCart, formatPrice } from './api.js'

export default function ProductsPage({ onCartChanged }) {
  const [products, setProducts] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [adding, setAdding] = useState(null)
  const [quickView, setQuickView] = useState(null)
  const [notice, setNotice] = useState('')

  async function load() {
    setLoading(true)
    setError('')
    try {
      const data = await getProducts()
      setProducts(data)
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function handleAdd(productId) {
    setAdding(productId)
    setNotice('')
    await addToCart(productId, 1)
    setNotice('Added to cart')
    setAdding(null)
    onCartChanged?.()
  }

  function handleQuickView(p) {
    try {
      const variant = p.variants.find((v) => v.default)
      setQuickView({ name: p.name, label: variant.label })
    } catch {
      setQuickView({ name: p.name, label: 'No variants available' })
    }
  }

  if (loading) return <div className="loading">Loading products…</div>
  if (error)
    return (
      <div className="error-box">
        <p>{error}</p>
        <button onClick={load}>Retry</button>
      </div>
    )

  return (
    <div>
      <h1>Products</h1>
      {notice && <div className="notice">{notice}</div>}
      {quickView && (
        <div className="quickview" onClick={() => setQuickView(null)}>
          <strong>{quickView.name}</strong> — {quickView.label} <span className="dismiss">✕</span>
        </div>
      )}
      <div className="grid">
        {products.map((p) => (
          <div className="card" key={p.id}>
            <img src={p.image_url} alt={p.name} loading="lazy" />
            <div className="card-body">
              <div className="card-cat">{p.category}</div>
              <div className="card-name">{p.name}</div>
              <div className="card-desc">{p.description}</div>
              <div className="card-meta">
                <span className="price">{formatPrice(p.price_cents)}</span>
                <span className="reviews">★ {p.review_count}</span>
              </div>
              <div className="card-actions">
                <button className="primary" disabled={adding === p.id} onClick={() => handleAdd(p.id)}>
                  {adding === p.id ? 'Adding…' : 'Add to cart'}
                </button>
                <button className="ghost" onClick={() => handleQuickView(p)}>
                  Quick view
                </button>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
