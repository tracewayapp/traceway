import React, { useEffect, useState } from 'react'
import { getCart, removeFromCart, formatPrice } from './api.js'

export default function CartPage({ onCartChanged, goCheckout }) {
  const [cart, setCart] = useState({ items: [], total_cents: 0 })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  async function load() {
    setLoading(true)
    setError('')
    try {
      const data = await getCart()
      setCart(data)
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function handleRemove(id) {
    try {
      await removeFromCart(id)
      await load()
      onCartChanged?.()
    } catch (e) {
      setError(e.message)
    }
  }

  if (loading) return <div className="loading">Loading cart…</div>
  if (error)
    return (
      <div className="error-box">
        <p>{error}</p>
        <button onClick={load}>Retry</button>
      </div>
    )

  return (
    <div>
      <h1>Your Cart</h1>
      {cart.items.length === 0 ? (
        <p className="muted">Your cart is empty.</p>
      ) : (
        <div className="cart">
          {cart.items.map((line) => (
            <div className="cart-row" key={line.id}>
              <img src={line.image_url} alt={line.name} />
              <div className="cart-info">
                <div className="card-name">{line.name}</div>
                <div className="muted">
                  {formatPrice(line.price_cents)} × {line.qty}
                </div>
              </div>
              <div className="cart-line-total">{formatPrice(line.line_total_cents)}</div>
              <button className="ghost" onClick={() => handleRemove(line.id)}>
                Remove
              </button>
            </div>
          ))}
          <div className="cart-footer">
            <div className="cart-total">
              Total: <strong>{formatPrice(cart.total_cents)}</strong>
            </div>
            <button className="primary" onClick={goCheckout}>
              Proceed to checkout
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
