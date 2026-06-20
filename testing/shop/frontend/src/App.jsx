import React, { useState, useEffect } from 'react'
import { getCart } from './api.js'
import ProductsPage from './ProductsPage.jsx'
import CartPage from './CartPage.jsx'
import CheckoutPage from './CheckoutPage.jsx'

export default function App() {
  const [page, setPage] = useState('products')
  const [cartCount, setCartCount] = useState(0)

  async function refreshCartCount() {
    try {
      const cart = await getCart()
      const count = (cart.items || []).reduce((n, l) => n + l.qty, 0)
      setCartCount(count)
    } catch {
      setCartCount(0)
    }
  }

  useEffect(() => {
    refreshCartCount()
  }, [page])

  return (
    <div className="app">
      <header className="topbar">
        <div className="brand" onClick={() => setPage('products')}>
          <span className="logo">🛍️</span> Traceway Demo Shop
        </div>
        <nav>
          <button className={page === 'products' ? 'active' : ''} onClick={() => setPage('products')}>
            Products
          </button>
          <button className={page === 'cart' ? 'active' : ''} onClick={() => setPage('cart')}>
            Cart <span className="badge">{cartCount}</span>
          </button>
          <button className={page === 'checkout' ? 'active' : ''} onClick={() => setPage('checkout')}>
            Checkout
          </button>
        </nav>
      </header>
      <main className="content">
        {page === 'products' && <ProductsPage onCartChanged={refreshCartCount} />}
        {page === 'cart' && (
          <CartPage onCartChanged={refreshCartCount} goCheckout={() => setPage('checkout')} />
        )}
        {page === 'checkout' && (
          <CheckoutPage
            onDone={() => {
              refreshCartCount()
              setPage('products')
            }}
          />
        )}
      </main>
    </div>
  )
}
